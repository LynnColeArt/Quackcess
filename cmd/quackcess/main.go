package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/db"
	internalmcp "github.com/LynnColeArt/Quackcess/internal/mcp"
	"github.com/LynnColeArt/Quackcess/internal/project"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
	"github.com/LynnColeArt/Quackcess/internal/ui/gtk"
	"github.com/LynnColeArt/Quackcess/internal/ui/shell"
	"github.com/LynnColeArt/Quackcess/internal/vector"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var runShellWindowFn = runShellWindow
var runMCPServerFn = runMCPServer
var runInstallFn = runInstall

var (
	errEmbeddingModelsUnavailable   = errors.New("provider did not expose a models endpoint")
	errEmbeddingDownloadUnsupported = errors.New("provider does not expose model download endpoint")
)

const defaultWorkspacePathEnv = "QUACKCESS_WORKSPACE_PATH"

func run(args []string) error {
	if len(args) == 0 {
		return runDefaultWorkspaceLaunch()
	}

	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "open":
		return runOpen(args[1:])
	case "mcp":
		return runMCP(args[1:])
	case "info":
		return runInfo(args[1:])
	case "install":
		return runInstall(args[1:])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runDefaultWorkspaceLaunch() error {
	workspacePath, err := defaultWorkspacePath()
	if err != nil {
		return err
	}
	if strings.TrimSpace(workspacePath) == "" {
		return fmt.Errorf("default workspace path is empty")
	}

	workspaceDir := filepath.Dir(workspacePath)
	if workspaceDir != "" && workspaceDir != "." {
		if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(workspacePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(workspacePath), filepath.Ext(workspacePath))
		if strings.TrimSpace(name) == "" || name == "." {
			name = "workspace"
		}
		if err := runInit([]string{"--skip-vector-setup", "--name", name, workspacePath}); err != nil {
			return err
		}
	}

	return runOpen([]string{workspacePath})
}

func defaultWorkspacePath() (string, error) {
	customPath := strings.TrimSpace(os.Getenv(defaultWorkspacePathEnv))
	if customPath != "" {
		return customPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".quackcess", "workspace.qdb"), nil
}

func main() {
	flag.Parse()
	if err := run(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	name := fs.String("name", "", "project name")
	dbPath := fs.String("db", "", "path to duckdb source file")
	skipVectorSetup := fs.Bool("skip-vector-setup", false, "skip vector model setup during init")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("project path is required")
	}

	manifest := project.DefaultManifest()
	if *name != "" {
		manifest.ProjectName = *name
	}
	manifest.CreatedBy = os.Getenv("USER")
	if manifest.CreatedBy == "" {
		manifest.CreatedBy = os.Getenv("USERNAME")
	}
	if manifest.CreatedBy == "" {
		manifest.CreatedBy = "unknown"
	}

	if err := project.Create(fs.Arg(0), project.CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: *dbPath,
	}); err != nil {
		return err
	}

	if *skipVectorSetup {
		return nil
	}
	if err := runInstallFn([]string{}); err != nil {
		return fmt.Errorf("initial vector setup failed: %w", err)
	}
	return nil
}

func runOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	uiMode := fs.Bool("ui", true, "launch interactive shell UI (default: true)")
	headless := fs.Bool("no-ui", false, "skip interactive shell UI")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("project path is required")
	}

	p, err := project.Open(fs.Arg(0))
	if err != nil {
		return err
	}

	database, cleanup, err := openProjectDatabase(p)
	if err != nil {
		return err
	}
	defer cleanup()
	defer database.Close()

	if *headless {
		emitOpenModeMessage("headless")
		return nil
	}
	if !*uiMode {
		emitOpenModeMessage("headless")
		return nil
	}

	emitOpenModeMessage("ui")
	if err := runShellWindowFn(database, p); err != nil {
		if errors.Is(err, gtk.ErrGTKUnavailable) {
			emitOpenModeMessage("headless (ui unavailable)")
			return nil
		}
		return err
	}
	return nil
}

func runInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("project path is required")
	}

	p, err := project.Open(fs.Arg(0))
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(os.Stdout, "name=%s\nformat=%s\nversion=%s\ndataFile=%s\nartifactRoot=%s\ncreatedBy=%s\n",
		p.Manifest.ProjectName,
		p.Manifest.Format,
		p.Manifest.Version,
		p.Manifest.DataFile,
		p.Manifest.ArtifactRoot,
		p.Manifest.CreatedBy,
	)
	if err != nil {
		return err
	}

	vectorProviderStatus := "disabled"
	if cfg, enabled, cfgErr := vectorProviderConfigFromEnv(); cfgErr != nil {
		vectorProviderStatus = "error:" + cfgErr.Error()
		if _, err := fmt.Fprintf(os.Stdout, "vectorProviderStatus=%s\n", vectorProviderStatus); err != nil {
			return err
		}
		return nil
	} else if enabled {
		vectorProviderStatus = "enabled"
		_, err = fmt.Fprintf(os.Stdout, "vectorProviderStatus=%s\n", vectorProviderStatus)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(os.Stdout, "vectorProviderBackend=%s\nvectorProvider=%s\nvectorProviderModel=%s\nvectorProviderEndpoint=%s\nvectorProviderDimension=%d\nvectorProviderTimeoutSeconds=%d\nvectorProviderApiKeySet=%t\n",
			cfg.backend, cfg.http.Name, cfg.http.Model, cfg.http.Endpoint, cfg.http.Dimension, int(cfg.http.Timeout.Seconds()), cfg.http.APIKey != "",
		)
		return err
	}

	_, err = fmt.Fprintf(os.Stdout, "vectorProviderStatus=%s\n", vectorProviderStatus)
	return err
}

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("install command does not accept arguments")
	}

	cfg, enabled, err := vectorProviderConfigFromEnv()
	if err != nil {
		return err
	}
	if !enabled {
		_, err = fmt.Fprintln(os.Stdout, "vector provider disabled")
		return err
	}

	if cfg.backend == "cpu" {
		_, err = fmt.Fprintf(os.Stdout, "vector backend ready: %s\n", cfg.backend)
		return err
	}

	providerURL, err := url.Parse(cfg.http.Endpoint)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: cfg.http.Timeout}
	ctx := context.Background()

	available, modelCatalogErr := isEmbeddingModelListed(ctx, client, providerURL, cfg.http.Model)
	if modelCatalogErr == nil {
		if !available {
			_, err = fmt.Fprintln(os.Stdout, "vector model not loaded, attempting download")
			if err != nil {
				return err
			}
			if err := installEmbeddingModel(ctx, client, providerURL, cfg.http.Model); err != nil {
				if errors.Is(err, errEmbeddingDownloadUnsupported) {
					return fmt.Errorf("vector model not available: %s", cfg.http.Model)
				}
				return fmt.Errorf("vector model not available: %s: %w", cfg.http.Model, err)
			}
			var availableAfterInstall bool
			availableAfterInstall, err = isEmbeddingModelAvailable(ctx, client, providerURL, cfg.http.Model, cfg.http.Dimension)
			if err != nil {
				return err
			}
			available = availableAfterInstall
		}
	} else if errors.Is(modelCatalogErr, errEmbeddingModelsUnavailable) {
		if err := probeEmbeddingModel(ctx, client, providerURL, strings.ToLower(strings.TrimSpace(cfg.http.Model)), cfg.http.Dimension); err != nil {
			if err := installEmbeddingModel(ctx, client, providerURL, cfg.http.Model); err != nil {
				if errors.Is(err, errEmbeddingDownloadUnsupported) {
					return fmt.Errorf("vector model not available: %s", cfg.http.Model)
				}
				return fmt.Errorf("vector model not available: %s: %w", cfg.http.Model, err)
			}
			var availableAfterInstall bool
			availableAfterInstall, err = isEmbeddingModelAvailable(ctx, client, providerURL, cfg.http.Model, cfg.http.Dimension)
			if err != nil {
				return err
			}
			if availableAfterInstall {
				available = true
			}
		} else {
			available = true
		}
	} else {
		return modelCatalogErr
	}

	if !available {
		return fmt.Errorf("vector model not available: %s", cfg.http.Model)
	}

	_, err = fmt.Fprintf(os.Stdout, "vector model ready: backend=%s provider=%s model=%s\n", cfg.backend, cfg.http.Name, cfg.http.Model)
	return err
}

