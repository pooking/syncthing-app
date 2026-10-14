// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build tools

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/syncthing/syncthing/desktop/brand"
)

// Inno Setup is the one external tool this repo's build depends on, and only
// for -installer. Everything else the app ships is produced by the Go toolchain
// alone. It is looked for where Inno installs itself rather than assumed to be
// on PATH, because the IDE's install directory is deliberately not added there.
const isccName = "ISCC.exe"

// installerScript is the .iss compiled into the setup executable, relative to
// the repository root.
const installerScript = "desktop/installer/OspreySyncTool.iss"

// installerScriptPath resolves the script and, through it, the generated art
// beside it. Both are addressed from the repository root rather than the
// working directory, because the build can be run from anywhere.
func installerScriptPath() string { return filepath.Join(repoRoot(), installerScript) }

// installerAssetDir is where the icon and the wizard panels live: beside the
// script, so the .iss can name them without a path.
func installerAssetDir() string { return filepath.Dir(installerScriptPath()) }

// The wizard art is drawn rather than checked in, from the same geometry as the
// icon and the tray (desktop/brand), so there is no second copy of the owl to
// keep in step with the first. Inno wants uncompressed BMPs at fixed sizes.
const (
	wizardLargeW, wizardLargeH = 164, 314 // the panel down the left of the wizard
	wizardSmallW, wizardSmallH = 55, 58   // the square in the header of later pages

	// How large the owl is drawn inside those panels. The small one is small
	// because the same drawing has to work in the header strip, where a mark
	// the size of the panel's would crowd the page title.
	wizardLargeIcon = 140
	wizardSmallIcon = 44
)

// wizardPaper is the pale blue the wizard panels are filled with: the tile's
// own light stop (0x4a9ae8) laid over white at a third of its strength. Pale
// enough that the blue owl on top stays the thing being looked at, and far
// enough from white that the panel has an edge against the dialog.
var wizardPaper = color.NRGBA{R: 0xc3, G: 0xde, B: 0xf7, A: 0xff}

// buildInstaller compiles the .iss into a setup executable in outDir.
func buildInstaller(outDir, ver string) error {
	iscc, err := isccPath()
	if err != nil {
		return err
	}
	wrote, err := ensureInstallerAssets()
	for _, p := range wrote {
		log.Printf("wrote %s", p)
	}
	if err != nil {
		return err
	}

	// The script refers to its assets and to the executable it packs by path,
	// and Inno resolves relative source paths against the script's own
	// directory. OutputDir is passed in absolute terms so the result lands in
	// this build's -out directory rather than in a fixed dist/.
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	absExe, err := filepath.Abs(filepath.Join(outDir, winBinaryName()))
	if err != nil {
		return err
	}
	if _, err := os.Stat(absExe); err != nil {
		return fmt.Errorf("%s is missing; the installer packs it, so build it first", absExe)
	}
	script := installerScriptPath()
	if err != nil {
		return err
	}

	args := []string{
		"/Qp",
		"/DAppVersion=" + ver,
		"/DVersionInfoVersion=" + numericVersion(ver),
		"/DSourceExe=" + absExe,
		"/DOutDir=" + absOut,
		script,
	}
	cmd := exec.Command(iscc, args...)
	cmd.Dir = repoRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("building installer %s -> %s", ver, outDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(iscc), err)
	}
	return nil
}

// checkInstallerHost rejects an -installer build aimed at another platform.
// The installer is a Windows program that packs a Windows program, so asking
// for it for linux or darwin is worth naming rather than leaving to an ISCC
// error further down.
func checkInstallerHost() error {
	if *goos != "windows" || *goarch != "amd64" {
		return fmt.Errorf(
			"the installer packs a windows/amd64 build; got -goos %s -goarch %s. "+
				"Build the desktop app for another platform with -zip instead",
			*goos, *goarch)
	}
	return nil
}

// winBinaryName is the file build() writes for windows/amd64. The installer and
// the build that precedes it both derive it here, so the setup cannot end up
// packing a differently named executable than the one just linked.
func winBinaryName() string {
	return fmt.Sprintf("%s-windows-amd64.exe", binaryName)
}

// ensureInstallerAssets writes the icon and the wizard panels beside the .iss
// if they are not already there, and reports which ones it wrote.
//
// They are build products, not sources: brand draws them and the .iss refers to
// them by path. Reusing what is already on disk is what lets the installer be
// built from a copy of the source tree without the Go toolchain, which is the
// point of shipping the generated files in the first place.
func ensureInstallerAssets() ([]string, error) {
	dir := installerAssetDir()
	assets := []struct {
		name string
		draw func() []byte
	}{
		{"appicon-256.ico", func() []byte { return brand.ICO([]int{256}, tileImage) }},
		{"wizard-large.bmp", func() []byte {
			return encodeBMP(wizardPanel(wizardLargeW, wizardLargeH, wizardLargeIcon))
		}},
		{"wizard-small.bmp", func() []byte {
			return encodeBMP(wizardPanel(wizardSmallW, wizardSmallH, wizardSmallIcon))
		}},
	}

	var wrote []string
	for _, a := range assets {
		path := filepath.Join(dir, a.name)
		if _, err := os.Stat(path); err == nil {
			continue // already generated; leave it alone
		}
		data := a.draw()
		if len(data) == 0 {
			return wrote, fmt.Errorf("%s: nothing was drawn", a.name)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return wrote, err
		}
		wrote = append(wrote, path)
	}
	return wrote, nil
}

