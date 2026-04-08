package upgrade

import (
	"context"
	"errors"
	"faynoSync-cli/internal/config"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func downloadWithTUF(_ context.Context, _ string) (string, error) {
	return "", errors.New("tuf download is not implemented yet")
}

func downloadDirect(ctx context.Context, updateURL string) (string, error) {
	filename, err := filenameFromURL(updateURL)
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

	targetPath := filepath.Join(tmpDir, filename)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	partFile, err := os.CreateTemp(tmpDir, filename+".*.part")
	if err != nil {
		return "", err
	}

	partPath := partFile.Name()
	success := false
	defer func() {
		_ = partFile.Close()
		if !success {
			_ = os.Remove(partPath)
		}
	}()

	written, err := io.Copy(partFile, io.LimitReader(resp.Body, maxDownloadSize+1))
	if err != nil {
		return "", err
	}
	if written > maxDownloadSize {
		return "", fmt.Errorf("downloaded artifact exceeds %d bytes limit", maxDownloadSize)
	}

	if err := partFile.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(partPath, targetPath); err != nil {
		return "", err
	}

	success = true
	return targetPath, nil
}

func filenameFromURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid update_url: %w", err)
	}

	filename := strings.TrimSpace(path.Base(parsed.Path))
	if filename == "" || filename == "." || filename == "/" {
		return "", errors.New("update_url must include a file name")
	}

	return filename, nil
}
