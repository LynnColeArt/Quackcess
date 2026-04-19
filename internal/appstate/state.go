package appstate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

const (
	ActionToggleConsole    = "toggle_console"
	ActionRunTerminal      = "run_terminal"
	ActionRunVectorize      = "run_vectorize"
	ActionShortcut         = "shortcut"
	ActionSetConsoleState  = "set_console_state"
	ActionSetCanvas        = "set_canvas"
	ActionSetCanvasDraft   = "set_canvas_draft"
	ActionRunCanvas        = "run_canvas"
	ActionCanvasNew        = "canvas_new"
	ActionCanvasRename     = "canvas_rename"
	ActionCanvasDelete     = "canvas_delete"
	ActionMoveCanvasNode   = "move_canvas_node"
	ActionSetNodeFields    = "set_canvas_node_fields"
	ActionAddCanvasNode    = "add_canvas_node"
	ActionAddCanvasEdge    = "add_canvas_edge"
	ActionPatchCanvasEdge  = "patch_canvas_edge"
	ActionDeleteCanvasEdge = "delete_canvas_edge"
	ActionRevertCanvas     = "revert_canvas"
	ActionSaveCanvas       = "save_canvas"
	ActionClearCanvas      = "clear_canvas"

	maxProjectionRows     = 10
	canvasPreviewLimit    = 200
	canvasRunDefaultLimit = 200
)

type Action struct {
	Kind    string
	Payload string
}

type TerminalRunner interface {
	RunCommand(input string) (terminal.TerminalResult, error)
}

type VectorizeArtifactWriter func(input string, metadata terminal.TerminalVectorizeMetadata) error

type CatalogExplorer interface {
	ListTables() ([]string, error)
	ListViews() ([]string, error)
	ListCanvases() ([]string, error)
	LoadCanvasSpec(name string) (string, error)
}

type ShellState struct {
	consoleVisible   bool
	lastInput        string
	lastResult       terminal.TerminalResult
	console          *terminal.EventConsole
	catalog          CatalogExplorer
	activeCanvas     string
	canvasDraftSpec  string
	canvasSavedSpec  string
	canvasValidation string
	canvasStatus     string
	canvasDirty      bool
}

type ShellProjection struct {
	ConsoleVisible   bool
	LastInput        string
	LastStatus       string
	LastKind         string
	RowCount         int
	ErrorText        string
	ConsoleItems     int
	SQLText          string
	OutputText       string
	ResultColumns    []string
	ResultRows       []string
	ResultParameters []string
	ResultEstimate   string
	CatalogTables    []string
	CatalogViews     []string
	CatalogCanvases  []string
	ActiveCanvas     string
	CanvasDraftSpec  string
	CanvasValidation string
	CanvasStatus     string
	CanvasDirty      bool
	CanvasSQLPreview string
}

func NewShellState(console *terminal.EventConsole) *ShellState {
	return NewShellStateWithCatalogExplorer(console, nil)
}

func NewShellStateWithCatalogExplorer(console *terminal.EventConsole, catalog CatalogExplorer) *ShellState {
	state := &ShellState{
		console: console,
		catalog: catalog,
	}
	if console != nil {
		state.consoleVisible = console.IsVisible()
	}
	return state
}

func (s *ShellState) IsConsoleVisible() bool {
	if s == nil {
		return false
	}
	return s.consoleVisible
}

func (s *ShellState) SetConsoleVisible(visible bool) {
	if s == nil {
		return
	}
	s.consoleVisible = visible
	if s.console != nil {
		s.console.SetVisible(visible)
	}
}

func (s *ShellState) ToggleConsole() bool {
	if s == nil {
		return false
	}
	s.consoleVisible = !s.consoleVisible
	if s.console != nil {
		s.console.SetVisible(s.consoleVisible)
	}
	return s.consoleVisible
}

func (s *ShellState) LastResult() terminal.TerminalResult {
	if s == nil {
		return terminal.TerminalResult{}
	}
	return s.lastResult
}

func (s *ShellState) LastInput() string {
	if s == nil {
		return ""
	}
	return s.lastInput
}

func (s *ShellState) ConsoleEvents() []terminal.ConsoleEvent {
	if s == nil || s.console == nil {
		return nil
	}
	return s.console.Items()
}

func (s *ShellState) recordTerminalResult(input string, result terminal.TerminalResult) {
	if s == nil {
		return
	}
	s.lastInput = input
	s.lastResult = result
}

