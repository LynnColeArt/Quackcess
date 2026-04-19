package packaging

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPackagingWorkflowCoversLinuxAndMacos(t *testing.T) {
	workflow := testWorkflowPath(t)
	raw, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatalf("read workflow %q: %v", workflow, err)
	}

	rawText := string(raw)
	if !strings.Contains(rawText, "runs-on: ${{ matrix.os }}") {
		t.Fatal("expected matrix-based OS job execution in workflow")
	}
	if !strings.Contains(rawText, "- ubuntu-latest") {
		t.Fatal("expected workflow to include ubuntu-latest")
	}
	if !strings.Contains(rawText, "- macos-latest") {
		t.Fatal("expected workflow to include macos-latest")
	}
	if !strings.Contains(rawText, "go build ./cmd/quackcess") {
		t.Fatal("expected workflow to include release-build validation command for packaging checks")
	}
}

func TestReleasePackagingWorkflowBuildsArtifactsAndChecksums(t *testing.T) {
	releaseWorkflow := releaseWorkflowPath(t)
	raw, err := os.ReadFile(releaseWorkflow)
	if err != nil {
		t.Fatalf("read workflow %q: %v", releaseWorkflow, err)
	}

	rawText := string(raw)
	if !strings.Contains(rawText, "push:") {
		t.Fatal("expected release workflow to run on tags")
	}
	if !strings.Contains(rawText, "tags:") {
		t.Fatal("expected release workflow to declare tag trigger")
	}
	if !strings.Contains(rawText, "v*") {
		t.Fatal("expected release workflow to match version tags")
	}
	if !strings.Contains(rawText, "goos: darwin") || !strings.Contains(rawText, "goos: linux") {
		t.Fatal("expected release workflow matrix to include linux and darwin builds")
	}
	if !strings.Contains(rawText, "tar -czf") {
		t.Fatal("expected release workflow to create tar.gz archives")
	}
	if !strings.Contains(rawText, ".sha256") {
		t.Fatal("expected release workflow to emit sha256 artifact checksums")
	}
	if !strings.Contains(rawText, "sha256sum") && !strings.Contains(rawText, "shasum -a 256") {
		t.Fatal("expected release workflow to emit sha256 artifact checksums")
	}
	if !strings.Contains(rawText, "actions/upload-artifact@v4") || !strings.Contains(rawText, "softprops/action-gh-release@v2") {
		t.Fatal("expected release workflow to publish packaged artifacts and attach to release")
	}
}

func testWorkflowPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve workflow path from test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../.github/workflows/test.yml"))
}

func releaseWorkflowPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve release workflow path from test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../.github/workflows/release.yml"))
}