func isEmbeddingModelListed(ctx context.Context, client *http.Client, endpoint *url.URL, model string) (bool, error) {
	if client == nil {
		client = &http.Client{Timeout: time.Second}
	}
	if endpoint == nil {
		return false, fmt.Errorf("missing embedding provider endpoint")
	}

	knownModel := strings.ToLower(strings.TrimSpace(model))
	if knownModel == "" {
		return false, fmt.Errorf("provider model is required")
	}

	models, err := fetchProviderModels(ctx, client, endpoint)
	if err != nil {
		return false, err
	}

	for _, current := range models {
		if strings.EqualFold(current, knownModel) {
			return true, nil
		}
	}
	return false, nil
}

func isEmbeddingModelAvailable(ctx context.Context, client *http.Client, endpoint *url.URL, model string, expectedDimension int) (bool, error) {
	if client == nil {
		client = &http.Client{Timeout: time.Second}
	}
	if endpoint == nil {
		return false, fmt.Errorf("missing embedding provider endpoint")
	}

	knownModel := strings.ToLower(strings.TrimSpace(model))
	if knownModel == "" {
		return false, fmt.Errorf("provider model is required")
	}

	listed, err := isEmbeddingModelListed(ctx, client, endpoint, model)
	if err == nil {
		return listed, nil
	}
	if !errors.Is(err, errEmbeddingModelsUnavailable) {
		return false, err
	}

	if err := probeEmbeddingModel(ctx, client, endpoint, knownModel, expectedDimension); err != nil {
		return false, err
	}
	return true, nil
}

func installEmbeddingModel(ctx context.Context, client *http.Client, endpoint *url.URL, model string) error {
	if client == nil {
		client = &http.Client{Timeout: time.Second}
	}
	if endpoint == nil {
		return fmt.Errorf("missing embedding provider endpoint")
	}

	modelToDownload := strings.TrimSpace(model)
	if modelToDownload == "" {
		return fmt.Errorf("provider model is required")
	}

	payload, err := json.Marshal(map[string]any{
		"name":   modelToDownload,
		"stream": false,
	})
	if err != nil {
		return fmt.Errorf("marshal model download request: %w", err)
	}

	pullCandidatePaths := modelDownloadCandidatePaths(endpoint)
	if len(pullCandidatePaths) == 0 {
		return errEmbeddingDownloadUnsupported
	}

	var lastErr error
	for _, path := range pullCandidatePaths {
		requestURL := *endpoint
		requestURL.Path = path
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), strings.NewReader(string(payload)))
		if err != nil {
			return fmt.Errorf("create model download request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			lastErr = fmt.Errorf("model download request failed: %w", err)
			continue
		}
		body, readErr := io.ReadAll(response.Body)
		if closeErr := response.Body.Close(); closeErr != nil && readErr == nil {
			readErr = closeErr
		}
		if readErr != nil {
			lastErr = fmt.Errorf("read model download response: %w", readErr)
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			lastErr = fmt.Errorf("model download endpoint %s returned %d: %s", path, response.StatusCode, trimText(string(body), 600))
			continue
		}
		if responseHasModelError(body) {
			lastErr = fmt.Errorf("model download endpoint %s rejected model %q", path, modelToDownload)
			continue
		}
		return nil
	}

	if lastErr == nil {
		return errEmbeddingDownloadUnsupported
	}
	return lastErr
}

