package upgrade

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"faynoSync-cli/internal/config"

	"github.com/sirupsen/logrus"
)

func TestRunUpdateAvailable(t *testing.T) {
	var gotQuery map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = map[string]string{}
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				gotQuery[key] = values[0]
			}
		}
		if r.URL.Path != "/checkVersion" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"critical":false,"update_available":true,"update_url":"https://updates.example/faynosync-cli-1.0.0"}`)
	}))
	defer srv.Close()

	logger, logOutput := newTestLogger()
	withTestConfig(t, srv.URL, "admin")
	t.Setenv(config.EnvToken, "")

	err := Run(Input{
		Logger:  logger,
		Version: "0.9.0",
		Channel: "stable",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	assertQuery(t, gotQuery, "app_name", defaultAppName)
	assertQuery(t, gotQuery, "version", "0.9.0")
	assertQuery(t, gotQuery, "channel", "stable")
	assertQuery(t, gotQuery, "platform", runtime.GOOS)
	assertQuery(t, gotQuery, "arch", runtime.GOARCH)
	assertQuery(t, gotQuery, "owner", "admin")

	logText := logOutput.String()
	if !strings.Contains(logText, "update is available") {
		t.Fatalf("expected update log message, got: %s", logText)
	}
}

func TestRunNoUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"critical":false,"possible_rollback":false,"update_available":false,"update_url":"https://updates.example/faynosync-cli-1.0.0"}`)
	}))
	defer srv.Close()

	logger, logOutput := newTestLogger()
	withTestConfig(t, srv.URL, "admin")

	err := Run(Input{
		Logger:  logger,
		Version: "1.0.0",
		Channel: "stable",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(logOutput.String(), "current version is up-to-date") {
		t.Fatalf("expected up-to-date log, got: %s", logOutput.String())
	}
}

func TestRunPossibleRollback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"critical":false,"possible_rollback":true,"update_available":false,"update_url":"https://updates.example/faynosync-cli-1.0.0"}`)
	}))
	defer srv.Close()

	logger, logOutput := newTestLogger()
	withTestConfig(t, srv.URL, "admin")

	err := Run(Input{
		Logger:  logger,
		Version: "2.0.0",
		Channel: "stable",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(logOutput.String(), "possible rollback detected") {
		t.Fatalf("expected rollback warning log, got: %s", logOutput.String())
	}
}

func TestRunPossibleRollbackWithExtendedUpdateURLField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"critical":false,"possible_rollback":true,"update_available":false,"update_url_exe":"https://updates.example/faynosync-cli-1.0.0.exe"}`)
	}))
	defer srv.Close()

	logger, logOutput := newTestLogger()
	withTestConfig(t, srv.URL, "admin")
	t.Setenv(config.EnvURL, srv.URL)
	t.Setenv(config.EnvAccount, "admin")

	err := Run(Input{
		Logger:  logger,
		Version: "2.0.0",
		Channel: "stable",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	logText := logOutput.String()
	if !strings.Contains(logText, "possible rollback detected") {
		t.Fatalf("expected rollback warning log, got: %s", logText)
	}
	if !strings.Contains(logText, "faynosync-cli-1.0.0.exe") {
		t.Fatalf("expected extended update url to be logged, got: %s", logText)
	}
}

func TestRunMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"critical":`)
	}))
	defer srv.Close()

	logger, _ := newTestLogger()
	withTestConfig(t, srv.URL, "admin")

	err := Run(Input{
		Logger:  logger,
		Version: "1.0.0",
		Channel: "stable",
	})
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode checkVersion response") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunServerErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"boom"}`)
	}))
	defer srv.Close()

	logger, _ := newTestLogger()
	withTestConfig(t, srv.URL, "admin")

	err := Run(Input{
		Logger:  logger,
		Version: "1.0.0",
		Channel: "stable",
	})
	if err == nil {
		t.Fatal("expected status error")
	}
	if !strings.Contains(err.Error(), "checkVersion failed with status 500") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestLogger() (*logrus.Logger, *bytes.Buffer) {
	buf := bytes.NewBuffer(nil)
	logger := logrus.New()
	logger.SetOutput(buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
	logger.SetLevel(logrus.TraceLevel)
	return logger, buf
}

func withTestConfig(t *testing.T, serverURL, owner string) {
	t.Helper()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv(config.EnvURL, "")
	t.Setenv(config.EnvAccount, "")
	t.Setenv(config.EnvToken, "")

	cfgPath := filepath.Join(tempHome, ".faynosync", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	content := fmt.Sprintf("server: %s\nowner: %s\n", serverURL, owner)
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func assertQuery(t *testing.T, query map[string]string, key, want string) {
	t.Helper()

	if got := query[key]; got != want {
		t.Fatalf("unexpected query %s: want %q, got %q", key, want, got)
	}
}