func (s *ShellState) SetActiveCanvas(name string) error {
	if s == nil {
		return fmt.Errorf("shell state is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		s.clearActiveCanvas()
		s.canvasStatus = "no canvas selected"
		return nil
	}

	spec, err := s.catalogCanvasSpec(name)
	if err != nil {
		s.clearActiveCanvas()
		s.canvasStatus = "failed to load canvas: " + err.Error()
		s.activeCanvas = name
		s.canvasValidation = err.Error()
		return err
	}
	s.activeCanvas = name
	normalizedSpec, err := normalizeCanvasSpecText(spec)
	if err != nil {
		s.clearActiveCanvas()
		s.canvasStatus = "failed to normalize canvas: " + err.Error()
		return err
	}
	s.canvasSavedSpec = normalizedSpec
	s.canvasDraftSpec = normalizedSpec
	s.canvasDirty = false
	s.canvasValidation = validateCanvasSpec(s.canvasDraftSpec)
	s.canvasStatus = "canvas loaded"
	return nil
}

func (s *ShellState) SetCanvasDraft(spec string) error {
	if s == nil {
		return fmt.Errorf("shell state is nil")
	}
	if s.activeCanvas == "" {
		return fmt.Errorf("no active canvas")
	}
	normalized, err := normalizeCanvasSpecText(spec)
	if err != nil {
		return err
	}
	s.canvasDraftSpec = normalized
	s.canvasDirty = s.canvasDraftSpec != s.canvasSavedSpec
	s.canvasValidation = validateCanvasSpec(s.canvasDraftSpec)
	s.canvasStatus = "canvas draft updated"
	return nil
}

func (s *ShellState) applyCanvasDraftMutation(mutator func(query.CanvasSpec) (query.CanvasSpec, error)) error {
	if s == nil {
		return fmt.Errorf("shell state is nil")
	}
	if s.activeCanvas == "" {
		return fmt.Errorf("no active canvas")
	}
	if strings.TrimSpace(s.canvasDraftSpec) == "" {
		return fmt.Errorf("canvas draft is required")
	}
	draft, err := query.ParseCanvasSpec([]byte(s.canvasDraftSpec))
	if err != nil {
		return err
	}
	spec, err := mutator(draft)
	if err != nil {
		return err
	}
	normalizedSpec, err := query.MarshalCanvasSpec(spec)
	if err != nil {
		return err
	}
	s.canvasDraftSpec = string(normalizedSpec)
	s.canvasDirty = s.canvasDraftSpec != s.canvasSavedSpec
	s.canvasValidation = validateCanvasSpec(s.canvasDraftSpec)
	s.canvasStatus = "canvas draft updated"
	return nil
}

// MoveActiveCanvasNode moves a node in the active canvas draft by id.
func (s *ShellState) MoveActiveCanvasNode(nodeID string, x, y float64) error {
	return s.applyCanvasDraftMutation(func(spec query.CanvasSpec) (query.CanvasSpec, error) {
		return query.MoveCanvasNode(spec, nodeID, x, y)
	})
}

func (s *ShellState) AddCanvasNodeToActiveCanvas(payload query.CanvasNode) error {
	return s.applyCanvasDraftMutation(func(spec query.CanvasSpec) (query.CanvasSpec, error) {
		return query.AddCanvasNode(spec, payload)
	})
}

// SetActiveCanvasNodeFields updates selected fields for a node in the active canvas draft.
func (s *ShellState) SetActiveCanvasNodeFields(nodeID string, fields []string) error {
	return s.applyCanvasDraftMutation(func(spec query.CanvasSpec) (query.CanvasSpec, error) {
		return query.SetCanvasNodeSelectedFields(spec, nodeID, fields)
	})
}

// AddCanvasEdgeToActiveCanvas adds an edge definition to the active canvas draft.
func (s *ShellState) AddCanvasEdgeToActiveCanvas(payload query.CanvasEdge) error {
	return s.applyCanvasDraftMutation(func(spec query.CanvasSpec) (query.CanvasSpec, error) {
		return query.AddCanvasEdge(spec, payload)
	})
}

// PatchActiveCanvasEdge updates an existing edge in the active canvas draft.
func (s *ShellState) PatchActiveCanvasEdge(payload query.CanvasEdge) error {
	return s.applyCanvasDraftMutation(func(spec query.CanvasSpec) (query.CanvasSpec, error) {
		return query.PatchCanvasEdge(spec, payload)
	})
}

// DeleteActiveCanvasEdge removes an edge from the active canvas draft.
func (s *ShellState) DeleteActiveCanvasEdge(edgeID string) error {
	return s.applyCanvasDraftMutation(func(spec query.CanvasSpec) (query.CanvasSpec, error) {
		return query.DeleteCanvasEdge(spec, edgeID)
	})
}

func (s *ShellState) RevertCanvasDraft() error {
	if s == nil {
		return fmt.Errorf("shell state is nil")
	}
	if s.activeCanvas == "" {
		return fmt.Errorf("no active canvas")
	}
	s.canvasDraftSpec = s.canvasSavedSpec
	s.canvasDirty = false
	s.canvasValidation = validateCanvasSpec(s.canvasDraftSpec)
	s.canvasStatus = "canvas draft reverted"
	return nil
}

func (s *ShellState) CommitCanvasDraft() {
	if s == nil {
		return
	}
	s.canvasSavedSpec = s.canvasDraftSpec
	s.canvasDirty = false
	s.canvasStatus = "canvas saved"
}

func (s *ShellState) ClearActiveCanvas() {
	if s == nil {
		return
	}
	s.clearActiveCanvas()
	s.canvasStatus = "no canvas selected"
}

func (s *ShellState) clearActiveCanvas() {
	s.activeCanvas = ""
	s.canvasDraftSpec = ""
	s.canvasSavedSpec = ""
	s.canvasValidation = ""
	s.canvasDirty = false
	s.canvasStatus = "no canvas selected"
}

func validateCanvasSpec(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "canvas spec is required"
	}
	if _, err := query.ParseCanvasSpec([]byte(raw)); err != nil {
		return err.Error()
	}
	return ""
}

