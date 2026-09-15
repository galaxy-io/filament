// Package upgrade installs newer Filament CLI release binaries.
package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	defaultReleaseURL = "https://api.github.com/repos/galaxy-io/filament/releases/latest"
	maxMetadataBytes  = 4 << 20
	maxArchiveBytes   = 256 << 20
	maxBinaryBytes    = 256 << 20
)

// ErrHomebrewManaged indicates that Homebrew, rather than Filament, must
// replace the current executable.
var ErrHomebrewManaged = errors.New("installation is managed by Homebrew")

// Options describes the current installation and release endpoint.
type Options struct {
	Client         *http.Client
	CurrentVersion string
	Executable     string
	GOOS           string
	GOARCH         string
	ReleaseURL     string
	Status         func(string) error
}

// Result reports whether Run installed the latest release.
type Result struct {
	Current string
	Latest  string
	Updated bool
}

type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Run verifies and atomically installs the latest compatible CLI release.
func Run(ctx context.Context, options Options) (Result, error) {
	options = normalizeOptions(options)
	executable, err := resolveExecutable(options.Executable)
	if err != nil {
		return Result{}, err
	}
	if homebrewManaged(executable) {
		return Result{}, fmt.Errorf("%w; run `brew upgrade galaxy-io/tap/filament`", ErrHomebrewManaged)
	}

	if err := reportStatus(options, "Checking for updates to latest version..."); err != nil {
		return Result{}, err
	}
	latest, err := fetchRelease(ctx, options.Client, options.ReleaseURL)
	if err != nil {
		return Result{}, err
	}
	result := Result{Current: options.CurrentVersion, Latest: latest.TagName}
	if upToDate(options.CurrentVersion, latest.TagName) {
		return result, nil
	}
	if err := reportStatus(options, fmt.Sprintf("Downloading Filament %s...", latest.TagName)); err != nil {
		return Result{}, err
	}

	archiveName, err := releaseArchiveName(options.GOOS, options.GOARCH)
	if err != nil {
		return Result{}, err
	}
	archiveURL, err := latest.assetURL(archiveName)
	if err != nil {
		return Result{}, err
	}
	checksumsURL, err := latest.assetURL("checksums.txt")
	if err != nil {
		return Result{}, err
	}
	archive, err := download(ctx, options.Client, archiveURL, maxArchiveBytes)
	if err != nil {
		return Result{}, fmt.Errorf("download %s: %w", archiveName, err)
	}
	checksums, err := download(ctx, options.Client, checksumsURL, maxMetadataBytes)
	if err != nil {
		return Result{}, fmt.Errorf("download checksums.txt: %w", err)
	}
	if err := reportStatus(options, "Verifying download..."); err != nil {
		return Result{}, err
	}
	if err := verifyChecksum(archiveName, archive, checksums); err != nil {
		return Result{}, err
	}
	binary, err := extractBinary(archive)
	if err != nil {
		return Result{}, err
	}
	if err := reportStatus(options, "Installing update..."); err != nil {
		return Result{}, err
	}
	if err := replaceExecutable(executable, binary); err != nil {
		return Result{}, err
	}
	result.Updated = true
	return result, nil
}

func reportStatus(options Options, message string) error {
	if options.Status == nil {
		return nil
	}
	return options.Status(message)
}

func normalizeOptions(options Options) Options {
	if options.Client == nil {
		options.Client = http.DefaultClient
	}
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.GOARCH == "" {
		options.GOARCH = runtime.GOARCH
	}
	if options.ReleaseURL == "" {
		options.ReleaseURL = defaultReleaseURL
	}
	return options
}

func resolveExecutable(path string) (string, error) {
	var err error
	if path == "" {
		path, err = os.Executable()
		if err != nil {
			return "", fmt.Errorf("locate filament executable: %w", err)
		}
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve filament executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve filament executable: %w", err)
	}
	return resolved, nil
}

func homebrewManaged(executable string) bool {
	return strings.Contains(filepath.ToSlash(executable), "/Cellar/filament/")
}

func fetchRelease(ctx context.Context, client *http.Client, url string) (release, error) {
	data, err := download(ctx, client, url, maxMetadataBytes)
	if err != nil {
		return release{}, fmt.Errorf("check latest release: %w", err)
	}
	var latest release
	if err := json.Unmarshal(data, &latest); err != nil {
		return release{}, fmt.Errorf("decode latest release: %w", err)
	}
	if !semver.IsValid(canonicalVersion(latest.TagName)) {
		return release{}, fmt.Errorf("latest release has invalid version %q", latest.TagName)
	}
	return latest, nil
}

func download(ctx context.Context, client *http.Client, url string, maximum int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "filament-cli")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("response exceeds %d bytes", maximum)
	}
	return data, nil
}

func canonicalVersion(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	return value
}

func upToDate(current, latest string) bool {
	current, latest = canonicalVersion(current), canonicalVersion(latest)
	return semver.IsValid(current) && semver.Compare(current, latest) >= 0
}

func releaseArchiveName(goos, goarch string) (string, error) {
	if (goos != "darwin" && goos != "linux") || (goarch != "amd64" && goarch != "arm64") {
		return "", fmt.Errorf("unsupported platform %s/%s", goos, goarch)
	}
	return fmt.Sprintf("filament_%s_%s.tar.gz", goos, goarch), nil
}

func (r release) assetURL(name string) (string, error) {
	for _, asset := range r.Assets {
		if asset.Name == name && asset.BrowserDownloadURL != "" {
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("release %s does not contain %s", r.TagName, name)
}

func verifyChecksum(name string, archive, checksums []byte) error {
	want := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[len(fields)-1], "*") == name {
			want = fields[0]
			break
		}
	}
	expected, err := hex.DecodeString(want)
	if err != nil || len(expected) != sha256.Size {
		return fmt.Errorf("checksums.txt does not contain a valid checksum for %s", name)
	}
	actual := sha256.Sum256(archive)
	if !bytes.Equal(expected, actual[:]) {
		return fmt.Errorf("checksum mismatch for %s", name)
	}
	return nil
}

func extractBinary(archive []byte) ([]byte, error) {
	compressed, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open release archive: %w", err)
	}
	defer func() { _ = compressed.Close() }()
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive: %w", err)
		}
		if filepath.Base(header.Name) != "filament" || header.Typeflag != tar.TypeReg {
			continue
		}
		if header.Size < 0 || header.Size > maxBinaryBytes {
			return nil, fmt.Errorf("release binary has invalid size %d", header.Size)
		}
		binary, err := io.ReadAll(io.LimitReader(reader, maxBinaryBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read release binary: %w", err)
		}
		if int64(len(binary)) != header.Size {
			return nil, errors.New("release binary is truncated")
		}
		return binary, nil
	}
	return nil, errors.New("release archive does not contain filament")
}

func replaceExecutable(path string, binary []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect current executable: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".filament-upgrade-*")
	if err != nil {
		return fmt.Errorf("prepare upgrade beside %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(binary); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write upgraded executable: %w", err)
	}
	if err := temporary.Chmod(info.Mode().Perm() | 0o500); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set upgraded executable permissions: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync upgraded executable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close upgraded executable: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
