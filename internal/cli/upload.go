package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	Changelog    string `json:"changelog"`
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
		return nil
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

func parseUploadFlags(args []string) (uploadFlags, error) {
	var out uploadFlags
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "-h" || arg == "--help" || arg == "help":
			return uploadFlags{}, errUploadHelp
		case arg == "--app":
			val, consumed, err := requireValue(args, i, "--app")
			if err != nil {
				return uploadFlags{}, err
			}
			out.AppName = val
			i += consumed
		case strings.HasPrefix(arg, "--app="):
			out.AppName = strings.TrimPrefix(arg, "--app=")
		case arg == "--file":
			val, consumed, err := requireValue(args, i, "--file")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Files = append(out.Files, val)
			i += consumed
		case strings.HasPrefix(arg, "--file="):
			out.Files = append(out.Files, strings.TrimPrefix(arg, "--file="))
		case arg == "--version":
			val, consumed, err := requireValue(args, i, "--version")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Version = val
			i += consumed
		case strings.HasPrefix(arg, "--version="):
			out.Version = strings.TrimPrefix(arg, "--version=")
		case arg == "--channel":
			val, consumed, err := requireValue(args, i, "--channel")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Channel = val
			i += consumed
		case strings.HasPrefix(arg, "--channel="):
			out.Channel = strings.TrimPrefix(arg, "--channel=")
		case arg == "--platform":
			val, consumed, err := requireValue(args, i, "--platform")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Platform = val
			i += consumed
		case strings.HasPrefix(arg, "--platform="):
			out.Platform = strings.TrimPrefix(arg, "--platform=")
		case arg == "--arch":
			val, consumed, err := requireValue(args, i, "--arch")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Arch = val
			i += consumed
		case strings.HasPrefix(arg, "--arch="):
			out.Arch = strings.TrimPrefix(arg, "--arch=")
		case arg == "--publish":
			val, consumed, err := parseBoolValue(args, i, "--publish")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Publish = val
			i += consumed
		case strings.HasPrefix(arg, "--publish="):
			val, err := parseBool(strings.TrimPrefix(arg, "--publish="), "--publish")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Publish = val
		case arg == "--critical":
			val, consumed, err := parseBoolValue(args, i, "--critical")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Critical = val
			i += consumed
		case strings.HasPrefix(arg, "--critical="):
			val, err := parseBool(strings.TrimPrefix(arg, "--critical="), "--critical")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Critical = val
		case arg == "--intermediate":
			val, consumed, err := parseBoolValue(args, i, "--intermediate")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Intermediate = val
			i += consumed
		case strings.HasPrefix(arg, "--intermediate="):
			val, err := parseBool(strings.TrimPrefix(arg, "--intermediate="), "--intermediate")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Intermediate = val
		case arg == "--changelog":
			val, consumed, err := requireValue(args, i, "--changelog")
			if err != nil {
				return uploadFlags{}, err
			}
			out.Changelog = val
			i += consumed
		case strings.HasPrefix(arg, "--changelog="):
			out.Changelog = strings.TrimPrefix(arg, "--changelog=")
		case arg == "--changelog-file":
			val, consumed, err := requireValue(args, i, "--changelog-file")
			if err != nil {
				return uploadFlags{}, err
			}
			out.ChangelogFile = val
			i += consumed
		case strings.HasPrefix(arg, "--changelog-file="):
			out.ChangelogFile = strings.TrimPrefix(arg, "--changelog-file=")
		case arg == "--changelog-stdin":
			val, consumed, err := parseBoolValue(args, i, "--changelog-stdin")
			if err != nil {
				return uploadFlags{}, err
			}
			out.ChangelogStdin = val
			i += consumed
		case strings.HasPrefix(arg, "--changelog-stdin="):
			val, err := parseBool(strings.TrimPrefix(arg, "--changelog-stdin="), "--changelog-stdin")
			if err != nil {
				return uploadFlags{}, err
			}
			out.ChangelogStdin = val
		default:
			return uploadFlags{}, fmt.Errorf("unknown upload flag: %s", arg)
		}
	}

	if err := validateChangelogInputMode(out); err != nil {
		return uploadFlags{}, err
	}

	return out, nil
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
	var flat map[string]any
	if err := json.Unmarshal(respBody, &flat); err != nil {
		return ""
	}

	if id, ok := flat["uploadResult.Uploaded"].(string); ok {
		return strings.TrimSpace(id)
	}

	if nested, ok := flat["uploadResult"].(map[string]any); ok {
		if id, ok := nested["Uploaded"].(string); ok {
			return strings.TrimSpace(id)
		}
		if id, ok := nested["uploaded"].(string); ok {
			return strings.TrimSpace(id)
		}
	}

	if id, ok := flat["uploaded_id"].(string); ok {
		return strings.TrimSpace(id)
	}

	return ""
}

func requireValue(args []string, idx int, name string) (string, int, error) {
	if idx+1 >= len(args) {
		return "", 0, fmt.Errorf("missing value for %s", name)
	}
	next := strings.TrimSpace(args[idx+1])
	if strings.HasPrefix(next, "-") {
		return "", 0, fmt.Errorf("missing value for %s", name)
	}
	return next, 1, nil
}

func parseBoolValue(args []string, idx int, name string) (bool, int, error) {
	if idx+1 < len(args) && !strings.HasPrefix(strings.TrimSpace(args[idx+1]), "-") {
		val, err := parseBool(strings.TrimSpace(args[idx+1]), name)
		if err != nil {
			return false, 0, err
		}
		return val, 1, nil
	}

	return true, 0, nil
}

func parseBool(value, name string) (bool, error) {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("invalid boolean value for %s: %q", name, value)
	}
	return parsed, nil
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
  --publish[=true|false]
  --critical[=true|false]
  --intermediate[=true|false]
  --changelog <text>
  --changelog-file <path>
  --changelog-stdin`)
}
