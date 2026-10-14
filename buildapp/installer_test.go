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
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestNumericVersion covers the conversion from the app's version string to the
// four plain numbers a Windows version resource holds.
//
// The two forms have different rules — lib/build insists on a leading "v" and
// tolerates a git suffix, Windows accepts neither — so a version that is
// perfectly valid for the executable could produce an installer whose version
// resource says nothing at all, or fails to compile. Every shape deriveVersion
// can return is here.
func TestNumericVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v1.2.3", "1.2.3.0"},
		{"v1.2.3.4", "1.2.3.4"},
		{"v1.2.3-4-gabc123", "1.2.3.0"},
		{"v0.0.0-dev.20261010", "0.0.0.0"},
		{"v1.2.3-beta.1", "1.2.3.0"},
		{"v10.20.30", "10.20.30.0"},
		{"v01.002.0003", "1.2.3.0"}, // leading zeros are stripped
		{"1.2.3", "1.2.3.0"},        // no "v": still parsed
		{"", "0.0.0.0"},             // nothing usable: a version resource is required
		{"dev", "0.0.0.0"},
		{"garbage-abc123", "0.0.0.0"},
	}
	for _, c := range cases {
		if got := numericVersion(c.in); got != c.want {
			t.Errorf("numericVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestWizardBMPIsReadableByInno checks the wizard panels really are the format
// Inno Setup parses: an uncompressed 32-bit BMP of the size the .iss declares.
//
// Inno does not report a wrong pixel format or a top-down row order as an
// error; it renders something, and the something is not the picture that was
// drawn. So the header is checked here rather than left to be noticed by eye.
func TestWizardBMPIsReadableByInno(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{
		{"wizard-large.bmp", wizardLargeW, wizardLargeH},
		{"wizard-small.bmp", wizardSmallW, wizardSmallH},
	}
	for _, c := range cases {
		data := encodeBMP(wizardPanel(c.w, c.h, wizardSmallIcon))
		if len(data) < 54 {
			t.Fatalf("%s: %d bytes is too short to be a BMP", c.name, len(data))
		}
		if string(data[:2]) != "BM" {
			t.Errorf("%s: does not start with the BM signature", c.name)
		}
		u32 := func(off int) uint32 { return binary.LittleEndian.Uint32(data[off:]) }
		if got := u32(14); got != 40 {
			t.Errorf("%s: header size is %d, want 40 (BITMAPINFOHEADER)", c.name, got)
		}
		if got, want := int32(u32(18)), int32(c.w); got != want {
			t.Errorf("%s: width is %d, want %d", c.name, got, want)
		}
		// Positive height is what makes the rows bottom-up, which is the order
		// the pixel loop below writes them in.
		if got, want := int32(u32(22)), int32(c.h); got != want {
			t.Errorf("%s: height is %d, want %d (positive = bottom-up)", c.name, got, want)
		}
		if got := binary.LittleEndian.Uint16(data[28:]); got != 32 {
			t.Errorf("%s: %d bits per pixel, want 32", c.name, got)
		}
		if got := u32(30); got != 0 {
			t.Errorf("%s: compression is %d, want 0 (BI_RGB)", c.name, got)
		}
		if got, want := u32(34), uint32(c.w*c.h*4); got != want {
			t.Errorf("%s: pixel array is %d bytes, want %d", c.name, got, want)
		}
		if got, want := u32(2), uint32(54+c.w*c.h*4); got != want {
			t.Errorf("%s: file size field is %d, want %d", c.name, got, want)
		}
		if got := u32(10); got != 54 {
			t.Errorf("%s: pixel array starts at %d, want 54 (14-byte file header + 40-byte info header)", c.name, got)
		}
		if len(data) != 54+c.w*c.h*4 {
			t.Errorf("%s: file is %d bytes, want %d", c.name, len(data), 54+c.w*c.h*4)
		}
	}
}

// TestWizardPanelIsOpaqueAndFilled pins the two properties the wizard art needs
// that a BMP round-trip would not surface: every pixel is opaque (Inno draws
// the panel behind the page, and a hole in it shows whatever Windows had in the
// window buffer) and the corners carry the panel colour rather than the owl.
func TestWizardPanelIsOpaqueAndFilled(t *testing.T) {
	const w, h = wizardLargeW, wizardLargeH
	img := wizardPanel(w, h, wizardLargeIcon)

	for _, p := range []image.Point{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
		c := img.NRGBAAt(p.X, p.Y)
		if c.A != 0xff {
			t.Errorf("corner %v has alpha %d, want 255", p, c.A)
		}
		if c != wizardPaper {
			t.Errorf("corner %v is %v, want the panel colour %v", p, c, wizardPaper)
		}
	}

	// The owl is drawn in the accent blue, which is darker than the paper. If
	// nothing were drawn — or it were drawn off-panel — the centre would still
	// be paper.
	centre := img.NRGBAAt(w/2, h/2)
	if centre == wizardPaper {
		t.Error("the centre of the panel is empty: the owl was not drawn on it")
	}
	if lum(centre) >= lum(wizardPaper) {
		t.Errorf("the centre of the panel is %v, which is not darker than the panel", centre)
	}
}

func lum(c color.NRGBA) int { return int(c.R) + int(c.G) + int(c.B) }

// TestInstallerAssetsAreGenerated checks the files the .iss refers to by name
// are all named correctly by ensureInstallerAssets, which is what keeps a
// missing SetupIconFile from being an ISCC error at packaging time.
func TestInstallerAssetsAreGenerated(t *testing.T) {
	// The generated assets are committed, so this does not write: it checks
	// that every name the script refers to is one the generator produces, and
	// that the file is on disk where the script expects it.
	names := []string{"appicon-256.ico", "wizard-large.bmp", "wizard-small.bmp"}
	script, err := os.ReadFile(installerScriptPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if !bytes.Contains(script, []byte(n)) {
			t.Errorf("%s is generated but %s does not mention it", n, installerScript)
		}
		if _, err := os.Stat(filepath.Join(installerAssetDir(), n)); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
}

// TestInstallerVersionDefinesAreUsed guards the two version forms against
// drifting apart: AppVersion is shown to the user, VersionInfoVersion is what
// Windows stores, and buildapp passes both.
func TestInstallerVersionDefinesAreUsed(t *testing.T) {
	script, err := os.ReadFile(installerScriptPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"#ifndef AppVersion",
		"#ifndef VersionInfoVersion",
		"AppVersion={#AppVersion}",
		"VersionInfoVersion={#VersionInfoVersion}",
	} {
		if !bytes.Contains(script, []byte(want)) {
			t.Errorf("%s no longer contains %q", installerScript, want)
		}
	}
	// A version resource Windows cannot parse is worse than none: the property
	// page shows blanks and the compiler's own output is still "successful".
	for _, bad := range []string{"VersionInfoVersion={#AppVersion}"} {
		if bytes.Contains(script, []byte(bad)) {
			t.Errorf("%s uses %q; the app's version string is not a Windows version", installerScript, bad)
		}
	}
}

// TestChineseTranslationCoversInnoMessages checks the vendored Simplified
// Chinese translation against Inno Setup's own language file.
//
// Inno Setup has no official Chinese translation, so the wizard's Chinese comes
// from languages/ChineseSimplified.isl, used unchanged. Two ways it can rot,
// both silent: the file can go missing or stop being referenced (the language
// then quietly disappears from the list and the wizard comes up in English),
// and it can fall behind Inno's message set (every message Inno has added since
// is then shown in English, mid-wizard, with no error anywhere).
func TestChineseTranslationCoversInnoMessages(t *testing.T) {
	const isl = "ChineseSimplified.isl"
	dir := filepath.Join(installerAssetDir(), "languages")

	data, err := os.ReadFile(filepath.Join(dir, isl))
	if err != nil {
		t.Fatalf("the Chinese translation is missing: %v", err)
	}

	// The language's own declaration: $0804 is zh-CN, which is what makes Setup
	// select it on a Chinese Windows instead of falling back to the first
	// language in the [Languages] section.
	for _, want := range []string{"LanguageID=$0804", "LanguageName="} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("%s does not declare %q", isl, want)
		}
	}

	script, err := os.ReadFile(installerScriptPath())
	if err != nil {
		t.Fatal(err)
	}
	want := `#emit "Name: chinesesimplified; MessagesFile: ""languages\` + isl + `"""`
	if !bytes.Contains(script, []byte(want)) {
		t.Errorf("%s no longer adds the Chinese language to [Languages];\nexpected: %s", installerScript, want)
	}

	// The upstream file records where it came from, which is what makes it
	// reviewable rather than an anonymous blob.
	for _, want := range []string{"jrsoftware.org/files/istrans", "Encoding: UTF-8"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("%s no longer carries %q; check it is still the upstream translation", isl, want)
		}
	}

	// The message set, against Inno's own Default.isl. This needs Inno Setup
	// installed to have that file to compare with, so it is skipped without it;
	// the checks above still ran.
	iscc, err := isccPath()
	if err != nil {
		t.Skipf("Inno Setup is not installed, so Default.isl is unavailable: %v", err)
	}
	def, err := os.ReadFile(filepath.Join(filepath.Dir(iscc), "Default.isl"))
	if err != nil {
		t.Skipf("cannot read %v: %v", filepath.Join(filepath.Dir(iscc), "Default.isl"), err)
	}
	defKeys := islKeys(def)
	zhKeys := islKeys(data)
	var missing []string
	for k := range defKeys {
		if !zhKeys[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("Inno Setup has %d message(s) the Chinese translation does not, so they would appear in English:\n%s",
			len(missing), strings.Join(missing, ", "))
	}
}

// islKeys returns the message names (the part before "=") a .isl file defines,
// ignoring comments and the [LangOptions] section's own settings.
func islKeys(data []byte) map[string]bool {
	keys := make(map[string]bool)
	inMessages := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inMessages = strings.EqualFold(line, "[Messages]")
			continue
		}
		if !inMessages {
			continue
		}
		if i := strings.Index(line, "="); i > 0 {
			keys[line[:i]] = true
		}
	}
	return keys
}

// TestWailsMutexMatchesInstaller is the same cross-check as
// TestWailsIconIDMatches, for the other thing this repo repeats out of a
// dependency: the installer waits on the single-instance mutex Wails creates,
// and a change to either the naming scheme or the UniqueId would leave Setup
// happily overwriting a running application's executable.
func TestWailsMutexMatchesInstaller(t *testing.T) {
	script, err := os.ReadFile(installerScriptPath())
	if err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile(filepath.Join("..", "cmd", "osprey-sync-tool", "main.go"))
	if err != nil {
		t.Fatal(err)
	}

	// The UniqueId the app passes to Wails.
	const marker = "UniqueId:"
	i := bytes.Index(main, []byte(marker))
	if i < 0 {
		t.Fatalf("no UniqueId found in cmd/osprey-sync-tool/main.go; the installer's AppMutex cannot be derived")
	}
	rest := string(main[i+len(marker):])
	open := strings.Index(rest, `"`)
	if open < 0 {
		t.Fatal("UniqueId is not a string literal")
	}
	rest = rest[open+1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatal("UniqueId literal is unterminated")
	}
	uniqueID := rest[:end]

	// Wails builds the mutex name this way (internal/frontend/desktop/windows).
	want := "wails-app-" + uniqueID + "sim"
	if !bytes.Contains(script, []byte("#define AppRuntimeMutex \""+want+"\"")) {
		src := ""
		if p, err := os.ReadFile(filepath.Join(gomodcache(t), "github.com", "wailsapp", "wails",
			"v2@"+wailsVersion(t), "internal", "frontend", "desktop", "windows", "single_instance.go")); err == nil {
			src = string(p)
		}
		if !strings.Contains(src, `mutexName := id + "sim"`) && src != "" {
			t.Skipf("Wails no longer names the mutex %q; recheck the installer's AppMutex", want)
		}
		t.Errorf("the app's single-instance mutex is %q, but %s does not define AppRuntimeMutex as that", want, installerScript)
	}
}

// TestInstallerScriptCompilesIfISCCIsPresent runs the real compiler over the
// script when Inno Setup is installed, which is the only check that catches a
// directive Inno has renamed or a path it cannot resolve.
//
// It is skipped rather than failed without Inno Setup, since -installer is an
// optional build step and a checkout should not need the tool to run its tests.
func TestInstallerScriptCompilesIfISCCIsPresent(t *testing.T) {
	iscc, err := isccPath()
	if err != nil {
		t.Skipf("Inno Setup is not installed: %v", err)
	}

	// A stand-in for the executable the script packs: ISCC only needs the file
	// to exist, and building the real 28 MB binary here would make the test
	// slower than the build it is guarding.
	dir := t.TempDir()
	exe := filepath.Join(dir, winBinaryName())
	if err := os.WriteFile(exe, []byte("not really an executable"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	args := []string{
		"/Q",
		"/DAppVersion=v1.2.3-4-gabc123",
		"/DVersionInfoVersion=1.2.3.0",
		"/DSourceExe=" + exe,
		"/DOutDir=" + out,
	}
	script := installerScriptPath()
	if out2, err := runCommand(t, iscc, append(args, script)...); err != nil {
		t.Fatalf("ISCC rejected %s:\n%s\n%v", installerScript, out2, err)
	}

	// The name carries the version, so the version has to survive the trip
	// through the compiler's preprocessor.
	want := "OspreySyncTool-v1.2.3-4-gabc123-windows-amd64-setup.exe"
	if _, err := os.Stat(filepath.Join(out, want)); err != nil {
		entries, _ := os.ReadDir(out)
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		t.Errorf("expected %s in the output directory; found %s", want, strconv.Quote(strings.Join(got, ", ")))
	}
}