func (s *ShellState) catalogCanvasSpec(name string) (string, error) {
	if s.catalog == nil {
		return "", fmt.Errorf("catalog explorer is not configured")
	}
	return s.catalog.LoadCanvasSpec(strings.TrimSpace(name))
}

func (s *ShellState) Projection() ShellProjection {
	if s == nil {
		return ShellProjection{
			LastStatus: "idle",
		}
	}

	projection := ShellProjection{
		ConsoleVisible:   s.consoleVisible,
		LastInput:        s.lastInput,
		ActiveCanvas:     s.activeCanvas,
		CanvasDraftSpec:  s.canvasDraftSpec,
		CanvasValidation: s.canvasValidation,
		CanvasStatus:     s.canvasStatus,
		CanvasDirty:      s.canvasDirty,
		CanvasSQLPreview: shellWindowCanvasSQLPreview(s.canvasDraftSpec),
	}
	projection.LastKind = s.lastResult.Kind
	projection.RowCount = s.lastResult.RowCount
	projection.ErrorText = s.lastResult.ErrorText
	projection.SQLText = s.lastResult.SQLText
	projection.ResultParameters = formatProjectionParameters(s.lastResult.Parameters)
	projection.ResultEstimate = formatProjectionEstimate(s.lastResult.RowCount, s.lastResult.DurationMilliseconds)
	projection.OutputText = formatProjectionOutput(s.lastResult)
	projection.ResultColumns = append([]string(nil), s.lastResult.Columns...)
	projection.ResultRows = projectionRows(s.lastResult.Rows)
	projection.ConsoleItems = len(s.console.Items())
	projection.CatalogTables = projectionCatalogItems(s.catalogListTables())
	projection.CatalogViews = projectionCatalogItems(s.catalogListViews())
	projection.CatalogCanvases = projectionCatalogItems(s.catalogListCanvases())
	if s.lastResult.ErrorText != "" {
		projection.LastStatus = s.lastResult.ErrorText
		return projection
	}
	switch s.lastResult.Kind {
	case terminal.TerminalKindQuery:
		projection.LastStatus = "query executed"
	case terminal.TerminalKindHistory:
		projection.LastStatus = "history shown"
	case terminal.TerminalKindHelp:
		projection.LastStatus = "ready"
	case "":
		projection.LastStatus = "idle"
	default:
		projection.LastStatus = "ready"
	}
	if projection.CanvasStatus == "" {
		projection.CanvasStatus = projection.LastStatus
	}
	return projection
}

func shellWindowCanvasSQLPreview(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	document, err := query.ParseCanvasSpec([]byte(spec))
	if err != nil {
		return ""
	}
	canvasSQL, err := query.GenerateSQLFromCanvasWithLimit(document, canvasPreviewLimit)
	if err != nil {
		return ""
	}
	return canvasSQL.SQL
}

func projectionRows(rows [][]any) []string {
	if len(rows) == 0 {
		return nil
	}
	limit := len(rows)
	if limit > maxProjectionRows {
		limit = maxProjectionRows
	}

	projected := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		projected = append(projected, projectionRow(rows[i]))
	}
	if len(rows) > maxProjectionRows {
		projected = append(projected, "... truncated")
	}
	return projected
}

func projectionRow(row []any) string {
	if len(row) == 0 {
		return ""
	}
	parts := make([]string, len(row))
	for i, value := range row {
		switch v := value.(type) {
		case nil:
			parts[i] = "NULL"
		case []byte:
			parts[i] = string(v)
		default:
			parts[i] = fmt.Sprint(v)
		}
	}
	return strings.Join(parts, "\t")
}