func responseHasModelError(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}

	var payload struct {
		Error any `json:"error"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	return payload.Error != nil
}

func fetchProviderModels(ctx context.Context, client *http.Client, endpoint *url.URL) ([]string, error) {
	if client == nil {
		client = &http.Client{}
	}
	var (
		lastErr error
		models  []string
	)
	for _, path := range modelListCandidatePaths(endpoint) {
		candidate := *endpoint
		candidate.Path = path
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("create models request: %w", err)
		}
		response, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("provider models request failed: %w", err)
			continue
		}
		func() {
			defer response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				lastErr = fmt.Errorf("provider models request returned %d", response.StatusCode)
				return
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				lastErr = fmt.Errorf("read models response: %w", err)
				return
			}
			modelList, parseErr := parseProviderModelList(body)
			if parseErr != nil {
				lastErr = parseErr
				return
			}
			lastErr = nil
			models = modelList
		}()
		if lastErr == nil {
			return models, nil
		}
	}

	if lastErr == nil {
		return nil, errEmbeddingModelsUnavailable
	}
	return nil, errEmbeddingModelsUnavailable
}

func modelListCandidatePaths(endpoint *url.URL) []string {
	basePaths := modelCatalogBasePaths(endpoint)
	paths := make([]string, 0, len(basePaths)*4)
	seen := map[string]struct{}{}
	buildPath := func(root, suffix string) string {
		root = strings.TrimSuffix(root, "/")
		suffix = strings.TrimPrefix(strings.TrimSpace(suffix), "/")
		if root == "" || root == "/" {
			return "/" + suffix
		}
		return root + "/" + suffix
	}
	for _, base := range basePaths {
		suffixes := []string{
			"models",
			"api/tags",
		}
		if base != "/v1" && !strings.HasSuffix(base, "/v1") {
			suffixes = append(suffixes, "v1/models", "v1/api/tags")
		}
		for _, suffix := range suffixes {
			candidate := buildPath(base, suffix)
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			paths = append(paths, candidate)
		}
	}
	return paths
}

func modelDownloadCandidatePaths(endpoint *url.URL) []string {
	basePaths := modelCatalogBasePaths(endpoint)
	paths := make([]string, 0, len(basePaths)*2)
	seen := map[string]struct{}{}
	buildPath := func(root, suffix string) string {
		root = strings.TrimSuffix(root, "/")
		suffix = strings.TrimPrefix(strings.TrimSpace(suffix), "/")
		if root == "" || root == "/" {
			return "/" + suffix
		}
		return root + "/" + suffix
	}
	for _, base := range basePaths {
		suffixes := []string{
			"api/pull",
		}
		if base != "/v1" && !strings.HasSuffix(base, "/v1") {
			suffixes = append(suffixes, "v1/api/pull")
		}
		for _, suffix := range suffixes {
			candidate := buildPath(base, suffix)
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			paths = append(paths, candidate)
		}
	}
	return paths
}

func modelCatalogBasePaths(endpoint *url.URL) []string {
	rawPath := strings.TrimSpace(endpoint.Path)
	rawPath = strings.TrimSuffix(rawPath, "/")
	if rawPath == "" {
		rawPath = "/"
	}
	if strings.HasSuffix(rawPath, "/embeddings") {
		rawPath = strings.TrimSuffix(rawPath, "/embeddings")
	}
	rawPath = strings.TrimSuffix(rawPath, "/")
	if rawPath == "" {
		rawPath = "/"
	}

	paths := make([]string, 0, 3)
	paths = append(paths, rawPath)
	if rawPath != "/" {
		paths = append(paths, "/")
	}
	seen := map[string]struct{}{}
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		filtered = append(filtered, path)
	}
	return filtered
}

func probeEmbeddingModel(ctx context.Context, client *http.Client, endpoint *url.URL, model string, expectedDimension int) error {
	payload, err := json.Marshal(map[string]any{
		"model": model,
		"input": []string{"__quackcess-vector-model-check"},
	})
	if err != nil {
		return fmt.Errorf("marshal probe request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("create probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("provider probe failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("provider probe response read: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("provider probe returned %d: %s", response.StatusCode, trimText(string(body), 600))
	}
	if len(body) == 0 {
		return fmt.Errorf("provider probe returned empty response")
	}
	if err := validateEmbeddingResponseForProbe(body, expectedDimension); err != nil {
		return err
	}
	return nil
}

func validateEmbeddingResponseForProbe(raw []byte, expectedDimension int) error {
	var response struct {
		Data []struct {
			Index     int         `json:"index"`
			Embedding []float64   `json:"embedding"`
			Object    string      `json:"object"`
			Error     interface{} `json:"error"`
		} `json:"data"`
		Embedding []float64 `json:"embedding"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return fmt.Errorf("decode provider probe response: %w", err)
	}
	if expectedDimension <= 0 {
		expectedDimension = 1
	}
	if len(response.Data) > 0 {
		if len(response.Data) != 1 {
			return fmt.Errorf("provider probe returned %d embeddings, expected 1", len(response.Data))
		}
		if len(response.Data[0].Embedding) != expectedDimension {
			return fmt.Errorf("provider probe returned embedding dimension %d, expected %d", len(response.Data[0].Embedding), expectedDimension)
		}
		return nil
	}
	if len(response.Embedding) > 0 {
		if len(response.Embedding) != expectedDimension {
			return fmt.Errorf("provider probe returned embedding dimension %d, expected %d", len(response.Embedding), expectedDimension)
		}
		return nil
	}
	return fmt.Errorf("provider probe response missing embedding payload")
}

