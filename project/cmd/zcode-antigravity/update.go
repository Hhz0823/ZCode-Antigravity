package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	githubLatestReleaseAPI = "https://api.github.com/repos/Hhz0823/ZCode-Antigravity/releases/latest"
	updateMaxMetadataBytes = 2 << 20
	updateMaxAssetBytes    = 600 << 20
)

var semanticVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

type githubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Draft       bool                 `json:"draft"`
	Prerelease  bool                 `json:"prerelease"`
	PublishedAt string               `json:"published_at"`
	HTMLURL     string               `json:"html_url"`
	Assets      []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type updateReport struct {
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion"`
	Available      bool      `json:"available"`
	PublishedAt    string    `json:"publishedAt,omitempty"`
	ReleaseURL     string    `json:"releaseURL"`
	AssetName      string    `json:"assetName"`
	AssetSize      int64     `json:"assetSize"`
	CheckedAt      time.Time `json:"checkedAt"`
}

type updateDownload struct {
	Version   string `json:"version"`
	Platform  string `json:"platform"`
	AssetName string `json:"assetName"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
}

type updateCandidate struct {
	Report      updateReport
	DownloadURL string
	SHA256      string
}

type updateActionRequest struct {
	Action string `json:"action"`
}

type semVersion struct {
	major      int
	minor      int
	patch      int
	prerelease []string
}

func parseSemanticVersion(value string) (semVersion, error) {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V"))
	if !semanticVersionPattern.MatchString(value) {
		return semVersion{}, fmt.Errorf("版本号 %q 不是受支持的语义版本", value)
	}
	withoutBuild := strings.SplitN(value, "+", 2)[0]
	parts := strings.SplitN(withoutBuild, "-", 2)
	core := strings.Split(parts[0], ".")
	major, err := strconv.Atoi(core[0])
	if err != nil {
		return semVersion{}, fmt.Errorf("版本号主版本过大")
	}
	minor, err := strconv.Atoi(core[1])
	if err != nil {
		return semVersion{}, fmt.Errorf("版本号次版本过大")
	}
	patch, err := strconv.Atoi(core[2])
	if err != nil {
		return semVersion{}, fmt.Errorf("版本号修订号过大")
	}
	parsed := semVersion{major: major, minor: minor, patch: patch}
	if len(parts) == 2 {
		parsed.prerelease = strings.Split(parts[1], ".")
	}
	return parsed, nil
}

func compareSemanticVersions(left, right string) (int, error) {
	a, err := parseSemanticVersion(left)
	if err != nil {
		return 0, err
	}
	b, err := parseSemanticVersion(right)
	if err != nil {
		return 0, err
	}
	for _, pair := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1, nil
		}
		if pair[0] > pair[1] {
			return 1, nil
		}
	}
	if len(a.prerelease) == 0 && len(b.prerelease) == 0 {
		return 0, nil
	}
	if len(a.prerelease) == 0 {
		return 1, nil
	}
	if len(b.prerelease) == 0 {
		return -1, nil
	}
	for index := 0; index < len(a.prerelease) && index < len(b.prerelease); index++ {
		leftPart, rightPart := a.prerelease[index], b.prerelease[index]
		if leftPart == rightPart {
			continue
		}
		leftNumber, leftNumeric := numericIdentifier(leftPart)
		rightNumber, rightNumeric := numericIdentifier(rightPart)
		if leftNumeric && rightNumeric {
			if leftNumber < rightNumber {
				return -1, nil
			}
			return 1, nil
		}
		if leftNumeric {
			return -1, nil
		}
		if rightNumeric {
			return 1, nil
		}
		if leftPart < rightPart {
			return -1, nil
		}
		return 1, nil
	}
	if len(a.prerelease) < len(b.prerelease) {
		return -1, nil
	}
	if len(a.prerelease) > len(b.prerelease) {
		return 1, nil
	}
	return 0, nil
}

func numericIdentifier(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	number, err := strconv.Atoi(value)
	return number, err == nil
}

func normalizedReleaseVersion(tag string) (string, error) {
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(tag, "v"), "V"))
	if _, err := parseSemanticVersion(value); err != nil {
		return "", err
	}
	return value, nil
}

func expectedUpdateAsset(goos, releaseVersion string) (string, error) {
	switch goos {
	case "darwin":
		return "ZCode-Antigravity-macOS-Universal-v" + releaseVersion + ".zip", nil
	case "windows":
		return "ZCode-Antigravity-Setup-v" + releaseVersion + ".exe", nil
	default:
		return "", fmt.Errorf("当前平台 %s 不支持自动更新", goos)
	}
}

func parseSHA256Digest(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(value, "sha256:") {
		return "", fmt.Errorf("更新资产缺少 GitHub SHA-256 摘要")
	}
	value = strings.TrimPrefix(value, "sha256:")
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("更新资产的 SHA-256 摘要无效")
	}
	return value, nil
}

func trustedUpdateURL(value *url.URL, allowInsecure bool) bool {
	if value == nil || value.User != nil || value.Fragment != "" {
		return false
	}
	if allowInsecure {
		return value.Scheme == "http" || value.Scheme == "https"
	}
	if value.Scheme != "https" {
		return false
	}
	switch strings.ToLower(value.Hostname()) {
	case "api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}

func trustedReleasePage(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.Scheme == "https" && strings.EqualFold(parsed.Hostname(), "github.com") &&
		strings.HasPrefix(parsed.EscapedPath(), "/Hhz0823/ZCode-Antigravity/releases/") && parsed.User == nil
}

func trustedAssetURL(rawURL, assetName string, allowInsecure bool) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || !trustedUpdateURL(parsed, allowInsecure) {
		return false
	}
	if allowInsecure {
		return true
	}
	prefix := "/Hhz0823/ZCode-Antigravity/releases/download/"
	return strings.EqualFold(parsed.Hostname(), "github.com") && strings.HasPrefix(parsed.EscapedPath(), prefix) && strings.HasSuffix(parsed.EscapedPath(), "/"+url.PathEscape(assetName))
}

func (a *app) updateHTTPClient(timeout time.Duration, allowInsecure bool) (*http.Client, error) {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DisableCompression:  false,
		MaxIdleConns:        4,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	if proxyValue, _ := a.resolveProxy(); strings.TrimSpace(proxyValue) != "" {
		proxyURL, err := url.Parse(proxyValue)
		if err != nil {
			return nil, fmt.Errorf("更新代理地址无效: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > 8 || !trustedUpdateURL(request.URL, allowInsecure) {
				return fmt.Errorf("更新下载重定向到不受信任的地址")
			}
			return nil
		},
	}, nil
}

func (a *app) latestUpdate(ctx context.Context) (updateCandidate, error) {
	client, err := a.updateHTTPClient(20*time.Second, false)
	if err != nil {
		return updateCandidate{}, err
	}
	return fetchLatestUpdate(ctx, client, githubLatestReleaseAPI, version, runtime.GOOS, false)
}

func fetchLatestUpdate(ctx context.Context, client *http.Client, apiURL, currentVersion, goos string, allowInsecure bool) (updateCandidate, error) {
	parsedAPI, err := url.Parse(apiURL)
	if err != nil || !trustedUpdateURL(parsedAPI, allowInsecure) {
		return updateCandidate{}, fmt.Errorf("更新检查地址不受信任")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return updateCandidate{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "ZCode-Antigravity/"+currentVersion)
	response, err := client.Do(request)
	if err != nil {
		return updateCandidate{}, fmt.Errorf("检查 GitHub 更新: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return updateCandidate{}, fmt.Errorf("GitHub 更新接口返回 HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, updateMaxMetadataBytes+1))
	if err != nil {
		return updateCandidate{}, err
	}
	if len(raw) > updateMaxMetadataBytes {
		return updateCandidate{}, fmt.Errorf("GitHub 更新元数据过大")
	}
	var release githubRelease
	if err := json.Unmarshal(raw, &release); err != nil {
		return updateCandidate{}, fmt.Errorf("GitHub 更新元数据无效: %w", err)
	}
	if release.Draft || release.Prerelease {
		return updateCandidate{}, fmt.Errorf("GitHub latest 返回了非正式版")
	}
	latestVersion, err := normalizedReleaseVersion(release.TagName)
	if err != nil {
		return updateCandidate{}, err
	}
	comparison, err := compareSemanticVersions(latestVersion, currentVersion)
	if err != nil {
		return updateCandidate{}, err
	}
	assetName, err := expectedUpdateAsset(goos, latestVersion)
	if err != nil {
		return updateCandidate{}, err
	}
	var selected *githubReleaseAsset
	for index := range release.Assets {
		if release.Assets[index].Name == assetName {
			selected = &release.Assets[index]
			break
		}
	}
	if selected == nil {
		return updateCandidate{}, fmt.Errorf("最新版缺少当前平台资产 %s", assetName)
	}
	if selected.Size <= 0 || selected.Size > updateMaxAssetBytes {
		return updateCandidate{}, fmt.Errorf("更新资产大小无效")
	}
	if !trustedAssetURL(selected.BrowserDownloadURL, assetName, allowInsecure) {
		return updateCandidate{}, fmt.Errorf("更新资产下载地址不受信任")
	}
	digest, err := parseSHA256Digest(selected.Digest)
	if err != nil {
		return updateCandidate{}, err
	}
	if !trustedReleasePage(release.HTMLURL) && !allowInsecure {
		return updateCandidate{}, fmt.Errorf("更新说明地址不受信任")
	}
	return updateCandidate{
		Report: updateReport{
			CurrentVersion: currentVersion,
			LatestVersion:  latestVersion,
			Available:      comparison > 0,
			PublishedAt:    release.PublishedAt,
			ReleaseURL:     release.HTMLURL,
			AssetName:      assetName,
			AssetSize:      selected.Size,
			CheckedAt:      time.Now().UTC(),
		},
		DownloadURL: selected.BrowserDownloadURL,
		SHA256:      digest,
	}, nil
}

func (a *app) downloadLatestUpdate(ctx context.Context, candidate updateCandidate) (updateDownload, error) {
	client, err := a.updateHTTPClient(20*time.Minute, false)
	if err != nil {
		return updateDownload{}, err
	}
	return downloadVerifiedUpdate(ctx, client, candidate, a.paths.Data, runtime.GOOS, false)
}

func downloadVerifiedUpdate(ctx context.Context, client *http.Client, candidate updateCandidate, dataRoot, platform string, allowInsecure bool) (updateDownload, error) {
	if !candidate.Report.Available {
		return updateDownload{}, fmt.Errorf("当前已是最新版")
	}
	if !trustedAssetURL(candidate.DownloadURL, candidate.Report.AssetName, allowInsecure) {
		return updateDownload{}, fmt.Errorf("更新资产下载地址不受信任")
	}
	updatesRoot := filepath.Join(dataRoot, "updates", candidate.Report.LatestVersion)
	if err := os.MkdirAll(updatesRoot, 0o700); err != nil {
		return updateDownload{}, fmt.Errorf("创建更新目录: %w", err)
	}
	target := filepath.Join(updatesRoot, candidate.Report.AssetName)
	if ok, _ := fileMatchesUpdate(target, candidate.SHA256, candidate.Report.AssetSize); ok {
		return updateDownload{Version: candidate.Report.LatestVersion, Platform: platform, AssetName: candidate.Report.AssetName, Path: target, SHA256: candidate.SHA256}, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.DownloadURL, nil)
	if err != nil {
		return updateDownload{}, err
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "ZCode-Antigravity/"+version)
	response, err := client.Do(request)
	if err != nil {
		return updateDownload{}, fmt.Errorf("下载更新: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return updateDownload{}, fmt.Errorf("下载更新返回 HTTP %d", response.StatusCode)
	}
	if response.ContentLength > updateMaxAssetBytes || response.ContentLength > 0 && response.ContentLength != candidate.Report.AssetSize {
		return updateDownload{}, fmt.Errorf("更新资产长度与 GitHub 元数据不一致")
	}
	temporary, err := os.CreateTemp(updatesRoot, ".update-*.part")
	if err != nil {
		return updateDownload{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(response.Body, updateMaxAssetBytes+1))
	if copyErr != nil {
		_ = temporary.Close()
		return updateDownload{}, fmt.Errorf("写入更新资产: %w", copyErr)
	}
	if written != candidate.Report.AssetSize || written > updateMaxAssetBytes {
		_ = temporary.Close()
		return updateDownload{}, fmt.Errorf("更新资产大小与 GitHub 元数据不一致")
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actualDigest, candidate.SHA256) {
		_ = temporary.Close()
		return updateDownload{}, fmt.Errorf("更新资产 SHA-256 校验失败")
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return updateDownload{}, err
	}
	if err := temporary.Close(); err != nil {
		return updateDownload{}, err
	}
	if err := os.Chmod(temporaryPath, 0o700); err != nil {
		return updateDownload{}, err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return updateDownload{}, err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return updateDownload{}, fmt.Errorf("完成更新下载: %w", err)
	}
	return updateDownload{Version: candidate.Report.LatestVersion, Platform: platform, AssetName: candidate.Report.AssetName, Path: target, SHA256: candidate.SHA256}, nil
}

func fileMatchesUpdate(path, expectedDigest string, expectedSize int64) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != expectedSize {
		return false, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedDigest), nil
}

func (g *guiRuntime) serveUpdate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		candidate, err := g.app.latestUpdate(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, candidate.Report)
	case http.MethodPost:
		var request updateActionRequest
		decoder := json.NewDecoder(io.LimitReader(r.Body, 8<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil || strings.ToLower(strings.TrimSpace(request.Action)) != "download" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "更新请求无效"})
			return
		}
		if !g.updateMu.TryLock() {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "更新已在下载"})
			return
		}
		defer g.updateMu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
		defer cancel()
		candidate, err := g.app.latestUpdate(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		if !candidate.Report.Available {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "当前已是最新版"})
			return
		}
		download, err := g.app.downloadLatestUpdate(ctx, candidate)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, download)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