func formatProjectionOutput(result terminal.TerminalResult) string {
	switch result.Kind {
	case terminal.TerminalKindQuery:
		return formatProjectionQueryOutput(result)
	case terminal.TerminalKindHistory:
		if result.Message != "" {
			return result.Message
		}
		return "history shown"
	case terminal.TerminalKindHelp:
		return result.Message
	case terminal.TerminalKindError:
		if result.ErrorText != "" {
			return "error: " + result.ErrorText
		}
		return "unknown error"
	default:
		return result.SQLText
	}
}

func formatProjectionQueryOutput(result terminal.TerminalResult) string {
	var lines []string
	if result.SQLText != "" {
		lines = append(lines, "sql: "+result.SQLText)
	}
	if len(result.Parameters) > 0 {
		lines = append(lines, "parameters: "+strings.Join(formatProjectionParameters(result.Parameters), ", "))
	}
	if len(result.Columns) > 0 {
		lines = append(lines, "columns: "+strings.Join(result.Columns, ", "))
	}
	if result.DurationMilliseconds >= 0 {
		lines = append(lines, "duration: "+fmt.Sprintf("%dms", result.DurationMilliseconds))
	}
	lines = append(lines, "rows: "+fmt.Sprintf("%d", result.RowCount))
	for _, row := range projectionRows(result.Rows) {
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

func formatProjectionParameters(parameters []any) []string {
	if len(parameters) == 0 {
		return nil
	}
	out := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		out = append(out, fmt.Sprint(parameter))
	}
	return out
}

func formatProjectionEstimate(rowCount int, durationMilliseconds int64) string {
	switch {
	case durationMilliseconds >= 0:
		return fmt.Sprintf("estimate: rows=%d, duration=%dms", rowCount, durationMilliseconds)
	default:
		return fmt.Sprintf("estimate: rows=%d", rowCount)
	}
}

func projectionCatalogItems(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	result := make([]string, len(items))
	copy(result, items)
	return result
}

func (s *ShellState) catalogListTables() []string {
	if s == nil || s.catalog == nil {
		return nil
	}
	items, err := s.catalog.ListTables()
	if err != nil {
		return nil
	}
	return items
}

func (s *ShellState) catalogListViews() []string {
	if s == nil || s.catalog == nil {
		return nil
	}
	items, err := s.catalog.ListViews()
	if err != nil {
		return nil
	}
	return items
}

func (s *ShellState) catalogListCanvases() []string {
	if s == nil || s.catalog == nil {
		return nil
	}
	items, err := s.catalog.ListCanvases()
	if err != nil {
		return nil
	}
	return items
}

type ShellCommandBus struct {
	state   *ShellState
	runner  TerminalRunner
	console *terminal.EventConsole
	writer  VectorizeArtifactWriter
}

func NewShellCommandBus(runner TerminalRunner, state *ShellState) *ShellCommandBus {
	return NewShellCommandBusWithVectorWriter(runner, state, nil)
}

func NewShellCommandBusWithVectorWriter(runner TerminalRunner, state *ShellState, writer VectorizeArtifactWriter) *ShellCommandBus {
	if state == nil {
		state = &ShellState{}
	}
	return &ShellCommandBus{
		state:   state,
		runner:  runner,
		console: state.console,
		writer:  writer,
	}
}

func (b *ShellCommandBus) State() *ShellState {
	return b.state
}

func (b *ShellCommandBus) Dispatch(action Action) error {
	if b.state == nil {
		return fmt.Errorf("command bus has no state")
	}
	switch action.Kind {
	case ActionToggleConsole:
		b.state.ToggleConsole()
	case ActionSetConsoleState:
		b.state.SetConsoleVisible(parseBoolLike(action.Payload))
	case ActionRunTerminal:
		if b.runner == nil {
			return fmt.Errorf("command bus has no terminal runner")
		}
		result, err := b.runner.RunCommand(action.Payload)
		if err != nil {
			return err
		}
		b.state.recordTerminalResult(action.Payload, result)
		if err := b.applyVectorizeTerminalSideEffects(action.Payload, result); err != nil {
			return err
		}
		b.applyCanvasTerminalSideEffects(action.Payload, result)
	case ActionRunVectorize:
		if b.runner == nil {
			return fmt.Errorf("command bus has no terminal runner")
		}
		result, err := b.runner.RunCommand(action.Payload)
		if err != nil {
			return err
		}
		b.state.recordTerminalResult(action.Payload, result)
		if err := b.applyVectorizeTerminalSideEffects(action.Payload, result); err != nil {
			return err
		}
		b.applyCanvasTerminalSideEffects(action.Payload, result)
	case ActionSetCanvas:
		if err := b.state.SetActiveCanvas(action.Payload); err != nil {
			return err
		}
	case ActionSetCanvasDraft:
		if err := b.state.SetCanvasDraft(action.Payload); err != nil {
			return err
		}
	case ActionMoveCanvasNode:
		payload, err := parseCanvasMoveNodePayload(action.Payload)
		if err != nil {
			return err
		}
		if err := b.state.MoveActiveCanvasNode(payload.nodeID, payload.x, payload.y); err != nil {
			return err
		}
	case ActionAddCanvasNode:
		payload, err := parseCanvasNodePayload(action.Payload)
		if err != nil {
			return err
		}
		if err := b.state.AddCanvasNodeToActiveCanvas(payload); err != nil {
			return err
		}
	case ActionSetNodeFields:
		payload, err := parseCanvasNodeFieldsPayload(action.Payload)
		if err != nil {
			return err
		}
		if err := b.state.SetActiveCanvasNodeFields(payload.nodeID, payload.fields); err != nil {
			return err
		}
	case ActionRunCanvas:
		if b.runner == nil {
			return fmt.Errorf("command bus has no terminal runner")
		}
		sql, err := activeCanvasSQL(b.state)
		if err != nil {
			return err
		}
		result, err := b.runner.RunCommand(sql)
		if err != nil {
			return err
		}
		b.state.recordTerminalResult(sql, result)
		b.state.canvasStatus = mapRunCanvasResultToStatus(b.state.activeCanvas, b.state.canvasDraftSpec, result)
	case ActionCanvasNew:
		payload, err := parseCanvasNamePayload(action.Payload)
		if err != nil {
			return err
		}
		if b.runner == nil {
			return fmt.Errorf("command bus has no terminal runner")
		}
		command := "\\canvas new " + payload
		result, err := b.runner.RunCommand(command)
		if err != nil {
			return err
		}
		b.state.recordTerminalResult(command, result)
		b.applyCanvasTerminalSideEffects(command, result)
	case ActionCanvasRename:
		oldName, newName, err := parseCanvasRenamePayload(action.Payload)
		if err != nil {
			return err
		}
		if b.runner == nil {
			return fmt.Errorf("command bus has no terminal runner")
		}
		command := "\\canvas rename " + oldName + " " + newName
		result, err := b.runner.RunCommand(command)
		if err != nil {
			return err
		}
		b.state.recordTerminalResult(command, result)
		b.applyCanvasTerminalSideEffects(command, result)
	case ActionCanvasDelete:
		name, err := parseCanvasNamePayload(action.Payload)
		if err != nil {
			return err
		}
		if b.runner == nil {
			return fmt.Errorf("command bus has no terminal runner")
		}
		command := "\\canvas delete " + name
		result, err := b.runner.RunCommand(command)
		if err != nil {
			return err
		}
		b.state.recordTerminalResult(command, result)
		b.applyCanvasTerminalSideEffects(command, result)
	case ActionAddCanvasEdge:
		payload, err := parseCanvasEdgePayload(action.Payload)
		if err != nil {
			return err
		}
		if err := b.state.AddCanvasEdgeToActiveCanvas(payload); err != nil {
			return err
		}
	case ActionPatchCanvasEdge:
		payload, err := parseCanvasEdgePayload(action.Payload)
		if err != nil {
			return err
		}
		if err := b.state.PatchActiveCanvasEdge(payload); err != nil {
			return err
		}
	case ActionDeleteCanvasEdge:
		payload, err := parseCanvasDeleteEdgePayload(action.Payload)
		if err != nil {
			return err
		}
		if err := b.state.DeleteActiveCanvasEdge(payload.edgeID); err != nil {
			return err
		}
	case ActionRevertCanvas:
		if err := b.state.RevertCanvasDraft(); err != nil {
			return err
		}
	case ActionSaveCanvas:
		if b.runner == nil {
			return fmt.Errorf("command bus has no terminal runner")
		}
		command := buildCanvasSaveCommand(b.state.activeCanvas, b.state.canvasDraftSpec)
		if command == "" {
			return fmt.Errorf("no active canvas to save")
		}
		if err := runCanvasValidation(b.state); err != nil {
			return err
		}
		result, err := b.runner.RunCommand(command)
		if err != nil {
			return err
		}
		b.state.recordTerminalResult(command, result)
		if result.Kind != terminal.TerminalKindError {
			b.state.CommitCanvasDraft()
		} else if result.ErrorText != "" {
			b.state.canvasStatus = "save failed: " + result.ErrorText
		}
	case ActionClearCanvas:
		b.state.ClearActiveCanvas()
	case ActionShortcut:
		if b.console == nil {
			if strings.EqualFold(action.Payload, "Escape") {
				b.state.SetConsoleVisible(false)
			}
			return nil
		}
		if strings.EqualFold(action.Payload, "Escape") {
			b.state.SetConsoleVisible(false)
			return nil
		}
		if !b.console.HandleShortcut(action.Payload) {
			return fmt.Errorf("unhandled shortcut: %s", action.Payload)
		}
		b.state.consoleVisible = b.console.IsVisible()
	default:
		return fmt.Errorf("unsupported action: %s", action.Kind)
	}
	return nil
}

func (b *ShellCommandBus) applyCanvasTerminalSideEffects(input string, result terminal.TerminalResult) {
	if b == nil || b.state == nil {
		return
	}
	if result.Kind == terminal.TerminalKindError {
		return
	}
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(strings.ToLower(trimmed), "\\canvas") {
		return
	}
	normalized := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(normalized, "\\canvas save "):
		name, spec, ok := parseCanvasSaveCommand(trimmed)
		if !ok {
			return
		}
		if b.state.activeCanvas != name {
			return
		}
		normalizedSpec, err := normalizeCanvasSpecText(spec)
		if err != nil {
			b.state.canvasStatus = "failed to normalize canvas save spec: " + err.Error()
			return
		}
		b.state.canvasSavedSpec = normalizedSpec
		b.state.canvasDraftSpec = normalizedSpec
		b.state.canvasValidation = validateCanvasSpec(b.state.canvasDraftSpec)
		b.state.canvasDirty = false
		b.state.canvasStatus = "canvas saved"
	case strings.HasPrefix(normalized, "\\canvas new "):
		name, ok := parseCanvasNewCommand(trimmed)
		if !ok {
			return
		}
		b.state.canvasStatus = "canvas created: " + name
	case strings.HasPrefix(normalized, "\\canvas rename "):
		oldName, newName, ok := parseCanvasRenameCommand(trimmed)
		if !ok {
			return
		}
		if b.state.activeCanvas == oldName {
			b.state.activeCanvas = newName
			b.state.canvasStatus = "canvas renamed"
		}
	case strings.HasPrefix(normalized, "\\canvas delete "):
		name, ok := parseCanvasSimpleCommand(trimmed)
		if !ok {
			return
		}
		if b.state.activeCanvas == name {
			b.state.ClearActiveCanvas()
		}
	}
}

