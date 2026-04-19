package shell

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/db"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

type presenterFakeRunner struct {
	calls  int
	result terminal.TerminalResult
	err    error
}

func (p *presenterFakeRunner) RunCommand(input string) (terminal.TerminalResult, error) {
	p.calls++
	if p.err != nil {
		return terminal.TerminalResult{}, p.err
	}
	p.result.Input = input
	return p.result, nil
}

func TestPresenterProjectsStateAfterSubmit(t *testing.T) {
	runner := &presenterFakeRunner{
		result: terminal.TerminalResult{
			Kind:     terminal.TerminalKindQuery,
			RowCount: 2,
		},
	}
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	var got appstate.ShellProjection
	presenter := NewShellPresenter(model, func(proj appstate.ShellProjection) {
		got = proj
	})
	if err := presenter.SubmitTerminalCommand("SELECT * FROM products"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got.LastKind != terminal.TerminalKindQuery {
		t.Fatalf("projection kind = %q, want %q", got.LastKind, terminal.TerminalKindQuery)
	}
	if got.RowCount != 2 {
		t.Fatalf("projection row count = %d, want 2", got.RowCount)
	}
}

func TestPresenterProjectsConsoleShortcut(t *testing.T) {
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))

	seen := 0
	presenter := NewShellPresenter(model, func(proj appstate.ShellProjection) {
		if proj.ConsoleVisible {
			seen++
		}
	})
	if err := presenter.HandleShortcut("F12"); err != nil {
		t.Fatalf("handle shortcut: %v", err)
	}
	if seen != 1 {
		t.Fatalf("seen = %d, want 1", seen)
	}
}

func TestPresenterSurfacesTerminalErrorInProjection(t *testing.T) {
	fail := errors.New("sql bad")
	runner := &presenterFakeRunner{err: fail}
	state := appstate.NewShellState(nil)
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))
	presenter := NewShellPresenter(model, nil)

	err := presenter.SubmitTerminalCommand("bad")
	if err != fail {
		t.Fatalf("expected runner failure %v, got %v", fail, err)
	}
	if projection := presenter.Projection(); projection.LastStatus != "idle" {
		t.Fatalf("projection status = %q, want idle", projection.LastStatus)
	}
}

