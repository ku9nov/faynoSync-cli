package upgrade

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"faynoSync-cli/internal/config"

	faynosync "github.com/ku9nov/faynosync-sdk-go"
	"github.com/sirupsen/logrus"
)

const (
	defaultAppName  = "faynosync-cli"
	defaultVersion  = "dev"
	defaultChannel  = "stable"
	maxBodySize     = 1 << 20
	maxDownloadSize = 128 << 20
)

type Input struct {
	Logger  *logrus.Logger
	Version string
	Channel string
	In      io.Reader
	Out     io.Writer
}

var installDownloadedArtifactFn = installDownloadedArtifact

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

	deviceID, err := config.EnsureDeviceID()
	if err != nil {
		return fmt.Errorf("ensure device id: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := faynosync.NewClient(faynosync.Config{BaseURL: runtimeCfg.Server})
	result, err := client.CheckForUpdates(ctx, faynosync.CheckOptions{
		Owner:    runtimeCfg.Owner,
		AppName:  defaultAppName,
		Version:  version,
		Channel:  channel,
		Platform: platform,
		Arch:     arch,
		DeviceID: deviceID,
	})
	if err != nil {
		input.Logger.WithFields(map[string]any{
			"owner":    runtimeCfg.Owner,
			"app_name": defaultAppName,
		}).Error("version check failed")
		return fmt.Errorf("checkVersion request failed: %w", err)
	}

	updateURL := resolveUpdateURL(result)

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
		"update_url":        updateURL,
	}

	switch {
	case result.UpdateAvailable:
		input.Logger.WithFields(fields).Info("update is available")
	case result.PossibleRollback:
		input.Logger.WithFields(fields).Warn("possible rollback detected")
		rollbackAllowed, err := promptRollbackConfirmation(input.In, input.Out, updateURL)
		if err != nil {
			return err
		}
		if !rollbackAllowed {
			input.Logger.WithFields(fields).Info("rollback cancelled by user")
			return nil
		}
		input.Logger.WithFields(fields).Info("rollback confirmed by user")
	default:
		input.Logger.WithFields(fields).Info("current version is up-to-date")
		return nil
	}

	if strings.TrimSpace(updateURL) == "" {
		return errors.New("update is available but update_url is empty")
	}

	downloadCtx, downloadCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer downloadCancel()

	var downloadPath string
	if runtimeCfg.TUF {
		downloadPath, err = downloadWithTUF(downloadCtx, updateURL)
	} else {
		downloadPath, err = downloadDirect(downloadCtx, updateURL)
	}
	if err != nil {
		return err
	}

	input.Logger.WithFields(logrus.Fields{
		"update_url": updateURL,
		"path":       downloadPath,
		"tuf":        runtimeCfg.TUF,
	}).Info("update artifact downloaded")

	if err := installDownloadedArtifactFn(downloadPath); err != nil {
		return fmt.Errorf("install downloaded artifact: %w", err)
	}

	input.Logger.WithFields(logrus.Fields{
		"path": downloadPath,
	}).Info("cli binary updated")

	return nil
}

func resolveUpdateURL(resp *faynosync.UpdateResponse) string {
	if u := strings.TrimSpace(resp.UpdateURL); u != "" {
		return u
	}

	// PackageURLs are sorted by package name; pick the first non-empty one.
	for _, pkg := range resp.PackageURLs {
		if u := strings.TrimSpace(pkg.URL); u != "" {
			return u
		}
	}

	return ""
}

func promptRollbackConfirmation(in io.Reader, out io.Writer, updateURL string) (bool, error) {
	if in == nil || out == nil {
		return false, nil
	}

	_, _ = fmt.Fprintf(out, "Possible rollback detected to %s. Continue with rollback? (y/N): ", strings.TrimSpace(updateURL))

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read rollback confirmation: %w", err)
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	switch answer {
	case "y", "yes":
		return true, nil
	case "", "n", "no":
		return false, nil
	default:
		_, _ = fmt.Fprintln(out, "Invalid response. Rollback cancelled.")
		return false, nil
	}
}

func installDownloadedArtifact(downloadPath string) error {
	executablePath, err := resolveCurrentExecutablePath()
	if err != nil {
		return err
	}

	return replaceBinaryWithMode(downloadPath, executablePath)
}

func resolveCurrentExecutablePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(executablePath)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlink: %w", err)
	}

	return resolvedPath, nil
}

func replaceBinaryWithMode(downloadPath, executablePath string) error {
	executableInfo, err := os.Stat(executablePath)
	if err != nil {
		return fmt.Errorf("stat current executable: %w", err)
	}

	// Preserve permission bits from the currently installed binary.
	if err := os.Chmod(downloadPath, executableInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("apply executable mode to downloaded artifact: %w", err)
	}

	if err := os.Rename(downloadPath, executablePath); err != nil {
		return fmt.Errorf("atomically replace executable: %w", err)
	}

	return nil
}