func (b *ShellCommandBus) applyVectorizeTerminalSideEffects(input string, result terminal.TerminalResult) error {
	if b == nil || b.writer == nil {
		return nil
	}
	if result.Vectorize == nil {
		return nil
	}
	return b.writer(input, *result.Vectorize)
}

func mapRunCanvasResultToStatus(activeCanvas string, draftSpec string, result terminal.TerminalResult) string {
	if result.Kind != terminal.TerminalKindError {
		if activeCanvas == "" {
			return "canvas executed"
		}
		if result.DurationMilliseconds > 0 {
			return fmt.Sprintf("canvas %s executed (%d rows, %dms)", activeCanvas, result.RowCount, result.DurationMilliseconds)
		}
		return fmt.Sprintf("canvas %s executed (%d rows)", activeCanvas, result.RowCount)
	}
	if nodeID, ok := mapCanvasErrorToNodeID(draftSpec, result.ErrorText); ok {
		return "canvas execution failed at node " + nodeID + ": " + result.ErrorText
	}
	return "canvas execution failed: " + result.ErrorText
}

var quotedIdentifierPattern = regexp.MustCompile(`"([^"]+)"`)

func mapCanvasErrorToNodeID(spec string, errorText string) (string, bool) {
	specText := strings.TrimSpace(spec)
	if specText == "" || strings.TrimSpace(errorText) == "" {
		return "", false
	}
	specModel, err := query.ParseCanvasSpec([]byte(specText))
	if err != nil {
		return "", false
	}
	aliasByLower := make(map[string]string)
	tableByLower := make(map[string]string)
	for _, node := range specModel.Nodes {
		nodeID := strings.TrimSpace(node.ID)
		if nodeID == "" {
			continue
		}
		if strings.TrimSpace(node.Alias) != "" {
			aliasByLower[strings.ToLower(strings.TrimSpace(node.Alias))] = nodeID
		}
		if strings.TrimSpace(node.Table) != "" {
			tableByLower[strings.ToLower(strings.TrimSpace(node.Table))] = nodeID
		}
	}

	errText := strings.ToLower(errorText)
	for _, match := range quotedIdentifierPattern.FindAllString(errText, -1) {
		token := strings.Trim(strings.ToLower(strings.TrimSpace(strings.Trim(match, `"`))), `"`)
		if token == "" {
			continue
		}
		if nodeID, ok := aliasByLower[token]; ok {
			return nodeID, true
		}
		if nodeID, ok := tableByLower[token]; ok {
			return nodeID, true
		}
		for _, part := range strings.Split(token, ".") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if nodeID, ok := aliasByLower[part]; ok {
				return nodeID, true
			}
			if nodeID, ok := tableByLower[part]; ok {
				return nodeID, true
			}
		}
	}
	for _, token := range tokenizeCanvasError(errText) {
		token = strings.Trim(strings.ToLower(strings.TrimSpace(token)), `"`)
		if token == "" {
			continue
		}
		if nodeID, ok := aliasByLower[token]; ok {
			return nodeID, true
		}
		if nodeID, ok := tableByLower[token]; ok {
			return nodeID, true
		}
		for _, part := range strings.Split(token, ".") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if nodeID, ok := aliasByLower[part]; ok {
				return nodeID, true
			}
			if nodeID, ok := tableByLower[part]; ok {
				return nodeID, true
			}
		}
	}
	return "", false
}