func TestPresenterIntegratesRealTerminalServiceWithConsoleEvents(t *testing.T) {
	tmp := t.TempDir()
	databasePath := filepath.Join(tmp, "shell-ui.duckdb")
	conn, err := db.Bootstrap(databasePath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer conn.Close()

	_, err = conn.SQL.Exec(`CREATE TABLE t(id BIGINT, name TEXT);`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = conn.SQL.Exec(`INSERT INTO t VALUES (1, 'alice'), (2, 'bob');`)
	if err != nil {
		t.Fatalf("seed data: %v", err)
	}

	console := terminal.NewEventConsole(10)
	service := terminal.NewTerminalServiceWithConsole(conn.SQL, console)
	state := appstate.NewShellState(console)
	model := NewShellModel(appstate.NewShellCommandBus(service, state))
	presenter := NewShellPresenter(model, nil)

	if err := presenter.SubmitTerminalCommand("SELECT name FROM t ORDER BY id"); err != nil {
		t.Fatalf("execute query: %v", err)
	}

	projection := presenter.Projection()
	if projection.LastKind != terminal.TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", projection.LastKind, terminal.TerminalKindQuery)
	}
	if projection.RowCount != 2 {
		t.Fatalf("row count = %d, want 2", projection.RowCount)
	}
	if projection.ConsoleItems == 0 {
		t.Fatal("expected console events to be recorded")
	}
}

func TestPresenterForwardsCanvasEdgeLifecycleAndUpdatesProjection(t *testing.T) {
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"name"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"},{"name":"total"}]}
			],"edges":[]}`,
		},
	})
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	presenter := NewShellPresenter(model, nil)

	if err := presenter.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := presenter.AddCanvasEdge(query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "id",
		ToColumn:   "customer_id",
		JoinType:   string(query.JoinLeft),
	}); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if !strings.Contains(presenter.Projection().CanvasSQLPreview, "LEFT JOIN") {
		t.Fatalf("preview = %q, want LEFT JOIN", presenter.Projection().CanvasSQLPreview)
	}

	if err := presenter.PatchCanvasEdge(query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "name",
		ToColumn:   "total",
		JoinType:   string(query.JoinInner),
	}); err != nil {
		t.Fatalf("patch edge: %v", err)
	}
	if !strings.Contains(presenter.Projection().CanvasSQLPreview, "\"customers\" \"customers\"") {
		t.Fatalf("preview = %q, want quoted customers table alias", presenter.Projection().CanvasSQLPreview)
	}

	if err := presenter.DeleteCanvasEdge("join-1"); err != nil {
		t.Fatalf("delete edge: %v", err)
	}
	if strings.Contains(presenter.Projection().CanvasSQLPreview, "JOIN") {
		t.Fatalf("preview after delete = %q, expected no JOIN", presenter.Projection().CanvasSQLPreview)
	}
}

func TestPresenterRunsActiveCanvasThroughModel(t *testing.T) {
	runner := &presenterFakeRunner{
		result: terminal.TerminalResult{
			Kind:     terminal.TerminalKindQuery,
			RowCount: 3,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"orders","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	presenter := NewShellPresenter(NewShellModel(appstate.NewShellCommandBus(runner, state)), nil)

	if err := presenter.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := presenter.RunActiveCanvas(); err != nil {
		t.Fatalf("run active canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if presenter.Projection().LastKind != terminal.TerminalKindQuery {
		t.Fatalf("last kind = %q, want %q", presenter.Projection().LastKind, terminal.TerminalKindQuery)
	}
}

func TestPresenterSurfacesCanvasRunParametersAndDuration(t *testing.T) {
	runner := &presenterFakeRunner{
		result: terminal.TerminalResult{
			Kind:                 terminal.TerminalKindQuery,
			SQLText:              `SELECT "c"."id" FROM "orders" "c" WHERE "c"."status" = ?`,
			Parameters:           []any{"open"},
			RowCount:             3,
			DurationMilliseconds: 16,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"orders","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	presenter := NewShellPresenter(NewShellModel(appstate.NewShellCommandBus(runner, state)), nil)

	if err := presenter.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := presenter.RunActiveCanvas(); err != nil {
		t.Fatalf("run active canvas: %v", err)
	}
	if !strings.Contains(presenter.Projection().OutputText, "parameters: open") {
		t.Fatalf("output = %q", presenter.Projection().OutputText)
	}
	if !strings.Contains(presenter.Projection().OutputText, "duration: 16ms") {
		t.Fatalf("output = %q", presenter.Projection().OutputText)
	}
	if len(presenter.Projection().ResultParameters) != 1 || presenter.Projection().ResultParameters[0] != "open" {
		t.Fatalf("result parameters = %#v", presenter.Projection().ResultParameters)
	}
}

func TestPresenterForwardsCanvasCreationToModel(t *testing.T) {
	runner := &presenterFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "created canvas project-overview",
		},
	}
	state := appstate.NewShellState(nil)
	presenter := NewShellPresenter(NewShellModel(appstate.NewShellCommandBus(runner, state)), nil)

	if err := presenter.CreateCanvas("project-overview"); err != nil {
		t.Fatalf("create canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, `\canvas new project-overview`) {
		t.Fatalf("runner input = %q, want prefix \\canvas new project-overview", runner.result.Input)
	}
}

func TestPresenterForwardsCanvasRenameToModelAndRefreshesProjection(t *testing.T) {
	runner := &presenterFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "renamed sales to sales-archive",
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(nil, &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	presenter := NewShellPresenter(NewShellModel(appstate.NewShellCommandBus(runner, state)), nil)

	if err := presenter.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := presenter.RenameCanvas("sales", "sales-archive"); err != nil {
		t.Fatalf("rename canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if presenter.Projection().ActiveCanvas != "sales-archive" {
		t.Fatalf("active canvas = %q, want sales-archive", presenter.Projection().ActiveCanvas)
	}
	if !strings.Contains(runner.result.Input, `\canvas rename sales sales-archive`) {
		t.Fatalf("runner input = %q, want prefix \\canvas rename sales sales-archive", runner.result.Input)
	}
}

func TestPresenterForwardsCanvasDeleteToModel(t *testing.T) {
	runner := &presenterFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "deleted canvas sales",
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(nil, &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	presenter := NewShellPresenter(NewShellModel(appstate.NewShellCommandBus(runner, state)), nil)

	if err := presenter.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := presenter.DeleteCanvas("sales"); err != nil {
		t.Fatalf("delete canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if presenter.Projection().ActiveCanvas != "" {
		t.Fatalf("active canvas = %q, want empty", presenter.Projection().ActiveCanvas)
	}
	if !strings.Contains(runner.result.Input, `\canvas delete sales`) {
		t.Fatalf("runner input = %q, want prefix \\canvas delete sales", runner.result.Input)
	}
}
