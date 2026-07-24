package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"faynoSync-cli/internal/config"
)

var errUploadHelp = errors.New("upload help requested")

type uploadFlags struct {
	AppName        string
	Files          []string
	Version        string
	Channel        string
	Platform       string
	Arch           string
	Updater        string
	Signature      string
	Publish        bool
	Critical       bool
	Intermediate   bool
	Changelog      string
	ChangelogFile  string
	ChangelogStdin bool
}

type uploadData struct {
	AppName      string `json:"app_name"`
	Version      string `json:"version"`
	Channel      string `json:"channel"`
	Publish      bool   `json:"publish"`
	Critical     bool   `json:"critical"`
	Intermediate bool   `json:"intermediate"`
	Platform     string `json:"platform"`
	Arch         string `json:"arch"`
	Updater      string `json:"updater,omitempty"`
	Signature    string `json:"signature,omitempty"`
	Changelog    string `json:"changelog"`
}

var validUpdaters = map[string]bool{
	"manual":           true,
	"velopack":         true,
	"squirrel_darwin":  true,
	"squirrel_windows": true,
	"electron-builder": true,
	"tauri":            true,
	"sparkle":          true,
}

func (a *App) runUpload(args []string) error {
	flags, err := parseUploadFlags(args)
	if err != nil {
		if errors.Is(err, errUploadHelp) {
			a.printUploadUsage()
			return nil
		}
		return err
	}

	if len(flags.Files) == 0 {
		return errors.New("at least one --file is required")
	}

	runtimeCfg, _, err := config.LoadRuntime()
	if err != nil {
		return err
	}

	endpoint := strings.TrimRight(runtimeCfg.Server, "/") + "/upload"
	changelog, err := a.resolveChangelog(flags)
	if err != nil {
		return err
	}

	payload := uploadData{
		AppName:      flags.AppName,
		Version:      flags.Version,
		Channel:      flags.Channel,
		Publish:      flags.Publish,
		Critical:     flags.Critical,
		Intermediate: flags.Intermediate,
		Platform:     flags.Platform,
		Arch:         flags.Arch,
		Updater:      flags.Updater,
		Signature:    flags.Signature,
		Changelog:    changelog,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	bodyReader, contentType := buildUploadBody(flags.Files, string(payloadJSON))
	req, err := http.NewRequest(http.MethodPost, endpoint, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+runtimeCfg.Token)
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		a.logger.WithFields(map[string]any{
			"status": resp.StatusCode,
			"body":   strings.TrimSpace(string(respBody)),
		}).Error("upload failed")
		return fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}

	a.logger.WithFields(map[string]any{
		"files":       len(flags.Files),
		"app":         flags.AppName,
		"version":     flags.Version,
		"uploaded_id": extractUploadedID(respBody),
	}).Info("Upload completed")

	return nil
}

func buildUploadBody(filePaths []string, dataField string) (io.Reader, string) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	contentType := writer.FormDataContentType()

	go func() {
		for _, path := range filePaths {
			if err := appendFilePart(writer, path); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}

		if err := writer.WriteField("data", dataField); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		if err := writer.Close(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		_ = pw.Close()
	}()

	return pr, contentType
}

func appendFilePart(writer *multipart.Writer, path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("file path cannot be empty")
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(cleanPath))
	if err != nil {
		return err
	}

	_, err = io.Copy(part, file)
	return err
}

type repeatedString []string

func (r *repeatedString) String() string { return strings.Join(*r, ",") }

func (r *repeatedString) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func parseUploadFlags(args []string) (uploadFlags, error) {
	if len(args) == 1 && args[0] == "help" {
		return uploadFlags{}, errUploadHelp
	}

	var out uploadFlags
	var files repeatedString

	fs := flag.NewFlagSet("upload", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&out.AppName, "app", "", "")
	fs.Var(&files, "file", "")
	fs.StringVar(&out.Version, "version", "", "")
	fs.StringVar(&out.Channel, "channel", "", "")
	fs.StringVar(&out.Platform, "platform", "", "")
	fs.StringVar(&out.Arch, "arch", "", "")
	fs.StringVar(&out.Updater, "updater", "", "")
	fs.StringVar(&out.Signature, "signature", "", "")
	fs.BoolVar(&out.Publish, "publish", false, "")
	fs.BoolVar(&out.Critical, "critical", false, "")
	fs.BoolVar(&out.Intermediate, "intermediate", false, "")
	fs.StringVar(&out.Changelog, "changelog", "", "")
	fs.StringVar(&out.ChangelogFile, "changelog-file", "", "")
	fs.BoolVar(&out.ChangelogStdin, "changelog-stdin", false, "")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return uploadFlags{}, errUploadHelp
		}
		return uploadFlags{}, err
	}
	if fs.NArg() > 0 {
		return uploadFlags{}, fmt.Errorf("unexpected argument: %s", fs.Arg(0))
	}

	out.Files = files

	if err := validateChangelogInputMode(out); err != nil {
		return uploadFlags{}, err
	}
	if err := validateUpdater(out.Updater); err != nil {
		return uploadFlags{}, err
	}

	return out, nil
}

func validateUpdater(updater string) error {
	if updater == "" {
		return nil
	}
	if !validUpdaters[updater] {
		return fmt.Errorf("invalid updater %q: must be one of manual, velopack, squirrel_darwin, squirrel_windows, electron-builder, tauri, sparkle", updater)
	}
	return nil
}

func validateChangelogInputMode(flags uploadFlags) error {
	used := 0
	if flags.Changelog != "" {
		used++
	}
	if strings.TrimSpace(flags.ChangelogFile) != "" {
		used++
	}
	if flags.ChangelogStdin {
		used++
	}

	if used > 1 {
		return errors.New("use only one changelog source: --changelog, --changelog-file, or --changelog-stdin")
	}

	return nil
}

func (a *App) resolveChangelog(flags uploadFlags) (string, error) {
	if err := validateChangelogInputMode(flags); err != nil {
		return "", err
	}

	switch {
	case strings.TrimSpace(flags.ChangelogFile) != "":
		raw, err := os.ReadFile(strings.TrimSpace(flags.ChangelogFile))
		if err != nil {
			return "", err
		}
		return normalizeChangelog(string(raw)), nil
	case flags.ChangelogStdin:
		raw, err := io.ReadAll(a.in)
		if err != nil {
			return "", err
		}
		return normalizeChangelog(string(raw)), nil
	default:
		return normalizeChangelog(flags.Changelog), nil
	}
}

func normalizeChangelog(in string) string {
	out := strings.TrimPrefix(in, "\ufeff")
	out = strings.ReplaceAll(out, "\r\n", "\n")
	return out
}

func extractUploadedID(respBody []byte) string {
	var resp struct {
		Uploaded string `json:"uploadResult.Uploaded"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return ""
	}

	return strings.TrimSpace(resp.Uploaded)
}

func (a *App) printUploadUsage() {
	_, _ = fmt.Fprintln(a.out, `faynosync upload

Usage:
  faynosync upload [flags]

Upload flags:
  --app <name>
  --file <path>          may be specified multiple times
  --version <value>
  --channel <value>
  --platform <value>
  --arch <value>
  --updater <value>       manual|velopack|squirrel_darwin|squirrel_windows|electron-builder|tauri|sparkle
  --signature <value>     Tauri signature (base64, e.g. --signature "$(cat myapp.app.tar.gz.sig)")
  --publish[=true|false]
  --critical[=true|false]
  --intermediate[=true|false]
  --changelog <text>
  --changelog-file <path>
  --changelog-stdin`)
}