func tokenizeCanvasError(errorText string) []string {
	raw := strings.FieldsFunc(errorText, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' && r != '.'
	})
	candidates := make([]string, 0, len(raw))
	for _, candidate := range raw {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func parseCanvasSaveCommand(input string) (string, string, bool) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "\\canvas save"))
	if raw == "" {
		return "", "", false
	}
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	name := strings.TrimSpace(parts[0])
	spec := strings.TrimSpace(parts[1])
	if name == "" || spec == "" {
		return "", "", false
	}
	if len(spec) >= 2 && ((spec[0] == '\'' && spec[len(spec)-1] == '\'') || (spec[0] == '"' && spec[len(spec)-1] == '"')) {
		spec = strings.Trim(spec, string(spec[0]))
	}
	return name, spec, true
}

type canvasMoveNodePayload struct {
	nodeID string
	x      float64
	y      float64
}

type canvasNodeFieldsPayload struct {
	nodeID string
	fields []string
}

type canvasEdgeIDPayload struct {
	edgeID string
}

func parseCanvasMoveNodePayload(raw string) (canvasMoveNodePayload, error) {
	type payload struct {
		NodeID string  `json:"node_id"`
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
	}
	var input payload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &input); err != nil {
		return canvasMoveNodePayload{}, fmt.Errorf("invalid move node payload: %w", err)
	}
	if strings.TrimSpace(input.NodeID) == "" {
		return canvasMoveNodePayload{}, fmt.Errorf("move node payload requires node_id")
	}
	return canvasMoveNodePayload{
		nodeID: strings.TrimSpace(input.NodeID),
		x:      input.X,
		y:      input.Y,
	}, nil
}

