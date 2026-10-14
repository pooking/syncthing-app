// Copyright (C) 2025 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build tools

// Command buildapp packages OspreySyncTool for the desktop platforms. It does not
// touch the upstream build.go behaviour; it only re-uses its conventions for
// version stamping and Windows resources.
//
// Usage:
//
//	go run ./buildapp                       # build for the host platform
//	go run ./buildapp -goos windows -goarch amd64 -zip
//	go run ./buildapp -goos windows -goarch amd64 -installer
//	go run ./buildapp -all                  # windows, linux and macOS archives
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"

	"github.com/syncthing/syncthing/desktop/brand"
)

const (
	appName    = "OspreySyncTool"
	binaryName = "osprey-sync-tool"
	mainPkg    = "./cmd/osprey-sync-tool"
	// appDescription is what Windows shows for the executable in the Details
	// tab and in Task Manager.
	appDescription = "OspreySyncTool — portable file synchronisation"
	// desktopTags: "production" is required by Wails for manual `go build`
	// (without it Wails compiles a stub that only shows an error dialog), and
	// "noassets" skips embedding the unused upstream web GUI.
	desktopTags = "noassets,production"

	// wailsAppIconID is the resource ID Wails looks the window icon up by
	// (internal/frontend/desktop/windows/winc.AppIconID). It is not exported,
	// so it is repeated here, and TestWailsIconIDMatches guards against it
	// moving.
	wailsAppIconID = 3
)

var (
	goos    = flag.String("goos", runtime.GOOS, "target operating system")
	goarch  = flag.String("goarch", runtime.GOARCH, "target architecture")
	outDir  = flag.String("out", "dist", "output directory")
	verFlag = flag.String("version", "", "version to stamp (default: derived from git)")
	doZip   = flag.Bool("zip", false, "produce a portable zip archive")
	doSetup = flag.Bool("installer", false, "also compile a Windows setup executable (needs Inno Setup 6)")
	all     = flag.Bool("all", false, "build windows/amd64, linux/amd64 and darwin/universal archives")
	skipGui = flag.Bool("cli-only", false, "build only the headless harness (useful for testing)")
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	if *all {
		// Windows is the only target that cross-compiles without a C
		// toolchain, so -all is mainly useful on a machine that has one.
		targets := []struct{ goos, goarch string }{
			{"windows", "amd64"},
			{"linux", "amd64"},
			{"darwin", "arm64"},
		}
		var failed []string
		for _, t := range targets {
			if err := build(t.goos, t.goarch, true); err != nil {
				log.Printf("skipping %s/%s: %v", t.goos, t.goarch, err)
				failed = append(failed, t.goos+"/"+t.goarch)
			}
		}
		if len(failed) == len(targets) {
			log.Fatalf("no targets built")
		}
		if len(failed) > 0 {
			log.Printf("built everything except: %s", strings.Join(failed, ", "))
		}
		return
	}

	if err := build(*goos, *goarch, *doZip); err != nil {
		log.Fatal(err)
	}
	if *doSetup {
		// Deliberately outside build(): the executable has to be linked and its
		// resources written before the installer can pack it, and the installer
		// packs exactly the file build() writes for windows/amd64 — so asking
		// for it from a build for another platform is an error, not a mystery.
		// Both sides derive the version the same way, so they agree without
		// being told to.
		if err := checkInstallerHost(); err != nil {
			log.Fatal(err)
		}
		ver := *verFlag
		if ver == "" {
			ver = deriveVersion()
		}
		if err := buildInstaller(*outDir, ver); err != nil {
			log.Fatal(err)
		}
	}
}

