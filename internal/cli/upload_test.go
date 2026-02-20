package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParseUploadFlagsSupportsChangelogFile(t *testing.T) {
	got, err := parseUploadFlags([]string{
		"--file", "./artifact.bin",
		"--changelog-file", "./CHANGELOG.md",
	})
	if err != nil {
		t.Fatalf("parseUploadFlags returned error: %v", err)
	}

	if got.ChangelogFile != "./CHANGELOG.md" {
		t.Fatalf("unexpected changelog file: %q", got.ChangelogFile)
	}
	if got.ChangelogStdin {
		t.Fatal("expected changelog stdin to be false")
	}
}

func TestParseUploadFlagsSupportsChangelogStdin(t *testing.T) {
	got, err := parseUploadFlags([]string{
		"--file", "./artifact.bin",
		"--changelog-stdin",
	})
	if err != nil {
		t.Fatalf("parseUploadFlags returned error: %v", err)
	}

	if !got.ChangelogStdin {
		t.Fatal("expected changelog stdin to be true")
	}
}

func TestParseUploadFlagsRejectsMultipleChangelogSources(t *testing.T) {
	_, err := parseUploadFlags([]string{
		"--file", "./artifact.bin",
		"--changelog", "inline text",
		"--changelog-file", "./CHANGELOG.md",
	})
	if err == nil {
		t.Fatal("expected exclusivity validation error")
	}
}

func TestResolveChangelogFromFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "CHANGELOG.md")
	content := "\ufeff## Changelog\r\n- Added feature X\r\n- Fixed bug Y\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write changelog file: %v", err)
	}

	app := New(bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	got, err := app.resolveChangelog(uploadFlags{ChangelogFile: path})
	if err != nil {
		t.Fatalf("resolveChangelog returned error: %v", err)
	}

	want := "## Changelog\n- Added feature X\n- Fixed bug Y\n"
	if got != want {
		t.Fatalf("unexpected changelog content:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestResolveChangelogFromStdinPreservesSpecialCharacters(t *testing.T) {
	content := "### Changelog\n- Added + escaped chars: -]!-%^:;\"{<+\"\\&££,!#>${$]>|:=?£:^[(`<):.&.(@{:\"@=>\"\n"
	app := New(bytes.NewBufferString(content), bytes.NewBuffer(nil))

	got, err := app.resolveChangelog(uploadFlags{ChangelogStdin: true})
	if err != nil {
		t.Fatalf("resolveChangelog returned error: %v", err)
	}

	if got != content {
		t.Fatalf("changelog content changed unexpectedly:\nwant: %q\ngot:  %q", content, got)
	}
}

func TestExtractUploadedIDFromFlatResponse(t *testing.T) {
	resp := []byte(`{"uploadResult.Uploaded":"6998434c38ab5a799af0afbd"}`)
	got := extractUploadedID(resp)
	want := "6998434c38ab5a799af0afbd"

	if got != want {
		t.Fatalf("unexpected uploaded id:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestExtractUploadedIDFromNestedResponse(t *testing.T) {
	resp := []byte(`{"uploadResult":{"Uploaded":"6998434c38ab5a799af0afbd"}}`)
	got := extractUploadedID(resp)
	want := "6998434c38ab5a799af0afbd"

	if got != want {
		t.Fatalf("unexpected uploaded id:\nwant: %q\ngot:  %q", want, got)
	}
}
