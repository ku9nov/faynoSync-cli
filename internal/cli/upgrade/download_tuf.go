package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"faynoSync-cli/internal/config"

	tufconfig "github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"
)

func downloadWithTUF(ctx context.Context, updateURL string) (string, error) {
	runtimeCfg, _, err := config.LoadPublicRuntime()
	if err != nil {
		return "", err
	}

	targetName, filename, baseURL, appName, err := parseTUFUpdateURL(updateURL, runtimeCfg.Owner)
	if err != nil {
		return "", err
	}

	metadataURL, remoteTargetsURL, err := deriveTUFURLs(baseURL, runtimeCfg.Owner, appName)
	if err != nil {
		return "", err
	}

	configPath, err := config.Path()
	if err != nil {
		return "", err
	}

	tmpDir := filepath.Join(filepath.Dir(configPath), "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}

	tufDir := filepath.Join(tmpDir, "tuf", runtimeCfg.Owner, appName)
	localMetadataDir := filepath.Join(tufDir, "metadata")
	localTargetsDir := filepath.Join(tufDir, "download")

	if err := os.MkdirAll(localMetadataDir, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(localTargetsDir, 0o755); err != nil {
		return "", err
	}

	if err := bootstrapRoot(ctx, localMetadataDir, metadataURL); err != nil {
		return "", err
	}

	rootBytes, err := os.ReadFile(filepath.Join(localMetadataDir, "root.json"))
	if err != nil {
		return "", fmt.Errorf("read trusted root metadata: %w", err)
	}

	updaterCfg, err := tufconfig.New(metadataURL, rootBytes)
	if err != nil {
		return "", fmt.Errorf("create tuf updater config: %w", err)
	}
	updaterCfg.LocalMetadataDir = localMetadataDir
	updaterCfg.LocalTargetsDir = localTargetsDir
	updaterCfg.RemoteTargetsURL = remoteTargetsURL
	updaterCfg.PrefixTargetsWithHash = false

	up, err := updater.New(updaterCfg)
	if err != nil {
		return "", fmt.Errorf("create tuf updater: %w", err)
	}

	if err := up.Refresh(); err != nil {
		return "", fmt.Errorf("refresh trusted metadata: %w", err)
	}

	targetInfo, err := up.GetTargetInfo(targetName)
	if err != nil {
		return "", fmt.Errorf("resolve target metadata %q: %w", targetName, err)
	}

	sourcePath, _, err := up.FindCachedTarget(targetInfo, "")
	if err != nil {
		return "", fmt.Errorf("find cached target %q: %w", targetName, err)
	}

	if sourcePath == "" {
		sourcePath, _, err = up.DownloadTarget(targetInfo, "", "")
		if err != nil {
			return "", fmt.Errorf("download target %q: %w", targetName, err)
		}
	}

	targetPath := filepath.Join(tmpDir, filename)
	if err := copyFileWithLimit(sourcePath, targetPath, maxDownloadSize); err != nil {
		return "", err
	}

	return targetPath, nil
}

func parseTUFUpdateURL(updateURL, owner string) (string, string, string, string, error) {
	parsed, err := url.Parse(updateURL)
	if err != nil {
		return "", "", "", "", fmt.Errorf("invalid update_url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", "", "", "", errors.New("update_url must contain scheme and host")
	}

	targetName := strings.Trim(parsed.Path, "/")
	if targetName == "" {
		return "", "", "", "", errors.New("update_url must include a target path")
	}

	filename, err := filenameFromURL(updateURL)
	if err != nil {
		return "", "", "", "", err
	}

	segments := strings.Split(targetName, "/")
	if len(segments) == 0 || strings.TrimSpace(segments[0]) == "" {
		return "", "", "", "", errors.New("update_url must include app segment")
	}

	firstSegment := strings.TrimSpace(segments[0])
	appName := firstSegment
	owner = strings.TrimSpace(owner)
	if owner != "" {
		suffix := "-" + owner
		trimmed := strings.TrimSuffix(firstSegment, suffix)
		if trimmed != "" && trimmed != firstSegment {
			appName = trimmed
		}
	}
	if appName == "" {
		appName = defaultAppName
	}

	baseURL := (&url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
	}).String()

	return targetName, filename, baseURL, appName, nil
}

func deriveTUFURLs(baseURL, owner, appName string) (string, string, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return "", "", errors.New("owner is empty")
	}
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return "", "", errors.New("app name is empty")
	}

	metadataURL, err := url.JoinPath(baseURL, "tuf_metadata", owner, appName)
	if err != nil {
		return "", "", fmt.Errorf("build metadata url: %w", err)
	}

	metadataURL = strings.TrimRight(metadataURL, "/") + "/"
	remoteTargetsURL := strings.TrimRight(baseURL, "/") + "/"

	return metadataURL, remoteTargetsURL, nil
}

func bootstrapRoot(ctx context.Context, localMetadataDir, metadataURL string) error {
	rootPath := filepath.Join(localMetadataDir, "root.json")

	_, err := os.Stat(rootPath)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("stat trusted root metadata: %w", err)
	}

	rootURL, err := url.JoinPath(metadataURL, "1.root.json")
	if err != nil {
		return fmt.Errorf("build initial root metadata url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rootURL, nil)
	if err != nil {
		return fmt.Errorf("create initial root metadata request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch initial root metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("fetch initial root metadata failed with status %d", resp.StatusCode)
	}

	rootBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return fmt.Errorf("read initial root metadata body: %w", err)
	}
	if len(rootBytes) == 0 {
		return errors.New("initial root metadata is empty")
	}

	if err := os.WriteFile(rootPath, rootBytes, 0o644); err != nil {
		return fmt.Errorf("write trusted root metadata: %w", err)
	}

	return nil
}

func copyFileWithLimit(sourcePath, targetPath string, limit int64) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open downloaded target: %w", err)
	}
	defer sourceFile.Close()

	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("prepare target directory: %w", err)
	}

	partFile, err := os.CreateTemp(targetDir, path.Base(targetPath)+".*.part")
	if err != nil {
		return fmt.Errorf("create temporary target file: %w", err)
	}

	partPath := partFile.Name()
	success := false
	defer func() {
		_ = partFile.Close()
		if !success {
			_ = os.Remove(partPath)
		}
	}()

	written, err := io.Copy(partFile, io.LimitReader(sourceFile, limit+1))
	if err != nil {
		return fmt.Errorf("copy downloaded target: %w", err)
	}
	if written > limit {
		return fmt.Errorf("downloaded artifact exceeds %d bytes limit", limit)
	}

	if err := partFile.Close(); err != nil {
		return fmt.Errorf("flush temporary target file: %w", err)
	}

	if err := os.Rename(partPath, targetPath); err != nil {
		return fmt.Errorf("move downloaded target to final path: %w", err)
	}

	success = true
	return nil
}