func build(goosValue, goarchValue string, zipUp bool) error {
	ver := *verFlag
	if ver == "" {
		ver = deriveVersion()
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}

	pkg := mainPkg
	if *skipGui {
		pkg = "./cmd/syncthing-cli"
	}

	bin := binaryName
	if goosValue == "windows" {
		bin += ".exe"
	}
	binPath := filepath.Join(*outDir, fmt.Sprintf("%s-%s-%s", binaryName, goosValue, goarchValue))
	if goosValue == "windows" {
		binPath += ".exe"
	}

	parts := []string{
		"-s", "-w",
		"-X", "github.com/syncthing/syncthing/lib/build.Version=" + ver,
		"-X", "github.com/syncthing/syncthing/lib/build.Tags=noupgrade",
	}
	// A GUI app must not open a console window on Windows. "-H windowsgui"
	// marks the binary as a GUI subsystem executable, so launching it from
	// Explorer (or at login) shows no black terminal. The headless CLI build
	// keeps its console.
	if goosValue == "windows" && !*skipGui {
		parts = append(parts, "-H", "windowsgui")
	}
	ldflags := strings.Join(parts, " ")

	tags := desktopTags
	if *skipGui {
		tags = "noassets"
	}
	args := []string{"build", "-trimpath", "-tags", tags, "-ldflags", ldflags, "-o", binPath, pkg}
	// The Windows desktop build is pure Go: modernc.org/sqlite, the WebView2
	// bindings and the Win32 tray need no C toolchain, so it cross-compiles
	// from anywhere. The Linux and macOS desktop builds are a different
	// story - Wails links GTK/WebKit on Linux and Cocoa on macOS, and
	// fyne.io/systray is cgo on both - so they need a C toolchain and a real
	// host (or a cross toolchain) rather than CGO_ENABLED=0.
	cgo := "0"
	if !*skipGui && (goosValue == "linux" || goosValue == "darwin") {
		if cc := cCompiler(goosValue); cc == "" {
			return fmt.Errorf(
				"building the %s desktop app needs a C toolchain (%s) and the platform's GUI "+
					"libraries; build it on that platform instead, or use -cli-only for the "+
					"headless harness",
				goosValue, ccName(goosValue))
		}
		cgo = "1"
	}

	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(),
		"GOOS="+goosValue,
		"GOARCH="+goarchValue,
		"CGO_ENABLED="+cgo,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = repoRoot()
	log.Printf("building %s/%s -> %s", goosValue, goarchValue, binPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	// Windows draws an executable's icon, its Details-tab strings and its DPI
	// behaviour from resources embedded in the file. `go build` puts none of
	// them there, so they are written in afterwards.
	if goosValue == "windows" && !*skipGui {
		if err := stampWindowsResources(binPath, ver); err != nil {
			return fmt.Errorf("write Windows resources: %w", err)
		}
	}

	if zipUp {
		archive := filepath.Join(*outDir, fmt.Sprintf("%s-%s-%s.zip", appName, goosValue, goarchValue))
		if err := writePortableZip(archive, binPath, filepath.Base(binPath)); err != nil {
			return err
		}
		log.Printf("wrote %s", archive)
	}
	return nil
}

// stampWindowsResources writes the icon and version information into the built
// executable.
//
// The icon goes in as a resource rather than being passed to Wails, because
// Wails only reads one from the project layout its own CLI generates — and this
// app is built with `go build`, so there is no project layout to put it in.
// A resource is also what Windows actually draws: Explorer, the taskbar and the
// title bar all read the executable's icon, and without one they show the
// generic application icon no matter what the window is titled.
//
// The version strings are a bonus of doing it this way: they are what the
// Details tab of the file's Properties shows, and what Task Manager displays.
func stampWindowsResources(exePath, ver string) error {
	// The art is drawn in Go (desktop/brand), so the build needs no image file
	// on disk and there is nothing to keep in step with it.
	rs := winres.ResourceSet{}
	ico, err := winres.NewIconFromImages(brand.TileImages(brand.ICOSizes))
	if err != nil {
		return err
	}
	// It goes in under the name Wails looks an application icon up by.
	//
	// ID 3 is winc.AppIconID. Its lookup failure is silent, and nothing else in
	// the app ever registers a window class icon, so this resource is the only
	// icon the title bar and the taskbar button have to work with. ID 1 — the
	// convention every resource compiler follows — is deliberately *not* also
	// registered: two groups doubled the resource section for no benefit, and
	// there is a limit to how much a PE section can hold before Windows stops
	// reading it.
	if err := rs.SetIcon(winres.ID(wailsAppIconID), ico); err != nil {
		return err
	}

	vi := version.Info{}
	vi.SetFileVersion(ver)
	vi.SetProductVersion(ver)
	for _, kv := range [][2]string{
		{version.ProductName, appName},
		{version.FileDescription, appDescription},
		{version.CompanyName, appName},
		{version.InternalName, binaryName},
		{version.OriginalFilename, filepath.Base(exePath)},
	} {
		if err := vi.Set(version.LangDefault, kv[0], kv[1]); err != nil {
			return err
		}
	}
	rs.SetVersionInfo(vi)

	// Per-monitor DPI awareness is what the webview needs to stay sharp on a
	// scaled display; the common controls v6 dependency is what gives the
	// native dialogs their modern look.
	rs.SetManifest(winres.AppManifest{
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true,
	})

	// winres can also produce a linkable object, but this executable has
	// already been linked. WriteToEXE restates its resource section instead,
	// which is the same content and skips a second link entirely.
	return replaceResources(exePath, rs)
}

// replaceResources rewrites the executable's resource section in place. The new
// file is written beside it and renamed over the top, so a failure part way
// through leaves the built executable rather than a truncated one.
func replaceResources(exePath string, rs winres.ResourceSet) error {
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, ".osprey-res-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	src, err := os.Open(exePath)
	if err != nil {
		tmp.Close()
		return err
	}
	err = rs.WriteToEXE(tmp, src)
	src.Close()
	if err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, exePath)
}