func parseCanvasNodeFieldsPayload(raw string) (canvasNodeFieldsPayload, error) {
	type payload struct {
		NodeID string   `json:"node_id"`
		Fields []string `json:"fields"`
	}
	var input payload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &input); err != nil {
		return canvasNodeFieldsPayload{}, fmt.Errorf("invalid node fields payload: %w", err)
	}
	if strings.TrimSpace(input.NodeID) == "" {
		return canvasNodeFieldsPayload{}, fmt.Errorf("node fields payload requires node_id")
	}
	return canvasNodeFieldsPayload{
		nodeID: strings.TrimSpace(input.NodeID),
		fields: input.Fields,
	}, nil
}

func parseCanvasNodePayload(raw string) (query.CanvasNode, error) {
	type payload struct {
		ID             string              `json:"id"`
		Kind           string              `json:"kind"`
		Table          string              `json:"table"`
		Alias          string              `json:"alias"`
		X              float64             `json:"x"`
		Y              float64             `json:"y"`
		Width          float64             `json:"width"`
		Height         float64             `json:"height"`
		Fields         []query.CanvasField `json:"fields"`
		SelectedFields []string            `json:"selected_fields"`
	}
	var input payload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &input); err != nil {
		return query.CanvasNode{}, fmt.Errorf("invalid canvas node payload: %w", err)
	}
	nodeID := strings.TrimSpace(input.ID)
	if nodeID == "" {
		return query.CanvasNode{}, fmt.Errorf("canvas node payload requires id")
	}
	return query.CanvasNode{
		ID:             nodeID,
		Kind:           strings.TrimSpace(input.Kind),
		Table:          strings.TrimSpace(input.Table),
		Alias:          strings.TrimSpace(input.Alias),
		X:              input.X,
		Y:              input.Y,
		Width:          input.Width,
		Height:         input.Height,
		Fields:         input.Fields,
		SelectedFields: input.SelectedFields,
	}, nil
}

func parseCanvasEdgePayload(raw string) (query.CanvasEdge, error) {
	var edge query.CanvasEdge
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &edge); err != nil {
		return query.CanvasEdge{}, fmt.Errorf("invalid canvas edge payload: %w", err)
	}
	edge.ID = strings.TrimSpace(edge.ID)
	if edge.ID == "" {
		return query.CanvasEdge{}, fmt.Errorf("canvas edge payload requires id")
	}
	edge.FromNode = strings.TrimSpace(edge.FromNode)
	edge.ToNode = strings.TrimSpace(edge.ToNode)
	edge.FromColumn = strings.TrimSpace(edge.FromColumn)
	edge.ToColumn = strings.TrimSpace(edge.ToColumn)
	edge.JoinType = strings.TrimSpace(edge.JoinType)
	return edge, nil
}