// tileImage adapts brand.Tile, which returns the concrete image type, to the
// signature brand.ICO takes.
func tileImage(size int) image.Image { return brand.Tile(size) }

// wizardPanel fills a panel with wizardPaper and draws the owl on it in the
// interface's accent blue, centred.
func wizardPanel(w, h, iconSize int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: wizardPaper.R, G: wizardPaper.G, B: wizardPaper.B, A: 0xff,
			})
		}
	}
	if iconSize > w {
		iconSize = w
	}
	if iconSize > h {
		iconSize = h
	}
	owl := brand.Mark(iconSize, brand.DefaultTheme())
	offX, offY := (w-iconSize)/2, (h-iconSize)/2
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			src := owl.NRGBAAt(x, y)
			if src.A == 0 {
				continue
			}
			img.SetNRGBA(offX+x, offY+y, over(src, img.NRGBAAt(offX+x, offY+y)))
		}
	}
	return img
}

// over composites src, which carries its own alpha, onto the opaque dst.
func over(src, dst color.NRGBA) color.NRGBA {
	a, inv := uint32(src.A), 255-uint32(src.A)
	ch := func(s, d uint8) uint8 {
		return uint8((uint32(s)*a + uint32(d)*inv + 127) / 255)
	}
	return color.NRGBA{R: ch(src.R, dst.R), G: ch(src.G, dst.G), B: ch(src.B, dst.B), A: 0xff}
}

// encodeBMP writes an uncompressed 32-bit BMP, which is the format Inno Setup 6
// accepts for its wizard images.
//
// Two details of the format are easy to get wrong, and each produces an image
// that looks nothing like the input rather than an error: rows run bottom to
// top, and every pixel is stored blue, green, red, alpha.
func encodeBMP(img *image.NRGBA) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	stride := w * 4 // 32-bit rows need no padding

	var buf bytes.Buffer
	buf.Write([]byte{'B', 'M'})
	writeU32(&buf, uint32(14+40+stride*h)) // file size
	writeU32(&buf, 0)                      // reserved
	writeU32(&buf, 14+40)                  // offset of the pixel array
	writeU32(&buf, 40)                     // header size
	writeU32(&buf, uint32(w))
	writeU32(&buf, uint32(h)) // positive: bottom-up
	writeU16(&buf, 1)         // colour planes
	writeU16(&buf, 32)        // bits per pixel
	writeU32(&buf, 0)         // BI_RGB, no compression
	writeU32(&buf, uint32(stride*h))
	writeU32(&buf, 2835) // 72 dpi, as pixels per metre
	writeU32(&buf, 2835)
	writeU32(&buf, 0) // palette entries
	writeU32(&buf, 0) // ...of which none matter

	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(b.Min.X+x, b.Min.Y+y)
			buf.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	return buf.Bytes()
}

func writeU16(buf *bytes.Buffer, v uint16) { binary.Write(buf, binary.LittleEndian, v) }
func writeU32(buf *bytes.Buffer, v uint32) { binary.Write(buf, binary.LittleEndian, v) }

var numericVersionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:\.(\d+))?`)

// numericVersion is the four-part form Windows puts in a version resource,
// derived from the app's version string. lib/build wants a leading "v" and may
// carry a git suffix (v1.2.3-4-gabc123); Windows wants four plain numbers, so
// the leading dotted numbers are kept and the rest zero-filled.
func numericVersion(v string) string {
	m := numericVersionRe.FindStringSubmatch(v)
	if m == nil {
		return "0.0.0.0"
	}
	parts := []string{m[1], m[2], m[3], "0"}
	if m[4] != "" {
		parts[3] = m[4]
	}
	for i, p := range parts {
		if n, err := strconv.Atoi(p); err == nil {
			parts[i] = strconv.Itoa(n) // drop leading zeros, which Windows dislikes
		}
	}
	return strings.Join(parts, ".")
}

// isccPath finds the Inno Setup command-line compiler. The environment variable
// comes first so a build can point at a differently placed installation without
// this code growing another flag for it.
func isccPath() (string, error) {
	if p := os.Getenv("ISCC"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("ISCC is set to %q, which does not exist", p)
	}
	for _, c := range isccCandidates() {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	if p, err := exec.LookPath(isccName); err == nil {
		return p, nil
	}
	return "", fmt.Errorf(
		"cannot find %s. Install Inno Setup 6 (winget install JRSoftware.InnoSetup), "+
			"or set ISCC to its full path", isccName)
}

// isccCandidates are the places Inno Setup 6 installs itself: a machine-wide
// install under Program Files (x86) or Program Files, or a per-user one under
// %LocalAppData%\Programs, which is what winget does.
func isccCandidates() []string {
	dir := filepath.Join("Inno Setup 6", isccName)
	var out []string
	if base := os.Getenv("ProgramFiles(x86)"); base != "" {
		out = append(out, filepath.Join(base, dir))
	}
	if base := os.Getenv("ProgramFiles"); base != "" {
		out = append(out, filepath.Join(base, dir))
	}
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		out = append(out, filepath.Join(base, "Programs", dir))
	}
	return out
}
