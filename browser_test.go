package omoikane

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestArchiveNames(t *testing.T) {
	tests := []struct {
		goos    string
		goarch  string
		lib     string
		archive string
	}{
		{"darwin", "amd64", "libomoikane.dylib", "omoikane-macos-x86_64.tar.gz"},
		{"darwin", "arm64", "libomoikane.dylib", "omoikane-macos-aarch64.tar.gz"},
		{"linux", "amd64", "libomoikane.so", "omoikane-linux-x86_64.tar.gz"},
		{"linux", "arm64", "libomoikane.so", "omoikane-linux-aarch64.tar.gz"},
	}

	for _, tt := range tests {
		lib, archive, err := archiveNames(tt.goos, tt.goarch)
		if err != nil {
			t.Fatalf("%s/%s: unexpected error: %v", tt.goos, tt.goarch, err)
		}
		if lib != tt.lib || archive != tt.archive {
			t.Fatalf("%s/%s: got %q %q", tt.goos, tt.goarch, lib, archive)
		}
	}
}

func TestArchiveNamesRejectsUnsupportedPlatform(t *testing.T) {
	if _, _, err := archiveNames("windows", "amd64"); err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}

func TestBrowserWithLocalLibrary(t *testing.T) {
	libPath := os.Getenv("OMOIKANE_LIBRARY_PATH")
	if libPath == "" {
		root := os.Getenv("OMOIKANE_RUST_REPO")
		if root != "" {
			libPath = filepath.Join(root, "target", "debug", libraryFileName(runtime.GOOS))
		}
	}
	if libPath == "" {
		t.Skip("set OMOIKANE_LIBRARY_PATH or OMOIKANE_RUST_REPO to run integration test")
	}

	browser, err := NewBrowser(Options{LibraryPath: libPath})
	if err != nil {
		t.Fatalf("NewBrowser: %v", err)
	}
	defer browser.Close()

	if err := browser.Navigate(`data:text/html,<html><body><main id="app">hello</main></body></html>`); err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	result, err := browser.Evaluate(`document.getElementById("app").nodeName`)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !strings.Contains(string(result), `"MAIN"`) {
		t.Fatalf("unexpected evaluate payload: %s", string(result))
	}

	content, err := browser.Content()
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	if !strings.Contains(content, `<main id="app">hello</main>`) {
		t.Fatalf("unexpected content: %s", content)
	}
}

func libraryFileName(goos string) string {
	if goos == "darwin" {
		return "libomoikane.dylib"
	}
	return "libomoikane.so"
}