func parseCanvasDeleteEdgePayload(raw string) (canvasEdgeIDPayload, error) {
	type payload struct {
		EdgeID string `json:"edge_id"`
	}
	var input payload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &input); err == nil && strings.TrimSpace(input.EdgeID) != "" {
		return canvasEdgeIDPayload{edgeID: strings.TrimSpace(input.EdgeID)}, nil
	}
	type legacy struct {
		ID string `json:"id"`
	}
	var alt legacy
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &alt); err == nil && strings.TrimSpace(alt.ID) != "" {
		return canvasEdgeIDPayload{edgeID: strings.TrimSpace(alt.ID)}, nil
	}
	return canvasEdgeIDPayload{}, fmt.Errorf("canvas edge id payload requires edge_id")
}

func normalizeCanvasSpecText(raw string) (string, error) {
	return canonicalCanvasSpecText(raw)
}

func canonicalCanvasSpecText(raw string) (string, error) {
	spec, err := query.ParseCanvasSpec([]byte(strings.TrimSpace(raw)))
	if err != nil {
		return "", err
	}
	rawSpec, err := query.MarshalCanvasSpec(spec)
	if err != nil {
		return "", err
	}
	return string(rawSpec), nil
}

func parseCanvasRenameCommand(input string) (string, string, bool) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "\\canvas rename"))
	parts := strings.Fields(raw)
	if len(parts) != 2 {
		return "", "", false
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseCanvasNewCommand(input string) (string, bool) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "\\canvas new"))
	if raw == "" {
		return "", false
	}
	parts := strings.Fields(raw)
	if len(parts) != 1 {
		return "", false
	}
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", false
	}
	return name, true
}

func parseCanvasSimpleCommand(input string) (string, bool) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "\\canvas"))
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "delete"))
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	fields := strings.Fields(raw)
	if len(fields) != 1 {
		return "", false
	}
	return fields[0], true
}

func activeCanvasSQL(state *ShellState) (string, error) {
	if state == nil {
		return "", fmt.Errorf("shell state is nil")
	}
	if strings.TrimSpace(state.activeCanvas) == "" {
		return "", fmt.Errorf("no active canvas")
	}
	if state.canvasValidation != "" {
		return "", fmt.Errorf("canvas validation failed: %s", state.canvasValidation)
	}
	spec, err := query.ParseCanvasSpec([]byte(state.canvasDraftSpec))
	if err != nil {
		return "", err
	}
	sqlText, err := query.GenerateSQLFromCanvasWithLimit(spec, canvasRunDefaultLimit)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(sqlText.SQL) == "" {
		return "", fmt.Errorf("no canvas SQL to run")
	}
	return strings.TrimSpace(sqlText.SQL), nil
}

func parseBoolLike(input string) bool {
	return strings.EqualFold(strings.TrimSpace(input), "true") || strings.EqualFold(strings.TrimSpace(input), "on") || strings.EqualFold(strings.TrimSpace(input), "1")
}

func buildCanvasSaveCommand(name, spec string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return "\\canvas save " + name + " '" + spec + "'"
}

func runCanvasValidation(state *ShellState) error {
	if state == nil {
		return fmt.Errorf("shell state is nil")
	}
	if state.activeCanvas == "" {
		return fmt.Errorf("no active canvas")
	}
	if state.canvasValidation != "" {
		return fmt.Errorf("canvas validation failed: %s", state.canvasValidation)
	}
	return nil
}

func parseCanvasNamePayload(raw string) (string, error) {
	type payload struct {
		Name string `json:"name"`
	}
	var input payload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &input); err != nil {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			return trimmed, nil
		}
		return "", fmt.Errorf("canvas name payload requires name")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return "", fmt.Errorf("canvas name payload requires name")
	}
	return name, nil
}

func parseCanvasRenamePayload(raw string) (string, string, error) {
	type payload struct {
		OldName string `json:"old_name"`
		NewName string `json:"new_name"`
	}
	var input payload
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &input); err == nil {
		oldName := strings.TrimSpace(input.OldName)
		newName := strings.TrimSpace(input.NewName)
		if oldName == "" || newName == "" {
			return "", "", fmt.Errorf("canvas rename payload requires old_name and new_name")
		}
		return oldName, newName, nil
	}
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) != 2 {
		return "", "", fmt.Errorf("canvas rename payload requires old_name and new_name")
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("canvas rename payload requires old_name and new_name")
	}
	return parts[0], parts[1], nil
}