func trimText(value string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func parseProviderModelList(raw []byte) ([]string, error) {
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	if modelsRaw, ok := generic["data"]; ok {
		if models, ok := modelsRaw.([]any); ok {
			result := make([]string, 0, len(models))
			for _, itemRaw := range models {
				if itemMap, ok := itemRaw.(map[string]any); ok {
					if idRaw, found := itemMap["id"]; found {
						if id, ok := idRaw.(string); ok {
							result = append(result, id)
						}
					}
					if nameRaw, found := itemMap["name"]; found {
						if name, ok := nameRaw.(string); ok && name != "" {
							result = append(result, name)
						}
					}
				}
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	}
	if modelsRaw, ok := generic["models"]; ok {
		if items, ok := modelsRaw.([]any); ok {
			result := make([]string, 0, len(items))
			for _, itemRaw := range items {
				if name, ok := itemRaw.(string); ok {
					result = append(result, name)
				}
			}
			if len(result) > 0 {
				return result, nil
			}
		}
	}
	return nil, fmt.Errorf("provider models response missing recognized model list")
}

func runMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	principal := fs.String("principal", "agent", "tool principal used for core ACL checks")
	permissionMatrixPath := fs.String("permission-matrix", "", "path to MCP permission matrix json file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("project path is required")
	}

	return runMCPServerFn(fs.Arg(0), *principal, *permissionMatrixPath)
}

func runMCPServer(projectPath string, principal string, permissionMatrixPath string) error {
	p, err := project.Open(projectPath)
	if err != nil {
		return err
	}

	database, cleanup, err := openProjectDatabase(p)
	if err != nil {
		return err
	}
	defer cleanup()
	defer database.Close()

	catalogExplorer := newCatalogExplorer(database)
	terminalService := terminal.NewTerminalServiceWithCanvasRepository(database.SQL, nil, catalog.NewCanvasRepository(database.SQL))
	artifactStore := internalmcp.NewMemoryArtifactStore()
	vectorService := newVectorService(database)
	eventBus := internalmcp.NewEventBus()

	authorizer := internalmcp.NewAllowlistAuthorizer(true)
	if strings.TrimSpace(permissionMatrixPath) != "" {
		authorizer, err = internalmcp.LoadPermissionMatrix(permissionMatrixPath)
		if err != nil {
			return err
		}
	}

	core := internalmcp.NewServer(authorizer, eventBus)
	if err := internalmcp.RegisterCoreTools(core, internalmcp.CoreTools{
		QueryRunner:    terminalService,
		CatalogService: catalogExplorer,
		Artifacts:      artifactStore,
		Reports:        p,
		Vector:         vectorService,
		EventBus:       eventBus,
	}); err != nil {
		return err
	}

	server, err := internalmcp.NewSDKServer(core, internalmcp.SDKServerOptions{
		Implementation: &mcpsdk.Implementation{
			Name:    "quackcess",
			Version: "0.1.0",
		},
	}, principal)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return server.Run(ctx, &mcpsdk.StdioTransport{})
}

func openProjectDatabase(p *project.Project) (*db.DB, func() error, error) {
	data, err := p.ReadDataFile()
	if err != nil {
		return nil, nil, err
	}

	tmp, err := os.CreateTemp("", "quackcess-*.duckdb")
	if err != nil {
		return nil, nil, err
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return nil, nil, err
	}

	if err := os.WriteFile(tmpName, data, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return nil, nil, err
	}

	database, err := db.Bootstrap(tmpName)
	if err != nil {
		_ = os.Remove(tmpName)
		return nil, nil, err
	}

	cleanup := func() error {
		return os.Remove(tmpName)
	}
	return database, cleanup, nil
}

func runShellWindow(database *db.DB, p *project.Project) error {
	if p == nil {
		return fmt.Errorf("project is required")
	}

	vectorSpecWriter := func(input string, metadata terminal.TerminalVectorizeMetadata) error {
		spec := project.VectorOperationSpec{
			ArtifactSpecV1: project.ArtifactSpecV1{
				ID:            newVectorOperationArtifactID(metadata),
				Kind:          project.ArtifactKindVectorOp,
				SchemaVersion: project.CurrentArtifactSchemaVersion(),
			},
			SourceTable:  metadata.TableName,
			SourceColumn: metadata.SourceColumn,
			TargetColumn: metadata.TargetColumn,
			Filter:       metadata.Filter,
			FieldID:      metadata.FieldID,
			Built:        metadata.Built,
			BatchSize:    metadata.BatchSize,
			VectorCount:  metadata.VectorCount,
			SkipReason:   metadata.SkipReason,
			ExecutedBy:   "terminal",
			ExecutedAt:   time.Now().Format(time.RFC3339Nano),
			CommandText:  input,
		}
		payload, err := project.MarshalVectorOperationSpec(spec)
		if err != nil {
			return err
		}
		return p.UpsertArtifact(payload)
	}

	console := terminal.NewEventConsole(terminal.DefaultMaxConsoleEvents)
	catalogExplorer := newCatalogExplorer(database)
	canvasRepository := catalog.NewCanvasRepository(database.SQL)
	vectorService := newVectorService(database)
	commandBus := appstate.NewShellCommandBusWithVectorWriter(
		terminal.NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, console, canvasRepository, vectorService),
		appstate.NewShellStateWithCatalogExplorer(console, catalogExplorer),
		vectorSpecWriter,
	)
	presenter := shell.NewShellPresenter(
		shell.NewShellModel(commandBus),
		nil,
	)
	window, err := gtk.NewShellWindow(gtk.NewShellBridge(presenter), nil)
	if err != nil {
		return err
	}
	if err := window.Run(); err != nil {
		return err
	}
	return nil
}

func newVectorOperationArtifactID(metadata terminal.TerminalVectorizeMetadata) string {
	normalizedField := strings.TrimSpace(metadata.FieldID)
	if normalizedField == "" {
		normalizedField = "unknown"
	}
	sanitized := strings.NewReplacer(`"`, "", "`", "", "\n", "-", "\r", "-", " ", "-", "	", "-").Replace(normalizedField)
	sum := sha1.Sum([]byte(metadata.TableName + "|" + metadata.SourceColumn + "|" + metadata.TargetColumn + "|" + metadata.Filter))
	return "vector-op-" + strings.Trim(sanitized, "-") + "-" + strings.ToUpper(hex.EncodeToString(sum[:4]))
}

type mcpVectorService struct {
	repo               *catalog.VectorFieldRepository
	rebuildSvc         *vector.VectorRebuildService
	searchSvc          *vector.VectorSearchService
	providerSetupError string
}

func (svc mcpVectorService) ListVectorFields() ([]vector.VectorField, error) {
	return svc.repo.List()
}

func (svc mcpVectorService) RebuildVector(fieldID string, force bool) (vector.VectorBuildResult, error) {
	return svc.RebuildVectorWithProgress(fieldID, force)
}

func (svc mcpVectorService) RebuildVectorWithFilter(fieldID string, filter string, force bool) (vector.VectorBuildResult, error) {
	if svc.providerSetupError != "" {
		return vector.VectorBuildResult{}, fmt.Errorf("vector provider setup: %v", svc.providerSetupError)
	}
	if svc.rebuildSvc == nil {
		return vector.VectorBuildResult{}, fmt.Errorf("vector rebuild service is not configured")
	}
	return svc.rebuildSvc.RebuildVectorWithFilter(fieldID, filter, force)
}

const (
	defaultVectorProviderName      = "qwen-local"
	defaultVectorCPUProviderName   = "qwen-cpu"
	defaultVectorLlamaProviderName = "qwen-cpp"
	defaultVectorModel             = "qwen3.5-0.8b"
	defaultVectorCPUModel          = "qwen3-embedding-0.6b"
	defaultVectorLlamaModel        = "qwen3-embedding-0.6b"
	defaultVectorEndpoint          = "http://127.0.0.1:11434/v1/embeddings"
	defaultVectorLlamaEndpoint     = "http://127.0.0.1:8080/v1/embeddings"
	defaultVectorDimension         = 1024
	defaultVectorTimeoutSeconds    = 30
	defaultVectorBackend           = "cpu"
)

type vectorProviderConfig struct {
	backend string
	http    vector.HTTPEmbeddingProviderConfig
	cpuSeed uint64
}

func (svc mcpVectorService) RebuildVectorWithProgress(
	fieldID string,
	force bool,
	progressCallbacks ...vector.VectorBuildProgressHandler,
) (vector.VectorBuildResult, error) {
	if svc.providerSetupError != "" {
		return vector.VectorBuildResult{}, fmt.Errorf("vector provider setup: %v", svc.providerSetupError)
	}
	if svc.rebuildSvc == nil {
		return vector.VectorBuildResult{}, fmt.Errorf("vector rebuild service is not configured")
	}
	return svc.rebuildSvc.RebuildVectorWithProgress(fieldID, force, progressCallbacks...)
}

func (svc mcpVectorService) SearchVector(fieldID string, queryText string, limit int) (vector.VectorSearchResult, error) {
	if svc.providerSetupError != "" {
		return vector.VectorSearchResult{}, fmt.Errorf("vector provider setup: %v", svc.providerSetupError)
	}
	if svc.searchSvc == nil {
		return vector.VectorSearchResult{}, fmt.Errorf("vector search service is not configured")
	}
	return svc.searchSvc.SearchByFieldID(context.Background(), fieldID, queryText, limit)
}

func newVectorService(database *db.DB) mcpVectorService {
	repo := catalog.NewVectorFieldRepository(database.SQL)
	registry := vector.NewEmbeddingProviderRegistry()
	buildService := vector.NewVectorBuildService(registry)
	var providerSetupError string

	vectorConfig, enabled, err := vectorProviderConfigFromEnv()
	if err != nil {
		providerSetupError = err.Error()
	}
	if enabled {
		var embeddingProvider vector.EmbeddingProvider
		switch vectorConfig.backend {
		case "cpu":
			embeddingProvider, err = vector.NewCPUEmbeddingProvider(vector.CPUEmbeddingProviderConfig{
				Name:      vectorConfig.http.Name,
				Model:     vectorConfig.http.Model,
				Dimension: vectorConfig.http.Dimension,
				Seed:      vectorConfig.cpuSeed,
			})
		default:
			embeddingProvider, err = vector.NewHTTPEmbeddingProvider(vectorConfig.http)
		}
		if err != nil {
			providerSetupError = err.Error()
		} else if err := registry.Register(vectorConfig.http.Name, vectorConfig.http.Model, embeddingProvider); err != nil {
			providerSetupError = err.Error()
		}
	}

	rebuildService := vector.NewVectorRebuildService(database.SQL, repo, buildService)
	searchService := vector.NewVectorSearchService(database.SQL, repo, buildService)
	return mcpVectorService{
		repo:               repo,
		rebuildSvc:         rebuildService,
		searchSvc:          searchService,
		providerSetupError: providerSetupError,
	}
}

func vectorProviderConfigFromEnv() (vectorProviderConfig, bool, error) {
	rawBackendEnv := strings.TrimSpace(os.Getenv("QUACKCESS_VECTOR_BACKEND"))
	rawBackend, err := normalizeVectorBackend(strings.ToLower(rawBackendEnv))
	if err != nil {
		return vectorProviderConfig{}, true, err
	}

	rawEndpoint := strings.TrimSpace(os.Getenv("QUACKCESS_VECTOR_ENDPOINT"))
	rawProvider := strings.TrimSpace(os.Getenv("QUACKCESS_VECTOR_PROVIDER"))
	rawModel := strings.TrimSpace(os.Getenv("QUACKCESS_VECTOR_MODEL"))
	rawDimension := strings.TrimSpace(os.Getenv("QUACKCESS_VECTOR_DIMENSION"))
	rawAPIKey := strings.TrimSpace(os.Getenv("QUACKCESS_VECTOR_API_KEY"))
	rawTimeout := strings.TrimSpace(os.Getenv("QUACKCESS_VECTOR_TIMEOUT_SECONDS"))
	rawCPUSeed := strings.TrimSpace(os.Getenv("QUACKCESS_VECTOR_CPU_SEED"))

	if rawProvider == "" {
		if rawBackend == "cpu" {
			rawProvider = defaultVectorCPUProviderName
		} else if rawBackend == "llamacpp" {
			rawProvider = defaultVectorLlamaProviderName
		} else {
			rawProvider = defaultVectorProviderName
		}
	}
	if rawModel == "" {
		if rawBackend == "cpu" {
			rawModel = defaultVectorCPUModel
		} else if rawBackend == "llamacpp" {
			rawModel = defaultVectorLlamaModel
		} else {
			rawModel = defaultVectorModel
		}
	}
	if rawEndpoint == "" {
		if rawBackend == "llamacpp" {
			rawEndpoint = defaultVectorLlamaEndpoint
		} else {
			rawEndpoint = defaultVectorEndpoint
		}
	}
	dimension := defaultVectorDimension
	if rawDimension != "" {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(rawDimension))
		if parseErr != nil || parsed <= 0 {
			return vectorProviderConfig{}, true, fmt.Errorf("invalid QUACKCESS_VECTOR_DIMENSION: %q", rawDimension)
		}
		dimension = parsed
	}
	if dimension <= 0 {
		return vectorProviderConfig{}, true, fmt.Errorf("invalid QUACKCESS_VECTOR_DIMENSION: %q", rawDimension)
	}

	cpuSeed := uint64(0)
	if rawCPUSeed != "" {
		parsed, parseErr := strconv.ParseUint(rawCPUSeed, 10, 64)
		if parseErr != nil {
			return vectorProviderConfig{}, true, fmt.Errorf("invalid QUACKCESS_VECTOR_CPU_SEED: %q", rawCPUSeed)
		}
		cpuSeed = parsed
	}

	timeout := time.Duration(defaultVectorTimeoutSeconds) * time.Second
	if rawTimeout != "" {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(rawTimeout))
		if parseErr != nil || parsed <= 0 {
			return vectorProviderConfig{}, true, fmt.Errorf("invalid QUACKCESS_VECTOR_TIMEOUT_SECONDS: %q", rawTimeout)
		}
		timeout = time.Duration(parsed) * time.Second
	}

	cfg := vectorProviderConfig{
		backend: rawBackend,
		http: vector.HTTPEmbeddingProviderConfig{
			Name:      rawProvider,
			Model:     rawModel,
			Endpoint:  rawEndpoint,
			Dimension: dimension,
			APIKey:    rawAPIKey,
			Timeout:   timeout,
		},
		cpuSeed: cpuSeed,
	}

	if cfg.backend == "cpu" {
		cfg.http.Endpoint = ""
		cfg.http.APIKey = ""
	}
	return cfg, true, nil
}

