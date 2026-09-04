package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCompareSemanticVersions(t *testing.T) {
	for _, test := range []struct {
		left  string
		right string
		want  int
	}{
		{left: "1.0.3", right: "1.0.2", want: 1},
		{left: "v1.0.3", right: "1.0.3", want: 0},
		{left: "1.0.3", right: "1.0.3-test", want: 1},
		{left: "1.0.3-test", right: "1.0.3", want: -1},
		{left: "1.0.3-rc.10", right: "1.0.3-rc.2", want: 1},
		{left: "1.10.0", right: "1.9.9", want: 1},
	} {
		got, err := compareSemanticVersions(test.left, test.right)
		if err != nil || got != test.want {
			t.Errorf("compareSemanticVersions(%q, %q) = %d, %v; want %d", test.left, test.right, got, err, test.want)
		}
	}
	if _, err := compareSemanticVersions("latest", "1.0.0"); err == nil {
		t.Fatal("invalid version was accepted")
	}
}

func TestFetchAndDownloadVerifiedUpdate(t *testing.T) {
	payload := []byte("verified update payload")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	var downloads atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(githubRelease{
				TagName: "v1.1.0", HTMLURL: server.URL + "/release", PublishedAt: "2026-09-04T00:00:00Z",
				Assets: []githubReleaseAsset{{
					Name: "ZCode-Antigravity-macOS-Universal-v1.1.0.zip", Size: int64(len(payload)),
					BrowserDownloadURL: server.URL + "/asset", Digest: "sha256:" + digest,
				}},
			})
		case "/asset":
			downloads.Add(1)
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	candidate, err := fetchLatestUpdate(context.Background(), server.Client(), server.URL+"/latest", "1.0.3-test", "darwin", true)
	if err != nil {
		t.Fatal(err)
	}
	if !candidate.Report.Available || candidate.Report.LatestVersion != "1.1.0" || candidate.Report.AssetSize != int64(len(payload)) || candidate.SHA256 != digest {
		t.Fatalf("candidate = %#v", candidate)
	}
	dataRoot := t.TempDir()
	download, err := downloadVerifiedUpdate(context.Background(), server.Client(), candidate, dataRoot, "darwin", true)
	if err != nil {
		t.Fatal(err)
	}
	if download.Version != "1.1.0" || download.Platform != "darwin" || download.SHA256 != digest {
		t.Fatalf("download = %#v", download)
	}
	raw, err := os.ReadFile(download.Path)
	if err != nil || string(raw) != string(payload) {
		t.Fatalf("downloaded payload = %q, %v", raw, err)
	}
	if !strings.HasPrefix(download.Path, filepath.Join(dataRoot, "updates", "1.1.0")+string(filepath.Separator)) {
		t.Fatalf("download escaped update root: %s", download.Path)
	}
	if _, err := downloadVerifiedUpdate(context.Background(), server.Client(), candidate, dataRoot, "darwin", true); err != nil {
		t.Fatal(err)
	}
	if downloads.Load() != 1 {
		t.Fatalf("verified cached asset was downloaded %d times", downloads.Load())
	}
}

func TestUpdateRejectsUntrustedOrMismatchedAssets(t *testing.T) {
	if trustedAssetURL("https://evil.example/ZCode-Antigravity-Setup-v1.1.0.exe", "ZCode-Antigravity-Setup-v1.1.0.exe", false) {
		t.Fatal("untrusted update host was accepted")
	}
	if !trustedAssetURL("https://github.com/Hhz0823/ZCode-Antigravity/releases/download/v1.1.0/ZCode-Antigravity-Setup-v1.1.0.exe", "ZCode-Antigravity-Setup-v1.1.0.exe", false) {
		t.Fatal("official update asset was rejected")
	}
	if _, err := parseSHA256Digest("sha256:abcd"); err == nil {
		t.Fatal("short digest was accepted")
	}
	payload := []byte("tampered")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload) }))
	defer server.Close()
	candidate := updateCandidate{
		Report:      updateReport{Available: true, LatestVersion: "1.1.0", AssetName: "ZCode-Antigravity-macOS-Universal-v1.1.0.zip", AssetSize: int64(len(payload))},
		DownloadURL: server.URL,
		SHA256:      strings.Repeat("0", 64),
	}
	if _, err := downloadVerifiedUpdate(context.Background(), server.Client(), candidate, t.TempDir(), "darwin", true); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("tampered asset error = %v", err)
	}
}
