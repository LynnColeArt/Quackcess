package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
	"github.com/LynnColeArt/Quackcess/internal/project"
	"github.com/LynnColeArt/Quackcess/internal/ui/gtk"
)

func TestInitRequiresOutputPath(t *testing.T) {
	if err := run([]string{"init"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	if err := run([]string{"bad-command"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunMCPRequiresProjectPath(t *testing.T) {
	if err := run([]string{"mcp"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunMCPInvokesServerRunner(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "mcp.qdb")
	if err := run([]string{"init", "--name", "MCP", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	var called bool
	var gotPath string
	var gotPrincipal string
	var gotPermissionMatrix string
	oldRunner := runMCPServerFn
	runMCPServerFn = func(path string, principal string, permissionMatrixPath string) error {
		called = true
		gotPath = path
		gotPrincipal = principal
		gotPermissionMatrix = permissionMatrixPath
		return nil
	}
	defer func() {
		runMCPServerFn = oldRunner
	}()

	if err := run([]string{"mcp", "--principal", "agent", projectPath}); err != nil {
		t.Fatalf("run mcp: %v", err)
	}
	if !called {
		t.Fatal("expected mcp server runner to be called")
	}
	if gotPath != projectPath {
		t.Fatalf("project path = %q, want %q", gotPath, projectPath)
	}
	if gotPrincipal != "agent" {
		t.Fatalf("principal = %q, want agent", gotPrincipal)
	}
	if gotPermissionMatrix != "" {
		t.Fatalf("permission-matrix = %q, want empty", gotPermissionMatrix)
	}
}

func TestRunMCPPassesPermissionMatrixPathThrough(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))
	matrixPath := filepath.Join(tmp, "permissions.json")
	if err := os.WriteFile(matrixPath, []byte(`{"defaultAllow":false,"principals":{"agent":["system.ping"]}}`), 0o644); err != nil {
		t.Fatalf("write matrix file: %v", err)
	}

	projectPath := filepath.Join(tmp, "mcp-matrix.qdb")
	if err := run([]string{"init", "--name", "MCP", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	var gotPermissionMatrix string
	oldRunner := runMCPServerFn
	runMCPServerFn = func(path string, principal string, permissionMatrixPath string) error {
		gotPermissionMatrix = permissionMatrixPath
		return nil
	}
	defer func() {
		runMCPServerFn = oldRunner
	}()

	if err := run([]string{"mcp", "--principal", "agent", "--permission-matrix", matrixPath, projectPath}); err != nil {
		t.Fatalf("run mcp: %v", err)
	}
	if gotPermissionMatrix != matrixPath {
		t.Fatalf("permission matrix path = %q, want %q", gotPermissionMatrix, matrixPath)
	}
}

func TestRunMCPRejectsInvalidPermissionMatrixPath(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "mcp-missing-matrix.qdb")
	if err := run([]string{"init", "--name", "MCP", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	missingPath := filepath.Join(tmp, "does-not-exist.json")
	if err := run([]string{"mcp", "--principal", "agent", "--permission-matrix", missingPath, projectPath}); err == nil {
		t.Fatal("expected error")
	}
}

func TestInitRejectsMalformedFlags(t *testing.T) {
	if err := run([]string{"init", "--does-not-exist"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestInitRejectsExtraArguments(t *testing.T) {
	if err := run([]string{"init", "one", "two"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestInitRunsVectorSetupByDefault(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "vector-init.qdb")

	calls := 0
	oldInstaller := runInstallFn
	runInstallFn = func(args []string) error {
		calls++
		if len(args) != 0 {
			t.Fatalf("install args = %#v, want empty", args)
		}
		return nil
	}
	defer func() {
		runInstallFn = oldInstaller
	}()

	if err := run([]string{"init", "--name", "AutoVector", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected vector setup to run, got %d calls", calls)
	}
}

func TestInitSkipVectorSetupFlag(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "vector-init-skip.qdb")

	calls := 0
	oldInstaller := runInstallFn
	runInstallFn = func(args []string) error {
		calls++
		return nil
	}
	defer func() {
		runInstallFn = oldInstaller
	}()

	if err := run([]string{"init", "--skip-vector-setup", "--name", "AutoVector", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected vector setup to be skipped, got %d calls", calls)
	}
}

func TestOpenRejectsMissingPath(t *testing.T) {
	if err := run([]string{"open"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenProjectRunsDBBootstrap(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "boot.qdb")
	if err := run([]string{"init", "--name", "Boot", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := run([]string{"open", "--no-ui", projectPath}); err != nil {
		t.Fatalf("open: %v", err)
	}
}

func TestInitWithoutSeedDBCanOpenProject(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "boot-default-db.qdb")

	if err := run([]string{"init", "--skip-vector-setup", "--name", "BootNoSeed", projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := run([]string{"open", "--no-ui", projectPath}); err != nil {
		t.Fatalf("open: %v", err)
	}
}

func TestOpenDefaultsToUiModeInvokesShellWindow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "default-ui.qdb")
	if err := run([]string{"init", "--name", "DefaultUI", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	called := false
	oldRunner := runShellWindowFn
	runShellWindowFn = func(*db.DB, *project.Project) error {
		called = true
		return nil
	}
	defer func() {
		runShellWindowFn = oldRunner
	}()

	output, err := captureStdout(func() error {
		return run([]string{"open", projectPath})
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if strings.TrimSpace(output) != "open mode: ui" {
		t.Fatalf("unexpected open output: %q", output)
	}
	if !called {
		t.Fatal("expected shell window runner to be invoked")
	}
}

func TestOpenUiUnavailableFallsBackToHeadless(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "ui-unavailable.qdb")
	if err := run([]string{"init", "--name", "Unavailable", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	called := false
	oldRunner := runShellWindowFn
	runShellWindowFn = func(*db.DB, *project.Project) error {
		called = true
		return gtk.ErrGTKUnavailable
	}
	defer func() {
		runShellWindowFn = oldRunner
	}()

	output, err := captureStdout(func() error {
		return run([]string{"open", projectPath})
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !called {
		t.Fatal("expected shell window runner to be invoked")
	}

	got := strings.TrimSpace(output)
	if got != "open mode: ui\nopen mode: headless (ui unavailable)" {
		t.Fatalf("unexpected open output: %q", output)
	}
}

func TestOpenHeadlessModeSkipsShellWindow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "headless.qdb")
	if err := run([]string{"init", "--name", "Headless", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	called := false
	oldRunner := runShellWindowFn
	runShellWindowFn = func(*db.DB, *project.Project) error {
		called = true
		return nil
	}
	defer func() {
		runShellWindowFn = oldRunner
	}()

	if err := run([]string{"open", "--no-ui", projectPath}); err != nil {
		t.Fatalf("open: %v", err)
	}
	if called {
		t.Fatal("expected shell window runner to be skipped in headless mode")
	}
}

func TestOpenProjectCanSkipUIShell(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "headless.qdb")
	if err := run([]string{"init", "--name", "Headless", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := run([]string{"open", "--no-ui", projectPath}); err != nil {
		t.Fatalf("open --no-ui: %v", err)
	}

	if err := run([]string{"open", "--ui=false", "--no-ui", projectPath}); err != nil {
		t.Fatalf("open --ui=false --no-ui: %v", err)
	}
}

func TestOpenHeadlessModeWritesModeLine(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))
	projectPath := filepath.Join(tmp, "headless-log.qdb")

	if err := run([]string{"init", "--name", "Loggy", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	output, err := captureStdout(func() error {
		return run([]string{"open", "--no-ui", projectPath})
	})
	if err != nil {
		t.Fatalf("open --no-ui: %v", err)
	}
	if strings.TrimSpace(output) != "open mode: headless" {
		t.Fatalf("unexpected open output: %q", output)
	}
}

func TestOpenRejectsUnsupportedCatalogVersion(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "unsupported.duckdb")
	createBootstrapDb(t, dbPath)

	sqlDB, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := sqlDB.Exec("CREATE TABLE IF NOT EXISTS quackcess_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);"); err != nil {
		t.Fatalf("seed meta table: %v", err)
	}
	if _, err := sqlDB.Exec("INSERT OR REPLACE INTO quackcess_meta(key, value) VALUES ('schema_version', '2.0.0');"); err != nil {
		sqlDB.Close()
		t.Fatalf("seed unsupported version: %v", err)
	}
	sqlDB.Close()

	projectPath := filepath.Join(tmp, "bad.qdb")
	if err := run([]string{"init", "--name", "Bad", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := run([]string{"open", "--no-ui", projectPath}); err == nil {
		t.Fatalf("expected open to fail")
	}
}

func TestInitOpenInfoFlow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "flow.qdb")
	if err := run([]string{"init", "--name", "Flow", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := run([]string{"open", "--no-ui", projectPath}); err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := run([]string{"info", projectPath}); err != nil {
		t.Fatalf("info: %v", err)
	}

	p, err := project.Open(projectPath)
	if err != nil {
		t.Fatalf("open via project: %v", err)
	}
	if p.Manifest.ProjectName != "Flow" {
		t.Fatalf("unexpected project name %q", p.Manifest.ProjectName)
	}
}

func TestInfoOutputsExpectedKeys(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed.duckdb"))

	projectPath := filepath.Join(tmp, "info.qdb")
	if err := run([]string{"init", "--name", "Flow", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	output, err := captureStdout(func() error {
		return run([]string{"info", projectPath})
	})
	if err != nil {
		t.Fatalf("info: %v", err)
	}

	expectedLines := []string{
		"name=Flow",
		"format=quackcess.qdb",
		"version=1.0.0",
		"dataFile=database.duckdb",
		"artifactRoot=artifacts/",
		"createdBy=",
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < len(expectedLines) {
		t.Fatalf("info output expected at least %d lines, got %d in %q", len(expectedLines), len(lines), output)
	}
	for i, line := range expectedLines {
		if strings.HasPrefix(line, "createdBy=") {
			if !strings.HasPrefix(lines[i], "createdBy=") {
				t.Fatalf("info output line %d expected prefix %q, got %q", i, line, lines[i])
			}
			continue
		}
		if lines[i] != line {
			t.Fatalf("info output line %d expected %q, got %q", i, line, lines[i])
		}
	}
	if !strings.HasPrefix(lines[6], "vectorProviderStatus=") {
		t.Fatalf("info output missing vector provider status: %q", output)
	}
	foundBackend := false
	for _, line := range lines {
		if strings.HasPrefix(line, "vectorProviderBackend=") {
			foundBackend = true
			break
		}
	}
	if !foundBackend {
		t.Fatalf("info output missing vector provider backend: %q", output)
	}
}

func TestInfoVectorProviderStatusReflectsEnvErrors(t *testing.T) {
	tmp := t.TempDir()
	dbPath := createBootstrapDb(t, filepath.Join(tmp, "seed-vector-info.duckdb"))

	projectPath := filepath.Join(tmp, "info-vector.qdb")
	if err := run([]string{"init", "--name", "Flow", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", "http://localhost:11434/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "")
	t.Setenv("QUACKCESS_VECTOR_TIMEOUT_SECONDS", "0")

	output, err := captureStdout(func() error {
		return run([]string{"info", projectPath})
	})
	if err != nil {
		t.Fatalf("info: %v", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	found := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "vectorProviderStatus=error:") {
			found = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan output: %v", err)
	}
	if !found {
		t.Fatalf("expected vector provider status error, got %q", output)
	}
}

func captureStdout(runFn func() error) (string, error) {
	oldStdout := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer read.Close()
	os.Stdout = write
	defer func() { os.Stdout = oldStdout }()

	var got bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- runFn()
		write.Close()
	}()

	if _, err := io.Copy(&got, read); err != nil {
		return "", err
	}
	copyErr := <-errCh
	return got.String(), copyErr
}

func createBootstrapDb(t *testing.T, path string) string {
	t.Helper()
	database, err := db.Bootstrap(path)
	if err != nil {
		t.Fatalf("bootstrap db: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	return path
}
