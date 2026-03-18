package omoikane

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	defaultVersion = "v0.2.2"
	releaseBaseURL = "https://github.com/ieee0824/omoikane/releases/download"
)

type Browser struct {
	lib     *library
	handle  unsafe.Pointer
	closeMu sync.Mutex
	closed  bool
}

type library struct {
	handle         uintptr
	initFn         func() unsafe.Pointer
	freeFn         func(unsafe.Pointer)
	navigateFn     func(unsafe.Pointer, *byte) bool
	setUserAgentFn func(unsafe.Pointer, *byte) bool
	evaluateFn     func(unsafe.Pointer, *byte) *byte
	contentFn      func(unsafe.Pointer) *byte
	lastErrorFn    func(unsafe.Pointer) *byte
	stringFree     func(*byte)
}

type Options struct {
	Version     string
	UserAgent   string
	LibraryPath string
	CacheDir    string
	HTTPClient  *http.Client
}

func NewBrowser(opts ...Options) (*Browser, error) {
	options := Options{}
	if len(opts) > 0 {
		options = opts[0]
	}

	lib, err := loadLibrary(options)
	if err != nil {
		return nil, err
	}

	handle := lib.initFn()
	if handle == nil {
		return nil, errors.New("omoikane_init returned null")
	}

	browser := &Browser{
		lib:    lib,
		handle: handle,
	}

	if options.UserAgent != "" {
		if err := browser.SetUserAgent(options.UserAgent); err != nil {
			browser.Close()
			return nil, err
		}
	}

	return browser, nil
}

func (b *Browser) Navigate(url string) error {
	if err := b.ensureOpen(); err != nil {
		return err
	}

	urlPtr, freeURL := cString(url)
	defer freeURL()

	if b.lib.navigateFn(b.handle, urlPtr) {
		return nil
	}

	return b.lastError()
}

func (b *Browser) Evaluate(expression string) (json.RawMessage, error) {
	if err := b.ensureOpen(); err != nil {
		return nil, err
	}

	exprPtr, freeExpr := cString(expression)
	defer freeExpr()

	result := b.lib.evaluateFn(b.handle, exprPtr)
	if result == nil {
		return nil, b.lastError()
	}
	defer b.lib.stringFree(result)

	raw := copyCString(result)
	return json.RawMessage(raw), nil
}

func (b *Browser) SetUserAgent(userAgent string) error {
	if err := b.ensureOpen(); err != nil {
		return err
	}

	if b.lib.setUserAgentFn == nil {
		return errors.New("omoikane library does not support user agent configuration; require v0.2.2+")
	}

	userAgentPtr, freeUserAgent := cString(userAgent)
	defer freeUserAgent()

	if b.lib.setUserAgentFn(b.handle, userAgentPtr) {
		return nil
	}

	return b.lastError()
}

func (b *Browser) Content() (string, error) {
	if err := b.ensureOpen(); err != nil {
		return "", err
	}

	result := b.lib.contentFn(b.handle)
	if result == nil {
		return "", b.lastError()
	}
	defer b.lib.stringFree(result)

	return string(copyCString(result)), nil
}

func (b *Browser) Close() {
	b.closeMu.Lock()
	defer b.closeMu.Unlock()

	if b.closed {
		return
	}

	b.lib.freeFn(b.handle)
	b.closed = true
	b.handle = nil
}

func (b *Browser) ensureOpen() error {
	b.closeMu.Lock()
	defer b.closeMu.Unlock()

	if b.closed || b.handle == nil {
		return errors.New("browser is closed")
	}

	return nil
}

func (b *Browser) lastError() error {
	ptr := b.lib.lastErrorFn(b.handle)
	if ptr == nil {
		return errors.New("omoikane operation failed")
	}
	defer b.lib.stringFree(ptr)

	return errors.New(string(copyCString(ptr)))
}

