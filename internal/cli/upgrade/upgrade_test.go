package upgrade

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"faynoSync-cli/internal/config"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sirupsen/logrus"
	tufmetadata "github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestRunUpdateAvailable(t *testing.T) {
	originalInstall := installDownloadedArtifactFn
	installDownloadedArtifactFn = func(_ string) error { return nil }
	t.Cleanup(func() {
		installDownloadedArtifactFn = originalInstall
	})

	var gotQuery map[string]string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checkVersion":
			gotQuery = map[string]string{}
			for key, values := range r.URL.Query() {
				if len(values) > 0 {
					gotQuery[key] = values[0]
				}
			}
			_, _ = fmt.Fprintf(w, `{"critical":false,"update_available":true,"update_url":"%s/files/faynosync-cli-1.0.0"}`, srv.URL)
		case "/files/faynosync-cli-1.0.0":
			_, _ = fmt.Fprint(w, "binary-content")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
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
	if !strings.Contains(logText, "update artifact downloaded") {
		t.Fatalf("expected download log message, got: %s", logText)
	}

	tmpFile := filepath.Join(testConfigDir(t), "tmp", "faynosync-cli-1.0.0")
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("expected downloaded file at %s: %v", tmpFile, err)
	}
	if string(content) != "binary-content" {
		t.Fatalf("unexpected downloaded content: %q", string(content))
	}
}