func normalizeVectorBackend(raw string) (string, error) {
	switch raw {
	case "":
		return defaultVectorBackend, nil
	case "cpu":
		return "cpu", nil
	case "http", "openai":
		return "http", nil
	case "llama", "llama-cpp", "llama_cpp", "llamacpp", "llama.cpp":
		return "llamacpp", nil
	default:
		return "", fmt.Errorf("invalid QUACKCESS_VECTOR_BACKEND: %q", raw)
	}
}

type shellCatalogExplorer struct {
	db *db.DB
}

func (s shellCatalogExplorer) ListTables() ([]string, error) {
	repository := catalog.NewTableRepository(s.db.SQL)
	tables, err := repository.List()
	if err != nil {
		return nil, err
	}
	tablesList := make([]string, 0, len(tables))
	for _, table := range tables {
		tablesList = append(tablesList, table.Name)
	}
	return tablesList, nil
}

func (s shellCatalogExplorer) ListViews() ([]string, error) {
	repository := catalog.NewViewRepository(s.db.SQL)
	views, err := repository.List()
	if err != nil {
		return nil, err
	}
	viewsList := make([]string, 0, len(views))
	for _, view := range views {
		viewsList = append(viewsList, view.Name)
	}
	return viewsList, nil
}

func (s shellCatalogExplorer) ListCanvases() ([]string, error) {
	repository := catalog.NewCanvasRepository(s.db.SQL)
	canvases, err := repository.List()
	if err != nil {
		return nil, err
	}
	canvasesList := make([]string, 0, len(canvases))
	for _, canvas := range canvases {
		canvasesList = append(canvasesList, canvas.Name)
	}
	return canvasesList, nil
}

func (s shellCatalogExplorer) LoadCanvasSpec(name string) (string, error) {
	repository := catalog.NewCanvasRepository(s.db.SQL)
	canvas, err := repository.FindByName(name)
	if err != nil {
		return "", err
	}
	return canvas.SpecJSON, nil
}

func newCatalogExplorer(database *db.DB) shellCatalogExplorer {
	return shellCatalogExplorer{db: database}
}

func emitOpenModeMessage(mode string) {
	fmt.Fprintf(os.Stdout, "open mode: %s\n", mode)
}