// writePortableZip produces the "green" package: the executable plus an empty
// marker file that tells the app to keep all state next to the executable.
func writePortableZip(archive, binPath, binName string) error {
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	src, err := os.Open(binPath)
	if err != nil {
		return err
	}
	defer src.Close()

	if err := addZipEntry(zw, src, binName, 0o755); err != nil {
		return err
	}
	// The portable marker: its presence switches the app into portable mode.
	if err := addZipEntry(zw, strings.NewReader(""), appName+".portable", 0o644); err != nil {
		return err
	}
	return nil
}

func addZipEntry(zw *zip.Writer, r io.Reader, name string, mode os.FileMode) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
	hdr.SetMode(mode)
	hdr.Modified = time.Now()
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}

// deriveVersion produces a version string that satisfies lib/build's parser,
// which insists on a leading "v" and dotted numeric components. A git
// description such as "v1.2.3-4-gabc123" is kept when it already fits; anything
// else falls back to a date-based development version.
func deriveVersion() string {
	out, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
	if err == nil {
		if v := normalizeVersion(strings.TrimSpace(string(out))); v != "" {
			return v
		}
	}
	return "v0.0.0-dev." + time.Now().UTC().Format("20060102")
}

var versionRe = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[a-z0-9]+)*(\.\d+)*(\+\d+-g[0-9a-f]+|\+[0-9a-z]+)?(-[^\s]+)?$`)

// normalizeVersion trims and validates a candidate version, returning "" when
// it cannot be used as-is.
func normalizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	v = strings.ReplaceAll(v, " ", "")
	if v == "" {
		return ""
	}
	candidate := "v" + v
	if versionRe.MatchString(candidate) {
		return candidate
	}
	return ""
}

// ccName is the conventional C compiler name for a target platform, used in
// error messages.
func ccName(goosValue string) string {
	if goosValue == "darwin" {
		return "clang"
	}
	return "gcc"
}

// cCompiler reports whether a C compiler for the target is available on PATH.
// Cross-compiling to macOS from Windows or Linux is not supported by the Go
// toolchain here, so a missing compiler means "build it on that platform".
func cCompiler(goosValue string) string {
	candidates := []string{"cc"}
	if goosValue == "darwin" {
		candidates = []string{"clang", "cc"}
	} else {
		candidates = []string{"gcc", "cc"}
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
