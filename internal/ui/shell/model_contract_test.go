package shell

import (
	"errors"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

type fakeRunner struct {
	calls  int
	result terminal.TerminalResult
	err    error
}

func (f *fakeRunner) RunCommand(input string) (terminal.TerminalResult, error) {
	f.calls++
	if f.err != nil {
		return terminal.TerminalResult{}, f.err
	}
	f.result.Input = input
	return f.result, nil
}

func TestUIShellModelProjectsStateAndTerminalResults(t *testing.T) {
	runner := &fakeRunner{
		result: terminal.TerminalResult{
			Kind:      terminal.TerminalKindHistory,
			SQLText:   "SELECT 1",
			RowCount:  1,
			ErrorText: "",
		},
	}
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	if err := model.Execute("SELECT 1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner.calls = %d, want 1", runner.calls)
	}
	projection := model.Projection()
	if projection.ConsoleVisible {
		t.Fatal("expected console hidden by default")
	}
	if projection.LastInput != "SELECT 1" {
		t.Fatalf("last input = %q, want SELECT 1", projection.LastInput)
	}
	if projection.LastKind != terminal.TerminalKindHistory {
		t.Fatalf("last kind = %q, want %q", projection.LastKind, terminal.TerminalKindHistory)
	}
	if projection.RowCount != 1 {
		t.Fatalf("row count = %d, want 1", projection.RowCount)
	}
}

func TestUIShellModelDispatchesShortcutForConsoleVisibility(t *testing.T) {
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))

	if err := model.HandleShortcut("F12"); err != nil {
		t.Fatalf("handle shortcut: %v", err)
	}
	if !model.Projection().ConsoleVisible {
		t.Fatal("expected console visible after F12 shortcut")
	}
	if !state.IsConsoleVisible() {
		t.Fatal("expected underlying state model to remain in sync")
	}
}

func TestUIShellModelHandlesNoBusGracefully(t *testing.T) {
	model := NewShellModel(nil)
	if err := model.Execute("SELECT 1"); err == nil {
		t.Fatal("expected execute without bus error")
	}
}

func TestUIShellModelPropagatesTerminalFailures(t *testing.T) {
	fail := errors.New("executor failed")
	runner := &fakeRunner{err: fail}
	model := NewShellModel(appstate.NewShellCommandBus(runner, appstate.NewShellState(nil)))

	if err := model.Execute("SELECT 2"); err != fail {
		t.Fatalf("execute failure = %v, want %v", err, fail)
	}
	if model.Projection().LastStatus != "idle" {
		t.Fatalf("status = %q, want idle", model.Projection().LastStatus)
	}
}

func TestUIShellModelAddCanvasNodeMutatesProjection(t *testing.T) {
	state := appstate.NewShellStateWithCatalogExplorer(nil, &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[{"id":"customers","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))

	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.AddCanvasNode(query.CanvasNode{
		ID:     "shipments",
		Table:  "shipments",
		Fields: []query.CanvasField{{Name: "id"}},
	}); err != nil {
		t.Fatalf("add node: %v", err)
	}
	if !model.Projection().CanvasDirty {
		t.Fatal("expected canvas dirty")
	}
}

func TestUIShellModelAddCanvasEdgeMutatesProjection(t *testing.T) {
	state := appstate.NewShellStateWithCatalogExplorer(nil, &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"region"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"},{"name":"total"}]}
			],"edges":[]}`,
		},
	})
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.AddCanvasEdge(query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "id",
		ToColumn:   "customer_id",
		JoinType:   string(query.JoinInner),
	}); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	spec, err := query.ParseCanvasSpec([]byte(model.Projection().CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft spec: %v", err)
	}
	if len(spec.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(spec.Edges))
	}
}

func TestUIShellModelPatchCanvasEdgeMutatesProjection(t *testing.T) {
	state := appstate.NewShellStateWithCatalogExplorer(nil, &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"region"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"},{"name":"total"}]}
			],"edges":[
				{"id":"join-1","kind":"join","from":"customers","to":"orders","fromColumn":"id","toColumn":"customer_id","joinType":"INNER"}
			]}`,
		},
	})
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.PatchCanvasEdge(query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "region",
		ToColumn:   "total",
		JoinType:   string(query.JoinLeft),
	}); err != nil {
		t.Fatalf("patch edge: %v", err)
	}
	spec, err := query.ParseCanvasSpec([]byte(model.Projection().CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft spec: %v", err)
	}
	if len(spec.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(spec.Edges))
	}
	if spec.Edges[0].FromColumn != "region" || spec.Edges[0].ToColumn != "total" {
		t.Fatalf("edge columns = %q -> %q, want region -> total", spec.Edges[0].FromColumn, spec.Edges[0].ToColumn)
	}
	if spec.Edges[0].JoinType != string(query.JoinLeft) {
		t.Fatalf("join type = %q, want %q", spec.Edges[0].JoinType, string(query.JoinLeft))
	}
}