func TestRunUpdateAvailableWithTUFEnabled(t *testing.T) {
	originalInstall := installDownloadedArtifactFn
	installDownloadedArtifactFn = func(_ string) error { return nil }
	t.Cleanup(func() {
		installDownloadedArtifactFn = originalInstall
	})

	targetName := "faynosync-cli-admin/stable/darwin/arm64/faynosync-cli-1.0.0"
	targetContent := []byte("signed-binary-content")

	repoRoot := t.TempDir()
	createTUFRepoFixture(t, repoRoot, "admin", "faynosync-cli", targetName, targetContent)

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/checkVersion", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"critical":false,"update_available":true,"update_url":"%s/%s"}`, srv.URL, targetName)
	})
	mux.Handle("/", http.FileServer(http.Dir(repoRoot)))
	srv = httptest.NewServer(mux)
	defer srv.Close()

	logger, _ := newTestLogger()
	withTestConfigTUF(t, srv.URL, "admin", true)
	t.Setenv(config.EnvToken, "")

	err := Run(Input{
		Logger:  logger,
		Version: "0.9.0",
		Channel: "stable",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	tmpFile := filepath.Join(testConfigDir(t), "tmp", "faynosync-cli-1.0.0")
	content, readErr := os.ReadFile(tmpFile)
	if readErr != nil {
		t.Fatalf("expected downloaded file at %s: %v", tmpFile, readErr)
	}
	if string(content) != string(targetContent) {
		t.Fatalf("unexpected downloaded content: %q", string(content))
	}
}

func TestParseTUFUpdateURLRejectsURLWithoutHost(t *testing.T) {
	_, _, _, _, err := parseTUFUpdateURL("faynosync-cli-admin/stable/darwin/arm64/faynosync-cli-1.0.0", "admin")
	if err == nil {
		t.Fatal("expected parse error for update_url without host")
	}
	if !strings.Contains(err.Error(), "scheme and host") {
		t.Fatalf("unexpected error: %v", err)
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

func TestRunInstallError(t *testing.T) {
	originalInstall := installDownloadedArtifactFn
	installDownloadedArtifactFn = func(_ string) error {
		return errors.New("replace failed")
	}
	t.Cleanup(func() {
		installDownloadedArtifactFn = originalInstall
	})

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checkVersion":
			_, _ = fmt.Fprintf(w, `{"critical":false,"update_available":true,"update_url":"%s/files/faynosync-cli-1.0.1"}`, srv.URL)
		case "/files/faynosync-cli-1.0.1":
			_, _ = fmt.Fprint(w, "binary-content")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	logger, _ := newTestLogger()
	withTestConfig(t, srv.URL, "admin")
	t.Setenv(config.EnvToken, "")

	err := Run(Input{
		Logger:  logger,
		Version: "1.0.0",
		Channel: "stable",
	})
	if err == nil {
		t.Fatal("expected install error")
	}
	if !strings.Contains(err.Error(), "install downloaded artifact") {
		t.Fatalf("unexpected error wrapper: %v", err)
	}
	if !strings.Contains(err.Error(), "replace failed") {
		t.Fatalf("unexpected install error detail: %v", err)
	}
}

func TestReplaceBinaryWithMode(t *testing.T) {
	tmpDir := t.TempDir()
	currentPath := filepath.Join(tmpDir, "faynosync-cli")
	downloadPath := filepath.Join(tmpDir, "faynosync-cli.new")

	if err := os.WriteFile(currentPath, []byte("old-binary"), 0o600); err != nil {
		t.Fatalf("write current executable: %v", err)
	}
	if err := os.Chmod(currentPath, 0o751); err != nil {
		t.Fatalf("chmod current executable: %v", err)
	}
	if err := os.WriteFile(downloadPath, []byte("new-binary"), 0o644); err != nil {
		t.Fatalf("write downloaded artifact: %v", err)
	}

	if err := replaceBinaryWithMode(downloadPath, currentPath); err != nil {
		t.Fatalf("replaceBinaryWithMode returned error: %v", err)
	}

	content, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("read replaced executable: %v", err)
	}
	if string(content) != "new-binary" {
		t.Fatalf("unexpected executable content: %q", string(content))
	}

	info, err := os.Stat(currentPath)
	if err != nil {
		t.Fatalf("stat replaced executable: %v", err)
	}
	if info.Mode().Perm() != 0o751 {
		t.Fatalf("unexpected executable mode: got %o, want %o", info.Mode().Perm(), os.FileMode(0o751))
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
	withTestConfigTUF(t, serverURL, owner, false)
}

func withTestConfigTUF(t *testing.T, serverURL, owner string, tuf bool) {
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
	content := fmt.Sprintf("server: %s\nowner: %s\ntuf: %t\n", serverURL, owner, tuf)
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func testConfigDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.Getenv("HOME"), ".faynosync")
}

func assertQuery(t *testing.T, query map[string]string, key, want string) {
	t.Helper()

	if got := query[key]; got != want {
		t.Fatalf("unexpected query %s: want %q, got %q", key, want, got)
	}
}

func createTUFRepoFixture(t *testing.T, rootDir, owner, appName, targetName string, targetContent []byte) {
	t.Helper()

	targetFilePath := filepath.Join(rootDir, filepath.FromSlash(targetName))
	if err := os.MkdirAll(filepath.Dir(targetFilePath), 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	if err := os.WriteFile(targetFilePath, targetContent, 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	metadataDir := filepath.Join(rootDir, "tuf_metadata", owner, appName)
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("create metadata directory: %v", err)
	}

	expires := time.Now().Add(24 * time.Hour).UTC()
	root := tufmetadata.Root(expires)
	root.Signed.ConsistentSnapshot = false
	targets := tufmetadata.Targets(expires)
	snapshot := tufmetadata.Snapshot(expires)
	timestamp := tufmetadata.Timestamp(expires)

	keys := map[string]ed25519.PrivateKey{}
	for _, role := range []string{"root", "targets", "snapshot", "timestamp"} {
		_, privateKey, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("generate %s key: %v", role, err)
		}
		keys[role] = privateKey

		publicKey, err := tufmetadata.KeyFromPublicKey(privateKey.Public())
		if err != nil {
			t.Fatalf("convert %s public key: %v", role, err)
		}
		if err := root.Signed.AddKey(publicKey, role); err != nil {
			t.Fatalf("add %s key to root: %v", role, err)
		}
	}

	targetInfo, err := tufmetadata.TargetFile().FromFile(targetFilePath, "sha256")
	if err != nil {
		t.Fatalf("build target file metadata: %v", err)
	}
	targets.Signed.Targets[targetName] = targetInfo

	snapshot.Signed.Meta["targets.json"] = tufmetadata.MetaFile(targets.Signed.Version)
	timestamp.Signed.Meta["snapshot.json"] = tufmetadata.MetaFile(snapshot.Signed.Version)

	signRole := func(name string, signed any) {
		signer, err := signature.LoadSigner(keys[name], crypto.Hash(0))
		if err != nil {
			t.Fatalf("load %s signer: %v", name, err)
		}

		switch md := signed.(type) {
		case *tufmetadata.Metadata[tufmetadata.RootType]:
			if _, err := md.Sign(signer); err != nil {
				t.Fatalf("sign %s metadata: %v", name, err)
			}
		case *tufmetadata.Metadata[tufmetadata.TargetsType]:
			if _, err := md.Sign(signer); err != nil {
				t.Fatalf("sign %s metadata: %v", name, err)
			}
		case *tufmetadata.Metadata[tufmetadata.SnapshotType]:
			if _, err := md.Sign(signer); err != nil {
				t.Fatalf("sign %s metadata: %v", name, err)
			}
		case *tufmetadata.Metadata[tufmetadata.TimestampType]:
			if _, err := md.Sign(signer); err != nil {
				t.Fatalf("sign %s metadata: %v", name, err)
			}
		default:
			t.Fatalf("unsupported metadata type for role %s", name)
		}
	}

	signRole("root", root)
	signRole("targets", targets)
	signRole("snapshot", snapshot)
	signRole("timestamp", timestamp)

	if err := root.ToFile(filepath.Join(metadataDir, "1.root.json"), true); err != nil {
		t.Fatalf("write root metadata: %v", err)
	}
	if err := targets.ToFile(filepath.Join(metadataDir, "targets.json"), true); err != nil {
		t.Fatalf("write targets metadata: %v", err)
	}
	if err := snapshot.ToFile(filepath.Join(metadataDir, "snapshot.json"), true); err != nil {
		t.Fatalf("write snapshot metadata: %v", err)
	}
	if err := timestamp.ToFile(filepath.Join(metadataDir, "timestamp.json"), true); err != nil {
		t.Fatalf("write timestamp metadata: %v", err)
	}
}
