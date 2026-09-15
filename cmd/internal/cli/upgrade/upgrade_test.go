package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestRunReplacesExecutableWithVerifiedRelease(t *testing.T) {
	t.Parallel()
	statuses := []string{}
	archive := releaseArchive(t, []byte("new filament"))
	digest := sha256.Sum256(archive)
	archiveName := "filament_darwin_arm64.tar.gz"
	client := testHTTPClient(func(request *http.Request) *http.Response {
		switch request.URL.Path {
		case "/latest":
			return testResponse(http.StatusOK, fmt.Sprintf(`{"tag_name":"v1.2.0","assets":[{"name":%q,"browser_download_url":"https://example.test/archive"},{"name":"checksums.txt","browser_download_url":"https://example.test/checksums"}]}`,
				archiveName))
		case "/archive":
			return testResponse(http.StatusOK, string(archive))
		case "/checksums":
			return testResponse(http.StatusOK, fmt.Sprintf("%s  %s\n", hex.EncodeToString(digest[:]), archiveName))
		default:
			return testResponse(http.StatusNotFound, "not found")
		}
	})

	executable := filepath.Join(t.TempDir(), "filament")
	if err := os.WriteFile(executable, []byte("old filament"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), Options{
		Client: client, CurrentVersion: "v1.1.0", Executable: executable,
		GOOS: "darwin", GOARCH: "arm64", ReleaseURL: "https://example.test/latest",
		Status: func(message string) error {
			statuses = append(statuses, message)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated || result.Latest != "v1.2.0" {
		t.Fatalf("result = %#v", result)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new filament" {
		t.Fatalf("executable = %q", data)
	}
	wantStatuses := []string{
		"Checking for updates to latest version...",
		"Downloading Filament v1.2.0...",
		"Verifying download...",
		"Installing update...",
	}
	if fmt.Sprint(statuses) != fmt.Sprint(wantStatuses) {
		t.Fatalf("statuses = %v, want %v", statuses, wantStatuses)
	}
}

func TestRunDoesNotReplaceCurrentRelease(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(*http.Request) *http.Response {
		return testResponse(http.StatusOK, `{"tag_name":"v1.2.0"}`)
	})
	executable := filepath.Join(t.TempDir(), "filament")
	if err := os.WriteFile(executable, []byte("current filament"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), Options{
		Client: client, CurrentVersion: "1.2.0", Executable: executable,
		GOOS: "linux", GOARCH: "amd64", ReleaseURL: "https://example.test/latest",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated {
		t.Fatalf("result = %#v, want no update", result)
	}
}

func TestRunRejectsChecksumMismatchWithoutReplacingExecutable(t *testing.T) {
	t.Parallel()
	archive := releaseArchive(t, []byte("new filament"))
	archiveName := "filament_linux_amd64.tar.gz"
	client := testHTTPClient(func(request *http.Request) *http.Response {
		switch request.URL.Path {
		case "/latest":
			return testResponse(http.StatusOK, fmt.Sprintf(`{"tag_name":"v1.2.0","assets":[{"name":%q,"browser_download_url":"https://example.test/archive"},{"name":"checksums.txt","browser_download_url":"https://example.test/checksums"}]}`,
				archiveName))
		case "/archive":
			return testResponse(http.StatusOK, string(archive))
		case "/checksums":
			return testResponse(http.StatusOK, fmt.Sprintf("%064d  %s\n", 0, archiveName))
		default:
			return testResponse(http.StatusNotFound, "not found")
		}
	})
	executable := filepath.Join(t.TempDir(), "filament")
	if err := os.WriteFile(executable, []byte("old filament"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Options{
		Client: client, CurrentVersion: "v1.1.0", Executable: executable,
		GOOS: "linux", GOARCH: "amd64", ReleaseURL: "https://example.test/latest",
	})
	if err == nil {
		t.Fatal("Run() error = nil, want checksum mismatch")
	}
	data, readErr := os.ReadFile(executable)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "old filament" {
		t.Fatalf("executable changed after failed verification: %q", data)
	}
}

func TestRunRejectsHomebrewManagedExecutable(t *testing.T) {
	t.Parallel()
	executable := filepath.Join(t.TempDir(), "Cellar", "filament", "1.0.0", "bin", "filament")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("filament"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Options{Executable: executable})
	if !errors.Is(err, ErrHomebrewManaged) {
		t.Fatalf("Run() error = %v, want ErrHomebrewManaged", err)
	}
}

func releaseArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var data bytes.Buffer
	compressed := gzip.NewWriter(&data)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{Name: "filament", Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

type roundTripFunc func(*http.Request) *http.Response

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request), nil
}

func testHTTPClient(roundTrip roundTripFunc) *http.Client {
	return &http.Client{Transport: roundTrip}
}

func testResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}