func TestUIShellModelDeleteCanvasEdgeMutatesProjection(t *testing.T) {
	state := appstate.NewShellStateWithCatalogExplorer(nil, &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"}]}
			],"edges":[
				{"id":"join-1","kind":"join","from":"customers","to":"orders","fromColumn":"id","toColumn":"customer_id","joinType":"INNER"}
			]}`,
		},
	})
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.DeleteCanvasEdge("join-1"); err != nil {
		t.Fatalf("delete edge: %v", err)
	}
	spec, err := query.ParseCanvasSpec([]byte(model.Projection().CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft spec: %v", err)
	}
	if len(spec.Edges) != 0 {
		t.Fatalf("edges = %d, want 0", len(spec.Edges))
	}
}

func TestUIShellModelSetCanvasNodeFieldsMutatesProjection(t *testing.T) {
	state := appstate.NewShellStateWithCatalogExplorer(nil, &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"name"}],"selected_fields":["id"]}
			],"edges":[]}`,
		},
	})
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.SetCanvasNodeFields("customers", []string{"name", "missing", "name"}); err != nil {
		t.Fatalf("set node fields: %v", err)
	}
	spec, err := query.ParseCanvasSpec([]byte(model.Projection().CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft spec: %v", err)
	}
	if len(spec.Nodes[0].SelectedFields) != 1 || spec.Nodes[0].SelectedFields[0] != "name" {
		t.Fatalf("selected fields = %#v, want [name]", spec.Nodes[0].SelectedFields)
	}
}

func TestUIShellModelRunActiveCanvasExecutesPreview(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(nil, explorer)
	runner := &fakeRunner{
		result: terminal.TerminalResult{
			Kind:     terminal.TerminalKindQuery,
			RowCount: 1,
		},
	}
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	spec, err := query.ParseCanvasSpec([]byte(`{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`))
	if err != nil {
		t.Fatalf("parse canvas spec: %v", err)
	}
	expected, err := query.GenerateSQLFromCanvas(spec)
	if err != nil {
		t.Fatalf("generate sql: %v", err)
	}

	if err := model.RunActiveCanvas(); err != nil {
		t.Fatalf("run active canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, expected.SQL) {
		t.Fatalf("runner input = %q, want contains %q", runner.result.Input, expected.SQL)
	}
	if projection := model.Projection(); projection.LastKind != terminal.TerminalKindQuery {
		t.Fatalf("last kind = %q, want query", projection.LastKind)
	}
}

func TestUIShellModelCreateCanvasDispatchesCanvasNew(t *testing.T) {
	runner := &fakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "created canvas project-overview",
		},
	}
	state := appstate.NewShellState(nil)
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	if err := model.CreateCanvas("project-overview"); err != nil {
		t.Fatalf("create canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, `\canvas new `) {
		t.Fatalf("runner input = %q, want prefix \\canvas new ", runner.result.Input)
	}
	if projection := model.Projection(); projection.CanvasStatus != "canvas created: project-overview" {
		t.Fatalf("canvas status = %q, want canvas created: project-overview", projection.CanvasStatus)
	}
}

func TestUIShellModelRenameCanvasDispatchesCanvasRename(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	runner := &fakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "renamed canvas sales -> sales-archive",
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(nil, explorer)
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.RenameCanvas("sales", "sales-archive"); err != nil {
		t.Fatalf("rename canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, `\canvas rename sales sales-archive`) {
		t.Fatalf("runner input = %q, want rename command", runner.result.Input)
	}
	if got := model.Projection().ActiveCanvas; got != "sales-archive" {
		t.Fatalf("active canvas = %q, want sales-archive", got)
	}
}

func TestUIShellModelShortcutSaveRevertsThroughModelContract(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	runner := &fakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "saved canvas sales",
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(nil, explorer)
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.SetCanvasDraft(`{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"created_at"}]}],"edges":[]}`); err != nil {
		t.Fatalf("set canvas draft: %v", err)
	}
	if err := model.HandleShortcut("Ctrl+S"); err != nil {
		t.Fatalf("save shortcut: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.HasPrefix(runner.result.Input, `\canvas save sales`) {
		t.Fatalf("runner input = %q, want \\canvas save sales", runner.result.Input)
	}

	stateProjection := model.Projection()
	if stateProjection.CanvasDirty {
		t.Fatal("expected dirty cleared after shortcut save")
	}
}

func TestUIShellModelShortcutRevertRunsRevertCommand(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	runner := &fakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindQuery,
			RowCount: 1,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(nil, explorer)
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.SetCanvasDraft(`{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"created_at"}]}],"edges":[]}`); err != nil {
		t.Fatalf("set canvas draft: %v", err)
	}
	if err := model.HandleShortcut("Ctrl+R"); err != nil {
		t.Fatalf("revert shortcut: %v", err)
	}
	projection := model.Projection()
	if projection.CanvasDirty {
		t.Fatal("expected canvas dirty cleared after revert")
	}
}

func TestUIShellModelDeleteCanvasDispatchesCanvasDelete(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	runner := &fakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "deleted canvas sales",
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(nil, explorer)
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.DeleteCanvas("sales"); err != nil {
		t.Fatalf("delete canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, `\canvas delete sales`) {
		t.Fatalf("runner input = %q, want delete command", runner.result.Input)
	}
	if state.Projection().ActiveCanvas != "" {
		t.Fatalf("active canvas = %q, want empty", state.Projection().ActiveCanvas)
	}
}
