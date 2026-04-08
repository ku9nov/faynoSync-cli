package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"time"

	"faynoSync-cli/internal/config"

	"github.com/sirupsen/logrus"
)

const (
	defaultAppName  = "faynosync-cli"
	defaultVersion  = "dev"
	defaultChannel  = "stable"
	maxBodySize     = 1 << 20
	maxDownloadSize = 512 << 20
)

type Input struct {
	Logger  *logrus.Logger
	Version string
	Channel string
}

type checkVersionResponse struct {
	Critical         bool   `json:"critical"`
	PossibleRollback bool   `json:"possible_rollback"`
	UpdateAvailable  bool   `json:"update_available"`
	UpdateURL        string `json:"update_url"`
}

func Run(input Input) error {
	if input.Logger == nil {
		return fmt.Errorf("upgrade: logger is required")
	}

	version := strings.TrimSpace(input.Version)
	if version == "" {
		version = defaultVersion
	}

	channel := strings.TrimSpace(input.Channel)
	if channel == "" {
		channel = defaultChannel
	}

	runtimeCfg, _, err := config.LoadPublicRuntime()
	if err != nil {
		return err
	}

	platform := runtime.GOOS
	arch := runtime.GOARCH
	endpoint := strings.TrimRight(runtimeCfg.Server, "/") + "/checkVersion"
	query := url.Values{
		"app_name": {defaultAppName},
		"version":  {version},
		"channel":  {channel},
		"platform": {platform},
		"arch":     {arch},
		"owner":    {runtimeCfg.Owner},
	}
	requestURL := endpoint + "?" + query.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		input.Logger.WithFields(map[string]any{
			"url": requestURL,
		}).Error("version check request failed")
		return fmt.Errorf("checkVersion request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		input.Logger.WithFields(map[string]any{
			"status": resp.StatusCode,
			"body":   strings.TrimSpace(string(respBody)),
		}).Error("version check failed")
		return fmt.Errorf("checkVersion failed with status %d", resp.StatusCode)
	}

	result, err := decodeCheckVersionResponse(respBody)
	if err != nil {
		input.Logger.WithFields(map[string]any{
			"body": strings.TrimSpace(string(respBody)),
		}).Error("failed to decode version check response")
		return fmt.Errorf("decode checkVersion response: %w", err)
	}

	fields := logrus.Fields{
		"app_name":          defaultAppName,
		"version":           version,
		"channel":           channel,
		"platform":          platform,
		"arch":              arch,
		"owner":             runtimeCfg.Owner,
		"update_available":  result.UpdateAvailable,
		"possible_rollback": result.PossibleRollback,
		"critical":          result.Critical,
		"update_url":        result.UpdateURL,
	}

	switch {
	case result.UpdateAvailable:
		input.Logger.WithFields(fields).Info("update is available")
	case result.PossibleRollback:
		input.Logger.WithFields(fields).Warn("possible rollback detected")
		return nil
	default:
		input.Logger.WithFields(fields).Info("current version is up-to-date")
		return nil
	}

	if strings.TrimSpace(result.UpdateURL) == "" {
		return errors.New("update is available but update_url is empty")
	}

	downloadCtx, downloadCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer downloadCancel()

	var downloadPath string
	if runtimeCfg.TUF {
		downloadPath, err = downloadWithTUF(downloadCtx, result.UpdateURL)
	} else {
		downloadPath, err = downloadDirect(downloadCtx, result.UpdateURL)
	}
	if err != nil {
		return err
	}

	input.Logger.WithFields(logrus.Fields{
		"update_url": result.UpdateURL,
		"path":       downloadPath,
		"tuf":        runtimeCfg.TUF,
	}).Info("update artifact downloaded")

	return nil
}

func decodeCheckVersionResponse(body []byte) (checkVersionResponse, error) {
	var result checkVersionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return checkVersionResponse{}, err
	}

	if strings.TrimSpace(result.UpdateURL) != "" {
		return result, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return checkVersionResponse{}, err
	}

	result.UpdateURL = pickExtendedUpdateURL(raw)
	return result, nil
}

func pickExtendedUpdateURL(raw map[string]json.RawMessage) string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		if strings.HasPrefix(key, "update_url_") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		var value string
		if err := json.Unmarshal(raw[key], &value); err != nil {
			continue
		}
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}

	return ""
}