func loadLibrary(opts Options) (*library, error) {
	libPath := opts.LibraryPath
	if libPath == "" {
		if envPath := os.Getenv("OMOIKANE_LIBRARY_PATH"); envPath != "" {
			libPath = envPath
		}
	}

	if libPath == "" {
		var err error
		libPath, err = ensureReleaseLibrary(opts)
		if err != nil {
			return nil, err
		}
	}

	handle, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("load omoikane library %q: %w", libPath, err)
	}

	lib := &library{handle: handle}
	purego.RegisterLibFunc(&lib.initFn, handle, "omoikane_init")
	purego.RegisterLibFunc(&lib.freeFn, handle, "omoikane_free")
	purego.RegisterLibFunc(&lib.navigateFn, handle, "omoikane_navigate")
	if _, err := purego.Dlsym(handle, "omoikane_set_user_agent"); err == nil {
		purego.RegisterLibFunc(&lib.setUserAgentFn, handle, "omoikane_set_user_agent")
	}
	purego.RegisterLibFunc(&lib.evaluateFn, handle, "omoikane_evaluate")
	purego.RegisterLibFunc(&lib.contentFn, handle, "omoikane_get_content")
	purego.RegisterLibFunc(&lib.lastErrorFn, handle, "omoikane_last_error")
	purego.RegisterLibFunc(&lib.stringFree, handle, "omoikane_string_free")

	return lib, nil
}

func ensureReleaseLibrary(opts Options) (string, error) {
	version := opts.Version
	if version == "" {
		version = defaultVersion
	}

	cacheDir := opts.CacheDir
	if cacheDir == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("resolve user cache dir: %w", err)
		}
		cacheDir = filepath.Join(userCacheDir, "omoikane-go", version)
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	libName, archiveName, err := archiveNames(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	libPath := filepath.Join(cacheDir, libName)
	if _, err := os.Stat(libPath); err == nil {
		return libPath, nil
	}

	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	archiveURL := fmt.Sprintf("%s/%s/%s", releaseBaseURL, version, archiveName)
	response, err := client.Get(archiveURL)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", archiveURL, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: unexpected status %s", archiveURL, response.Status)
	}

	if err := extractArchive(response.Body, cacheDir); err != nil {
		return "", err
	}

	if _, err := os.Stat(libPath); err != nil {
		return "", fmt.Errorf("expected extracted library %q: %w", libPath, err)
	}

	return libPath, nil
}

func archiveNames(goos, goarch string) (libName string, archiveName string, err error) {
	switch goos {
	case "darwin":
		libName = "libomoikane.dylib"
		switch goarch {
		case "amd64":
			return libName, "omoikane-macos-x86_64.tar.gz", nil
		case "arm64":
			return libName, "omoikane-macos-aarch64.tar.gz", nil
		}
	case "linux":
		libName = "libomoikane.so"
		switch goarch {
		case "amd64":
			return libName, "omoikane-linux-x86_64.tar.gz", nil
		case "arm64":
			return libName, "omoikane-linux-aarch64.tar.gz", nil
		}
	}

	return "", "", fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
}

func extractArchive(reader io.Reader, destDir string) error {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		targetPath := filepath.Join(destDir, filepath.Base(header.Name))
		switch header.Typeflag {
		case tar.TypeReg:
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create extracted file: %w", err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("write extracted file: %w", err)
			}

			if err := file.Close(); err != nil {
				return fmt.Errorf("close extracted file: %w", err)
			}
		}
	}
}

func cString(value string) (*byte, func()) {
	data := append([]byte(value), 0)
	return &data[0], func() {
		runtime.KeepAlive(data)
	}
}

func copyCString(ptr *byte) []byte {
	if ptr == nil {
		return nil
	}

	buffer := make([]byte, 0, 128)
	for offset := uintptr(0); ; offset++ {
		value := *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + offset))
		if value == 0 {
			break
		}
		buffer = append(buffer, value)
	}
	return buffer
}
