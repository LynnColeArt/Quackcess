//go:build gtk3
// +build gtk3

package gtk

import (
	"fmt"
	"math"
	"strings"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	gotk3gtk "github.com/gotk3/gotk3/gtk"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
)

// ErrGTKUnavailable is returned when no GUI runtime is linked.
var ErrGTKUnavailable = fmt.Errorf("GTK UI is unavailable in this build; recompile with -tags gtk3")

// ShellWindow renders the shell experience using GTK3.
type ShellWindow struct {
	bridge              *ShellBridge
	window              *gotk3gtk.Window
	entry               *gotk3gtk.Entry
	statusLabel         *gotk3gtk.Label
	explorer            *shellExplorerPanelState
	canvasViewport      *gotk3gtk.TextView
	canvasViewportArea  *gotk3gtk.DrawingArea
	canvasToolbar       *gotk3gtk.Box
	canvasSaveBtn       *gotk3gtk.Button
	canvasRunBtn        *gotk3gtk.Button
	canvasRevertBtn     *gotk3gtk.Button
	canvasClearBtn      *gotk3gtk.Button
	canvasFieldLabel    *gotk3gtk.Label
	canvasFieldEntry    *gotk3gtk.Entry
	canvasFieldApplyBtn *gotk3gtk.Button
	canvasEdgeLabel     *gotk3gtk.Label
	canvasEdgeFrom      *gotk3gtk.Entry
	canvasEdgeTo        *gotk3gtk.Entry
	canvasEdgeType      *gotk3gtk.Entry
	canvasEdgeApplyBtn  *gotk3gtk.Button
	sqlPreview          *gotk3gtk.TextView
	output              *gotk3gtk.TextView
	console             *gotk3gtk.TextView
	consoleBox          *gotk3gtk.Box
	projection          appstate.ShellProjection
	selectedCanvas      string

	draggingCanvasNodeID  string
	activeCanvasNodeID    string
	edgeDraftSourceNodeID string
	edgeDraftCursorX      float64
	edgeDraftCursorY      float64
	dragCanvasOffsetX     float64
	dragCanvasOffsetY     float64
	activeCanvasEdgeID    string
}

// NewShellWindow creates and wires a GTK3 shell window bound to a presenter bridge.
func NewShellWindow(bridge *ShellBridge, onProjection func(appstate.ShellProjection)) (*ShellWindow, error) {
	if bridge == nil {
		return nil, fmt.Errorf("shell window requires a bridge")
	}
	gotk3gtk.Init(nil)

	shellWindow := &ShellWindow{
		bridge: bridge,
	}

	window, err := gotk3gtk.WindowNew(gotk3gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return nil, err
	}
	window.SetTitle("Quackcess")
	window.SetDefaultSize(1120, 760)
	window.Connect("destroy", func() {
		gotk3gtk.MainQuit()
	})
	window.Connect("key-press-event", func(_ *gotk3gtk.Window, event *gdk.Event) bool {
		key := gdk.EventKeyNewFromEvent(event)
		if key == nil {
			return false
		}
		return handleShellWindowKeyEvent(shellWindow, key)
	})

	root, err := gotk3gtk.BoxNew(gotk3gtk.ORIENTATION_VERTICAL, 8)
	if err != nil {
		return nil, err
	}
	root.SetMarginTop(8)
	root.SetMarginBottom(8)
	root.SetMarginStart(8)
	root.SetMarginEnd(8)
	window.Add(root)

	header, err := gotk3gtk.LabelNew("Quackcess")
	if err != nil {
		return nil, err
	}
	header.SetHAlign(gotk3gtk.ALIGN_START)
	root.Add(header)

	statusLabel, err := gotk3gtk.LabelNew("Ready")
	if err != nil {
		return nil, err
	}
	statusLabel.SetHAlign(gotk3gtk.ALIGN_START)
	root.Add(statusLabel)

	split, err := gotk3gtk.BoxNew(gotk3gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		return nil, err
	}
	root.Add(split)

	explorer, err := shellExplorerPanel()
	if err != nil {
		return nil, err
	}
	explorer.SetOpenTableHandler(func(item string) {
		shellWindow.openExplorerSelection("table", item)
	})
	explorer.SetOpenViewHandler(func(item string) {
		shellWindow.openExplorerSelection("view", item)
	})
	explorer.SetOpenCanvasHandler(func(item string) {
		shellWindow.setSelectedCanvas(item)
		shellWindow.openExplorerSelection("canvas", item)
	})

	workspace, err := gotk3gtk.BoxNew(gotk3gtk.ORIENTATION_VERTICAL, 8)
	if err != nil {
		return nil, err
	}
	workspace.SetVExpand(true)

	canvasColumn, err := gotk3gtk.BoxNew(gotk3gtk.ORIENTATION_VERTICAL, 8)
	if err != nil {
		return nil, err
	}
	canvasColumn.SetVExpand(true)
	canvasColumn.SetSizeRequest(360, -1)

	canvasViewport, err := addCanvasViewportPanel(canvasColumn, "Canvas Viewport", 240)
	if err != nil {
		return nil, err
	}
	canvasViewport.SetCanFocus(true)
	canvasColumn.SetSizeRequest(360, -1)

	canvasInfo, err := addReadOnlyTextPanel(canvasColumn, "Canvas Info", 120)
	if err != nil {
		return nil, err
	}

	canvasToolbar, err := gotk3gtk.BoxNew(gotk3gtk.ORIENTATION_HORIZONTAL, 6)
	if canvasToolbar == nil {
		return nil, fmt.Errorf("failed to create canvas toolbar")
	}
	canvasSaveButton, err := gotk3gtk.ButtonNewWithLabel("Save Canvas")
	if err != nil {
		return nil, err
	}
	canvasRevertButton, err := gotk3gtk.ButtonNewWithLabel("Revert Canvas")
	if err != nil {
		return nil, err
	}
	canvasClearButton, err := gotk3gtk.ButtonNewWithLabel("Clear Canvas")
	if err != nil {
		return nil, err
	}
	canvasRunButton, err := gotk3gtk.ButtonNewWithLabel("Run Canvas")
	if err != nil {
		return nil, err
	}
	canvasSaveButton.SetSensitive(false)
	canvasRunButton.SetSensitive(false)
	canvasRevertButton.SetSensitive(false)
	canvasClearButton.SetSensitive(false)
	canvasToolbar.Add(canvasSaveButton)
	canvasToolbar.Add(canvasRevertButton)
	canvasToolbar.Add(canvasClearButton)
	canvasToolbar.Add(canvasRunButton)
	canvasColumn.Add(canvasToolbar)

	canvasSaveButton.Connect("clicked", func() { _ = shellWindow.SaveCanvas() })
	canvasRevertButton.Connect("clicked", func() { _ = shellWindow.RevertCanvas() })
	canvasClearButton.Connect("clicked", func() { _ = shellWindow.ClearCanvasSelection() })
	canvasRunButton.Connect("clicked", func() { _ = shellWindow.RunActiveCanvas() })

	canvasFieldLabel, canvasFieldEntry, canvasFieldApplyButton, err := addCanvasFieldEditorPanel(canvasColumn)
	if err != nil {
		return nil, err
	}
	canvasEdgeLabel, canvasEdgeFromEntry, canvasEdgeToEntry, canvasEdgeTypeEntry, canvasEdgeApplyButton, err := addCanvasEdgeEditorPanel(canvasColumn)
	if err != nil {
		return nil, err
	}
	canvasFieldApplyButton.Connect("clicked", func() { _ = shellWindow.applyActiveCanvasNodeFieldsFromEditor() })
	canvasEdgeApplyButton.Connect("clicked", func() { _ = shellWindow.applyActiveCanvasEdgeFromEditor() })

	wireCanvasViewportEvents(shellWindow, canvasViewport)

	sqlPreview, err := addReadOnlyTextPanel(workspace, "SQL Preview", 150)
	if err != nil {
		return nil, err
	}
	output, err := addReadOnlyTextPanel(workspace, "Results", 260)
	if err != nil {
		return nil, err
	}
	console, consoleBox, err := consolePanel(workspace)
	if err != nil {
		return nil, err
	}

	split.Add(explorer.Container)
	split.Add(canvasColumn)
	split.Add(workspace)

	entry, err := gotk3gtk.EntryNew()
	if err != nil {
		return nil, err
	}
	entry.SetPlaceholderText("Enter SQL or backslash command and press Enter...")
	root.Add(entry)

	window.ShowAll()

	shellWindow.window = window
	shellWindow.entry = entry
	shellWindow.statusLabel = statusLabel
	shellWindow.explorer = explorer
	shellWindow.canvasViewport = canvasInfo
	shellWindow.canvasViewportArea = canvasViewport
	shellWindow.canvasToolbar = canvasToolbar
	shellWindow.canvasSaveBtn = canvasSaveButton
	shellWindow.canvasRunBtn = canvasRunButton
	shellWindow.canvasRevertBtn = canvasRevertButton
	shellWindow.canvasClearBtn = canvasClearButton
	shellWindow.canvasFieldLabel = canvasFieldLabel
	shellWindow.canvasFieldEntry = canvasFieldEntry
	shellWindow.canvasFieldApplyBtn = canvasFieldApplyButton
	shellWindow.canvasEdgeLabel = canvasEdgeLabel
	shellWindow.canvasEdgeFrom = canvasEdgeFromEntry
	shellWindow.canvasEdgeTo = canvasEdgeToEntry
	shellWindow.canvasEdgeType = canvasEdgeTypeEntry
	shellWindow.canvasEdgeApplyBtn = canvasEdgeApplyButton
	shellWindow.sqlPreview = sqlPreview
	shellWindow.output = output
	shellWindow.console = console
	shellWindow.consoleBox = consoleBox

	entry.Connect("activate", func() {
		input, err := entry.GetText()
		if err != nil {
			return
		}
		entry.SetText("")
		_ = shellWindow.SubmitTerminalCommand(strings.TrimSpace(input))
	})

	shellWindow.refreshProjection()
	if onProjection != nil {
		onProjection(shellWindow.projection)
	}
	return shellWindow, nil
}

// SubmitTerminalCommand forwards text input into the shell presenter.
func (w *ShellWindow) SubmitTerminalCommand(input string) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.SubmitTerminalInput(input); err != nil {
		w.refreshProjection()
		if w.output != nil {
			setShellWindowText(w.output, err.Error())
		}
		return err
	}
	w.refreshProjection()
	return nil
}

// HandleKey forwards key actions into the shell presenter.
func (w *ShellWindow) HandleKey(name string) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.HandleKey(name); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// Projection returns the most recent shell projection from the presenter.
func (w *ShellWindow) Projection() appstate.ShellProjection {
	if w == nil {
		return appstate.ShellProjection{LastStatus: "idle"}
	}
	return w.projection
}

// Run starts the shell event loop.
func (w *ShellWindow) Run() error {
	if w == nil || w.window == nil {
		return fmt.Errorf("shell window is nil")
	}
	gotk3gtk.Main()
	return nil
}

func (w *ShellWindow) refreshProjection() {
	if w == nil || w.bridge == nil {
		return
	}
	w.projection = w.bridge.Projection()
	applyShellWindowProjection(w)
}

func (w *ShellWindow) openExplorerSelection(kind, name string) {
	if w == nil {
		return
	}
	command, err := explorerCommandForSelection(kind, name)
	if err != nil {
		setShellWindowText(w.output, err.Error())
		return
	}
	if err := w.SubmitTerminalCommand(command); err != nil {
		return
	}
}

func (w *ShellWindow) setSelectedCanvas(name string) {
	if w == nil {
		return
	}
	w.selectedCanvas = strings.TrimSpace(name)
	if err := w.bridge.SetActiveCanvas(w.selectedCanvas); err != nil {
		setShellWindowText(w.output, err.Error())
		return
	}
	w.refreshProjection()
	setShellWindowText(w.output, "selected canvas: "+w.selectedCanvas)
}

// SetActiveCanvas selects an active canvas in the shell projection.
func (w *ShellWindow) SetActiveCanvas(name string) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.SetActiveCanvas(name); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// SetCanvasDraft updates the active canvas draft.
func (w *ShellWindow) SetCanvasDraft(spec string) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.SetCanvasDraft(spec); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// MoveCanvasNode moves a node in the active canvas draft.
func (w *ShellWindow) MoveCanvasNode(nodeID string, x, y float64) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.MoveCanvasNode(nodeID, x, y); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// SetCanvasNodeFields updates selected fields for an active canvas node.
func (w *ShellWindow) SetCanvasNodeFields(nodeID string, fields []string) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.SetCanvasNodeFields(nodeID, fields); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// AddCanvasNode adds a node to the active canvas draft.
func (w *ShellWindow) AddCanvasNode(node query.CanvasNode) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.AddCanvasNode(node); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// AddCanvasEdge adds a join edge to the active canvas draft.
func (w *ShellWindow) AddCanvasEdge(edge query.CanvasEdge) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.AddCanvasEdge(edge); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// PatchCanvasEdge updates a join edge in active canvas draft.
func (w *ShellWindow) PatchCanvasEdge(edge query.CanvasEdge) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.PatchCanvasEdge(edge); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// DeleteCanvasEdge removes a join edge from active canvas draft.
func (w *ShellWindow) DeleteCanvasEdge(edgeID string) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.DeleteCanvasEdge(edgeID); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// SaveCanvas persists the active canvas draft.
func (w *ShellWindow) SaveCanvas() error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.SaveCanvas(); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// RevertCanvas restores the active canvas draft from persisted state.
func (w *ShellWindow) RevertCanvas() error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.RevertCanvas(); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// ClearCanvasSelection removes active canvas selection state.
func (w *ShellWindow) ClearCanvasSelection() error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.ClearCanvasSelection(); err != nil {
		w.refreshProjection()
		setShellWindowText(w.output, err.Error())
		return err
	}
	w.refreshProjection()
	return nil
}

// RunActiveCanvas executes the current canvas SQL preview.
func (w *ShellWindow) RunActiveCanvas() error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.RunActiveCanvas(); err != nil {
		w.refreshProjection()
		if w.output != nil {
			setShellWindowText(w.output, err.Error())
		}
		return err
	}
	w.refreshProjection()
	return nil
}

// CreateCanvas creates a new canvas artifact.
func (w *ShellWindow) CreateCanvas(name string) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.CreateCanvas(name); err != nil {
		w.refreshProjection()
		if w.output != nil {
			setShellWindowText(w.output, err.Error())
		}
		return err
	}
	w.refreshProjection()
	return nil
}

// RenameCanvas renames a canvas artifact.
func (w *ShellWindow) RenameCanvas(oldName, newName string) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.RenameCanvas(oldName, newName); err != nil {
		w.refreshProjection()
		if w.output != nil {
			setShellWindowText(w.output, err.Error())
		}
		return err
	}
	w.refreshProjection()
	return nil
}

// DeleteCanvas deletes a canvas artifact.
func (w *ShellWindow) DeleteCanvas(name string) error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if err := w.bridge.DeleteCanvas(name); err != nil {
		w.refreshProjection()
		if w.output != nil {
			setShellWindowText(w.output, err.Error())
		}
		return err
	}
	w.refreshProjection()
	return nil
}

func (w *ShellWindow) applyActiveCanvasNodeFieldsFromEditor() error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if w.activeCanvasNodeID == "" {
		setShellWindowText(w.output, "select a node before setting fields")
		return fmt.Errorf("no active canvas node")
	}
	if w.canvasFieldEntry == nil {
		return fmt.Errorf("node field editor is not available")
	}
	raw, err := w.canvasFieldEntry.GetText()
	if err != nil {
		return err
	}
	fields := parseCanvasFieldSelection(raw)
	if err := w.SetCanvasNodeFields(w.activeCanvasNodeID, fields); err != nil {
		setShellWindowText(w.output, err.Error())
		return err
	}
	return nil
}

func (w *ShellWindow) applyActiveCanvasEdgeFromEditor() error {
	if w == nil {
		return fmt.Errorf("shell window is nil")
	}
	if w.activeCanvasEdgeID == "" {
		setShellWindowText(w.output, "select an edge before updating join settings")
		return fmt.Errorf("no active canvas edge")
	}
	if w.canvasEdgeFrom == nil || w.canvasEdgeTo == nil || w.canvasEdgeType == nil {
		return fmt.Errorf("edge editor is not available")
	}

	spec, err := parseCanvasSpecForWindow(w)
	if err != nil {
		setShellWindowText(w.output, err.Error())
		return err
	}
	current, ok := canvasEdgeByID(spec, w.activeCanvasEdgeID)
	if !ok {
		setShellWindowText(w.output, "selected edge no longer exists")
		return fmt.Errorf("edge not found")
	}

	fromColumn, err := w.canvasEdgeFrom.GetText()
	if err != nil {
		return err
	}
	toColumn, err := w.canvasEdgeTo.GetText()
	if err != nil {
		return err
	}
	joinType, err := w.canvasEdgeType.GetText()
	if err != nil {
		return err
	}
	if strings.TrimSpace(fromColumn) == "" || strings.TrimSpace(toColumn) == "" {
		setShellWindowText(w.output, "join columns are required")
		return fmt.Errorf("join columns are required")
	}
	if _, err := query.ParseJoinType(strings.TrimSpace(joinType)); err != nil {
		setShellWindowText(w.output, err.Error())
		return err
	}

	if err := w.PatchCanvasEdge(query.CanvasEdge{
		ID:           current.ID,
		Kind:         current.Kind,
		FromNode:     current.FromNode,
		ToNode:       current.ToNode,
		FromColumn:   fromColumn,
		ToColumn:     toColumn,
		JoinType:     strings.TrimSpace(joinType),
		Label:        current.Label,
		Description:  current.Description,
		Tags:         current.Tags,
		Cardinality:  current.Cardinality,
		DirectedHint: current.DirectedHint,
	}); err != nil {
		setShellWindowText(w.output, err.Error())
		return err
	}
	return nil
}

func (w *ShellWindow) refreshCanvasEditPanels() {
	if w == nil {
		return
	}
	spec, err := parseCanvasSpecForWindow(w)
	if err != nil {
		resetCanvasNodeEditor(w)
		resetCanvasEdgeEditor(w)
		return
	}

	nodeID := strings.TrimSpace(w.activeCanvasNodeID)
	node := canvasNodeByID(spec, nodeID)
	if node != nil {
		if w.canvasFieldLabel != nil {
			w.canvasFieldLabel.SetText("Active Node: " + node.ID + " (" + node.Table + ")")
		}
		if w.canvasFieldEntry != nil {
			var values []string
			if len(node.SelectedFields) > 0 {
				values = append([]string(nil), node.SelectedFields...)
			} else {
				for _, field := range node.Fields {
					values = append(values, field.Name)
				}
			}
			w.canvasFieldEntry.SetText(strings.Join(values, ", "))
			w.canvasFieldEntry.SetSensitive(true)
		}
		if w.canvasFieldApplyBtn != nil {
			w.canvasFieldApplyBtn.SetSensitive(true)
		}
	} else {
		w.activeCanvasNodeID = ""
		resetCanvasNodeEditor(w)
	}

	edgeID := strings.TrimSpace(w.activeCanvasEdgeID)
	edge, ok := canvasEdgeByID(spec, edgeID)
	if ok {
		if w.canvasEdgeLabel != nil {
			w.canvasEdgeLabel.SetText("Active Edge: " + edge.ID)
		}
		if w.canvasEdgeFrom != nil && w.canvasEdgeTo != nil && w.canvasEdgeType != nil {
			w.canvasEdgeFrom.SetText(edge.FromColumn)
			w.canvasEdgeTo.SetText(edge.ToColumn)
			w.canvasEdgeType.SetText(edge.JoinType)
			w.canvasEdgeFrom.SetSensitive(true)
			w.canvasEdgeTo.SetSensitive(true)
			w.canvasEdgeType.SetSensitive(true)
		}
		if w.canvasEdgeApplyBtn != nil {
			w.canvasEdgeApplyBtn.SetSensitive(true)
		}
	} else {
		resetCanvasEdgeEditor(w)
		w.activeCanvasEdgeID = ""
	}
}

func parseCanvasFieldSelection(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ','
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func resetCanvasNodeEditor(window *ShellWindow) {
	if window == nil {
		return
	}
	if window.canvasFieldLabel != nil {
		window.canvasFieldLabel.SetText("Active Node: none")
	}
	if window.canvasFieldEntry != nil {
		window.canvasFieldEntry.SetText("")
		window.canvasFieldEntry.SetSensitive(false)
	}
	if window.canvasFieldApplyBtn != nil {
		window.canvasFieldApplyBtn.SetSensitive(false)
	}
}

func resetCanvasEdgeEditor(window *ShellWindow) {
	if window == nil {
		return
	}
	if window.canvasEdgeLabel != nil {
		window.canvasEdgeLabel.SetText("Active Edge: none")
	}
	if window.canvasEdgeFrom != nil {
		window.canvasEdgeFrom.SetText("")
		window.canvasEdgeFrom.SetSensitive(false)
	}
	if window.canvasEdgeTo != nil {
		window.canvasEdgeTo.SetText("")
		window.canvasEdgeTo.SetSensitive(false)
	}
	if window.canvasEdgeType != nil {
		window.canvasEdgeType.SetText("")
		window.canvasEdgeType.SetSensitive(false)
	}
	if window.canvasEdgeApplyBtn != nil {
		window.canvasEdgeApplyBtn.SetSensitive(false)
	}
}

func applyShellWindowProjection(window *ShellWindow) {
	if window == nil {
		return
	}
	if window.statusLabel != nil {
		window.statusLabel.SetText(window.projection.LastStatus)
	}
	if window.explorer != nil {
		setShellExplorerPanel(window.explorer, window.projection)
	}
	if window.canvasViewport != nil {
		setShellWindowText(window.canvasViewport, shellWindowCanvasViewport(window.projection))
	}
	if window.canvasViewportArea != nil {
		window.canvasViewportArea.QueueDraw()
	}
	if window.consoleBox != nil {
		window.consoleBox.SetVisible(window.projection.ConsoleVisible)
	}
	if window.canvasSaveBtn != nil {
		window.canvasSaveBtn.SetSensitive(window.projection.ActiveCanvas != "" && window.projection.CanvasDirty && window.projection.CanvasValidation == "")
	}
	if window.canvasRevertBtn != nil {
		window.canvasRevertBtn.SetSensitive(window.projection.ActiveCanvas != "" && window.projection.CanvasDirty)
	}
	if window.canvasClearBtn != nil {
		window.canvasClearBtn.SetSensitive(window.projection.ActiveCanvas != "")
	}
	if window.canvasRunBtn != nil {
		window.canvasRunBtn.SetSensitive(window.projection.ActiveCanvas != "" && strings.TrimSpace(window.projection.CanvasSQLPreview) != "")
	}
	if window.sqlPreview != nil {
		setShellWindowText(window.sqlPreview, shellWindowSQLPreview(window.projection))
	}
	if window.output != nil {
		setShellWindowText(window.output, shellWindowOutputText(window.projection))
	}
	if window.console != nil {
		setShellWindowText(window.console, formatConsoleEvents(window.projection.ConsoleItems))
	}
	window.refreshCanvasEditPanels()
	if strings.TrimSpace(window.projection.ActiveCanvas) == "" || strings.TrimSpace(window.projection.CanvasDraftSpec) == "" {
		window.resetCanvasInteractionState()
	}
}

func setShellWindowText(textView *gotk3gtk.TextView, value string) {
	if textView == nil {
		return
	}
	buffer, err := textView.GetBuffer()
	if err != nil {
		return
	}
	buffer.SetText(value)
}

func shellWindowSQLPreview(projection appstate.ShellProjection) string {
	if projection.CanvasSQLPreview != "" {
		return projection.CanvasSQLPreview
	}
	if projection.SQLText != "" {
		return projection.SQLText
	}
	return projection.LastInput
}

func shellWindowOutputText(projection appstate.ShellProjection) string {
	if projection.OutputText != "" {
		return projection.OutputText
	}
	if projection.RowCount > 0 {
		return "rows: " + fmt.Sprintf("%d", projection.RowCount)
	}
	return projection.LastStatus
}

func formatConsoleEvents(items int) string {
	return fmt.Sprintf("console events: %d", items)
}

func shellWindowCanvasViewport(projection appstate.ShellProjection) string {
	lines := []string{"Canvas viewport", "Stored canvases"}
	if len(projection.CatalogCanvases) == 0 {
		lines = append(lines, "• (none)")
		lines = append(lines, "")
		lines = append(lines, "Canvas viewport")
		return strings.Join(lines, "\n")
	}
	active := strings.TrimSpace(projection.ActiveCanvas)
	if active == "" {
		lines = append(lines, "selected: (none)")
	} else {
		lines = append(lines, "selected: "+active)
	}
	lines = append(lines, "status: "+projection.CanvasStatus)
	if projection.CanvasValidation != "" {
		lines = append(lines, "validation: "+projection.CanvasValidation)
	}
	if projection.CanvasDirty {
		lines = append(lines, "dirty: true")
	}
	for _, canvas := range projection.CatalogCanvases {
		lines = append(lines, "• "+canvas)
	}
	return strings.Join(lines, "\n")
}

func explorerCommandForSelection(kind, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("no explorer selection")
	}
	switch kind {
	case "table", "view":
		return fmt.Sprintf("SELECT * FROM %s", quoteIdentifier(name)), nil
	case "canvas":
		return `\canvas ` + name, nil
	default:
		return "", fmt.Errorf("unsupported explorer item: %s", kind)
	}
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func handleShellWindowKeyEvent(window *ShellWindow, event *gdk.EventKey) bool {
	if window == nil || event == nil {
		return false
	}
	keyName := formatShellWindowShortcut(event)
	if keyName == "" {
		return false
	}
	return window.HandleKey(keyName) == nil
}

func formatShellWindowShortcut(event *gdk.EventKey) string {
	if event == nil {
		return ""
	}
	keyName := gdk.KeyValName(event.KeyVal())
	if keyName == "" {
		return ""
	}
	modifiers := gdk.ModifierType(event.State())
	if modifiers&gdk.CONTROL_MASK != 0 {
		return "Ctrl+" + strings.ToUpper(keyName)
	}
	return keyName
}

func addCanvasViewportPanel(container *gotk3gtk.Box, label string, minHeight int) (*gotk3gtk.DrawingArea, error) {
	heading, err := gotk3gtk.LabelNew(label)
	if err != nil {
		return nil, err
	}
	heading.SetHAlign(gotk3gtk.ALIGN_START)
	container.Add(heading)

	canvasArea, err := gotk3gtk.DrawingAreaNew()
	if err != nil {
		return nil, err
	}
	canvasArea.SetCanFocus(true)
	canvasArea.SetVAlign(gotk3gtk.ALIGN_START)
	canvasArea.SetSizeRequest(300, minHeight)
	canvasArea.SetAppPaintable(true)
	container.Add(canvasArea)
	return canvasArea, nil
}

func addCanvasFieldEditorPanel(container *gotk3gtk.Box) (*gotk3gtk.Label, *gotk3gtk.Entry, *gotk3gtk.Button, error) {
	label, err := gotk3gtk.LabelNew("Active Node: none")
	if err != nil {
		return nil, nil, nil, err
	}
	label.SetHAlign(gotk3gtk.ALIGN_START)
	container.Add(label)

	entry, err := gotk3gtk.EntryNew()
	if err != nil {
		return nil, nil, nil, err
	}
	entry.SetPlaceholderText("id, created_at, ...")
	entry.SetSensitive(false)
	container.Add(entry)

	button, err := gotk3gtk.ButtonNewWithLabel("Apply Field Selection")
	if err != nil {
		return nil, nil, nil, err
	}
	button.SetSensitive(false)
	container.Add(button)
	return label, entry, button, nil
}

func addCanvasEdgeEditorPanel(container *gotk3gtk.Box) (*gotk3gtk.Label, *gotk3gtk.Entry, *gotk3gtk.Entry, *gotk3gtk.Entry, *gotk3gtk.Button, error) {
	label, err := gotk3gtk.LabelNew("Active Edge: none")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	label.SetHAlign(gotk3gtk.ALIGN_START)
	container.Add(label)

	fromEntry, err := gotk3gtk.EntryNew()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	fromEntry.SetPlaceholderText("join from column")
	fromEntry.SetSensitive(false)
	container.Add(fromEntry)

	toEntry, err := gotk3gtk.EntryNew()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	toEntry.SetPlaceholderText("join to column")
	toEntry.SetSensitive(false)
	container.Add(toEntry)

	typeEntry, err := gotk3gtk.EntryNew()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	typeEntry.SetPlaceholderText("INNER JOIN")
	typeEntry.SetSensitive(false)
	container.Add(typeEntry)

	button, err := gotk3gtk.ButtonNewWithLabel("Apply Edge Update")
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	button.SetSensitive(false)
	container.Add(button)

	return label, fromEntry, toEntry, typeEntry, button, nil
}

func wireCanvasViewportEvents(window *ShellWindow, canvasArea *gotk3gtk.DrawingArea) {
	if window == nil || canvasArea == nil {
		return
	}
	canvasArea.AddEvents(int(gdk.BUTTON_PRESS_MASK | gdk.BUTTON_RELEASE_MASK | gdk.POINTER_MOTION_MASK | gdk.ENTER_NOTIFY_MASK | gdk.LEAVE_NOTIFY_MASK))
	canvasArea.Connect("draw", window.handleCanvasViewportDraw)
	canvasArea.Connect("button-press-event", window.handleCanvasViewportButtonPress)
	canvasArea.Connect("button-release-event", window.handleCanvasViewportButtonRelease)
	canvasArea.Connect("motion-notify-event", window.handleCanvasViewportMotion)
}

func (w *ShellWindow) handleCanvasViewportDraw(_ *gotk3gtk.DrawingArea, cr *cairo.Context) {
	if w == nil {
		return
	}
	if cr == nil {
		return
	}

	spec, err := parseCanvasSpecForWindow(w)
	if err != nil || len(spec.Nodes) == 0 {
		cr.SetSourceRGB(0.98, 0.98, 0.98)
		cr.Paint()
		cr.SetSourceRGB(0.3, 0.3, 0.3)
		cr.SetFontSize(11)
		cr.MoveTo(12, 22)
		cr.ShowText("No active canvas draft to render")
		return
	}

	width := float64(w.canvasViewportArea.GetAllocatedWidth())
	height := float64(w.canvasViewportArea.GetAllocatedHeight())

	cr.SetSourceRGB(0.98, 0.98, 0.98)
	cr.Paint()
	cr.SetLineWidth(1.0)
	cr.SetSourceRGB(0.86, 0.86, 0.86)
	drawCanvasGrid(cr, 16, width, height)

	nodeBounds := make(map[string][4]float64)
	for _, node := range spec.Nodes {
		bounds := canvasNodeBounds(node)
		nodeBounds[node.ID] = bounds
	}

	for _, edge := range spec.Edges {
		fromBounds, fromExists := nodeBounds[edge.FromNode]
		toBounds, toExists := nodeBounds[edge.ToNode]
		if !fromExists || !toExists {
			continue
		}
		fromX := fromBounds[0] + fromBounds[2]/2
		fromY := fromBounds[1] + fromBounds[3]/2
		toX := toBounds[0] + toBounds[2]/2
		toY := toBounds[1] + toBounds[3]/2

		if edge.ID == w.activeCanvasEdgeID {
			cr.SetSourceRGB(0.12, 0.45, 0.72)
			cr.SetLineWidth(2.2)
		} else {
			cr.SetSourceRGB(0.35, 0.35, 0.35)
			cr.SetLineWidth(1.4)
		}
		cr.MoveTo(fromX, fromY)
		cr.LineTo(toX, toY)
		cr.Stroke()

		midX := (fromX + toX) / 2
		midY := (fromY + toY) / 2
		cr.SetSourceRGB(0.16, 0.16, 0.16)
		cr.SetFontSize(9)
		cr.MoveTo(midX+4, midY-2)
		label := strings.TrimSpace(edge.JoinType)
		if label == "" {
			label = query.CanvasEdgeKindJoin
		}
		cr.ShowText(label)
	}

	if w.edgeDraftSourceNodeID != "" {
		if sourceBounds, ok := nodeBounds[w.edgeDraftSourceNodeID]; ok {
			cr.SetSourceRGB(0.2, 0.56, 0.2)
			cr.SetLineWidth(2.0)
			cr.MoveTo(sourceBounds[0]+sourceBounds[2]/2, sourceBounds[1]+sourceBounds[3]/2)
			cr.LineTo(w.edgeDraftCursorX, w.edgeDraftCursorY)
			cr.Stroke()
		}
	}

	for _, node := range spec.Nodes {
		bounds := canvasNodeBounds(node)
		nodeColor := 0.95
		borderColor := 0.24
		if node.ID == w.draggingCanvasNodeID || node.ID == w.activeCanvasNodeID {
			nodeColor = 0.86
			borderColor = 0.15
		}
		if node.ID == w.edgeDraftSourceNodeID {
			nodeColor = 0.8
			borderColor = 0.18
		}

		cr.SetSourceRGB(nodeColor, nodeColor, 1.0)
		cr.Rectangle(bounds[0], bounds[1], bounds[2], bounds[3])
		cr.FillPreserve()
		cr.SetSourceRGB(borderColor, borderColor, borderColor)
		cr.SetLineWidth(1.3)
		cr.Stroke()

		cr.SetSourceRGB(0.12, 0.12, 0.12)
		cr.SetFontSize(10.8)
		title := node.Table
		if title == "" {
			title = node.ID
		}
		header := node.Alias
		if header == "" {
			header = node.Table
		}
		cr.MoveTo(bounds[0]+6, bounds[1]+16)
		cr.ShowText(header)
		if title != header {
			cr.MoveTo(bounds[0]+6, bounds[1]+30)
			cr.ShowText("(" + title + ")")
		}

		cr.SetSourceRGB(0.2, 0.2, 0.2)
		cr.SetFontSize(9.5)
		fieldY := bounds[1] + 44
		for _, field := range node.Fields {
			if fieldY >= bounds[1]+bounds[3]-10 {
				break
			}
			label := field.Name
			if field.Alias != "" {
				label = label + " [" + field.Alias + "]"
			}
			cr.MoveTo(bounds[0]+7, fieldY)
			cr.ShowText(label)
			fieldY += 13
		}
	}
}

func (w *ShellWindow) handleCanvasViewportButtonPress(_ *gotk3gtk.DrawingArea, event *gdk.Event) bool {
	if w == nil || w.canvasViewportArea == nil || event == nil {
		return false
	}
	button := gdk.EventButtonNewFromEvent(event)
	if button == nil {
		return false
	}

	x := button.X()
	y := button.Y()
	w.edgeDraftCursorX = x
	w.edgeDraftCursorY = y
	spec, err := parseCanvasSpecForWindow(w)
	if err != nil {
		return false
	}
	nodeID := hitCanvasNodeAt(spec, x, y)
	if nodeID == "" {
		if w.edgeDraftSourceNodeID != "" {
			w.edgeDraftSourceNodeID = ""
			if w.canvasViewportArea != nil {
				w.canvasViewportArea.QueueDraw()
			}
			return true
		}
		edgeID := hitCanvasEdgeAt(spec, x, y)
		if edgeID == "" {
			w.activeCanvasNodeID = ""
			w.activeCanvasEdgeID = ""
			w.refreshCanvasEditPanels()
			return false
		}
		if button.Button() == 1 {
			w.activeCanvasNodeID = ""
			w.activeCanvasEdgeID = edgeID
			w.refreshCanvasEditPanels()
		}
		return true
	}

	if button.Button() == 3 {
		w.activeCanvasEdgeID = ""
		w.refreshCanvasEditPanels()
		return true
	}
	if button.Button() != 1 {
		return false
	}

	ctrlPressed := gdk.ModifierType(button.State())&gdk.CONTROL_MASK != 0
	if ctrlPressed {
		if w.edgeDraftSourceNodeID == "" || w.edgeDraftSourceNodeID == nodeID {
			w.edgeDraftSourceNodeID = nodeID
			w.activeCanvasEdgeID = ""
			w.activeCanvasNodeID = nodeID
		} else {
			if err := w.createCanvasEdgeForNodes(w.edgeDraftSourceNodeID, nodeID); err != nil {
				setShellWindowText(w.output, err.Error())
			}
			w.edgeDraftSourceNodeID = ""
		}
		w.refreshCanvasEditPanels()
		w.canvasViewportArea.QueueDraw()
		return true
	}

	node := canvasNodeByID(spec, nodeID)
	if node == nil {
		return false
	}
	w.draggingCanvasNodeID = nodeID
	w.dragCanvasOffsetX = x - node.X
	w.dragCanvasOffsetY = y - node.Y
	w.activeCanvasNodeID = nodeID
	w.activeCanvasEdgeID = ""
	w.refreshCanvasEditPanels()
	return true
}

func (w *ShellWindow) handleCanvasViewportMotion(_ *gotk3gtk.DrawingArea, event *gdk.Event) bool {
	if w == nil || event == nil || w.canvasViewportArea == nil {
		return false
	}
	motion := gdk.EventMotionNewFromEvent(event)
	if motion == nil {
		return false
	}
	x, y := motion.MotionVal()
	w.edgeDraftCursorX = x
	w.edgeDraftCursorY = y

	if w.draggingCanvasNodeID != "" {
		nextX := x - w.dragCanvasOffsetX
		nextY := y - w.dragCanvasOffsetY
		if nextX < 0 {
			nextX = 0
		}
		if nextY < 0 {
			nextY = 0
		}
		if err := w.MoveCanvasNode(w.draggingCanvasNodeID, nextX, nextY); err != nil {
			setShellWindowText(w.output, err.Error())
		}
		return true
	}
	if w.edgeDraftSourceNodeID != "" {
		if w.canvasViewportArea != nil {
			w.canvasViewportArea.QueueDraw()
		}
		return true
	}
	return false
}

func (w *ShellWindow) handleCanvasViewportButtonRelease(_ *gotk3gtk.DrawingArea, event *gdk.Event) bool {
	if w == nil || event == nil {
		return false
	}
	button := gdk.EventButtonNewFromEvent(event)
	if button == nil {
		return false
	}
	if button.Button() == 1 {
		if w.draggingCanvasNodeID != "" {
			w.draggingCanvasNodeID = ""
			return true
		}
	}
	return false
}

func (w *ShellWindow) createCanvasEdgeForNodes(fromNodeID, toNodeID string) error {
	spec, err := parseCanvasSpecForWindow(w)
	if err != nil {
		return err
	}
	if fromNodeID == "" || toNodeID == "" || fromNodeID == toNodeID {
		return fmt.Errorf("cannot create edge from and to same node")
	}
	fromNode := canvasNodeByID(spec, fromNodeID)
	toNode := canvasNodeByID(spec, toNodeID)
	if fromNode == nil || toNode == nil {
		return fmt.Errorf("edge nodes not found")
	}
	if len(fromNode.Fields) == 0 {
		return fmt.Errorf("source node has no fields")
	}
	if len(toNode.Fields) == 0 {
		return fmt.Errorf("target node has no fields")
	}

	nextID := nextCanvasDraftEdgeID(spec)
	return w.AddCanvasEdge(query.CanvasEdge{
		ID:         nextID,
		Kind:       query.CanvasEdgeKindJoin,
		FromNode:   fromNode.ID,
		ToNode:     toNode.ID,
		FromColumn: fromNode.Fields[0].Name,
		ToColumn:   toNode.Fields[0].Name,
		JoinType:   "INNER",
	})
}

func nextCanvasDraftEdgeID(spec query.CanvasSpec) string {
	existing := map[string]struct{}{}
	for _, edge := range spec.Edges {
		existing[edge.ID] = struct{}{}
	}
	for i := 1; ; i++ {
		id := fmt.Sprintf("edge-%d", i)
		if _, ok := existing[id]; !ok {
			return id
		}
	}
}

func parseCanvasSpecForWindow(window *ShellWindow) (query.CanvasSpec, error) {
	if window == nil {
		return query.CanvasSpec{}, fmt.Errorf("window is nil")
	}
	raw := strings.TrimSpace(window.projection.CanvasDraftSpec)
	if raw == "" {
		return query.CanvasSpec{}, fmt.Errorf("canvas draft spec is empty")
	}
	spec, err := query.ParseCanvasSpec([]byte(raw))
	if err != nil {
		return query.CanvasSpec{}, err
	}
	return spec, nil
}

func canvasNodeByID(spec query.CanvasSpec, nodeID string) *query.CanvasNode {
	for i, node := range spec.Nodes {
		if node.ID == nodeID {
			return &spec.Nodes[i]
		}
	}
	return nil
}

func canvasEdgeByID(spec query.CanvasSpec, edgeID string) (query.CanvasEdge, bool) {
	for _, edge := range spec.Edges {
		if edge.ID == edgeID {
			return edge, true
		}
	}
	return query.CanvasEdge{}, false
}

func hitCanvasNodeAt(spec query.CanvasSpec, x, y float64) string {
	for _, node := range spec.Nodes {
		bounds := canvasNodeBounds(node)
		if x >= bounds[0] && x <= bounds[0]+bounds[2] && y >= bounds[1] && y <= bounds[1]+bounds[3] {
			return node.ID
		}
	}
	return ""
}

func hitCanvasEdgeAt(spec query.CanvasSpec, x, y float64) string {
	for _, edge := range spec.Edges {
		from := canvasNodeByID(spec, edge.FromNode)
		to := canvasNodeByID(spec, edge.ToNode)
		if from == nil || to == nil {
			continue
		}
		fromX := from.X + from.Width/2
		fromY := from.Y + from.Height/2
		toX := to.X + to.Width/2
		toY := to.Y + to.Height/2
		if distancePointToSegment(x, y, fromX, fromY, toX, toY) < 8 {
			return edge.ID
		}
	}
	return ""
}

func distancePointToSegment(px, py, x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	if dx == 0 && dy == 0 {
		return math.Hypot(px-x1, py-y1)
	}
	t := ((px-x1)*dx + (py-y1)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	closestX := x1 + t*dx
	closestY := y1 + t*dy
	return math.Hypot(px-closestX, py-closestY)
}

func canvasNodeBounds(node query.CanvasNode) [4]float64 {
	return [4]float64{
		node.X,
		node.Y,
		node.Width,
		node.Height,
	}
}

func drawCanvasGrid(cr *cairo.Context, spacing float64, width, height float64) {
	if spacing <= 0 {
		return
	}
	for x := float64(0); x <= width; x += spacing {
		cr.MoveTo(x, 0)
		cr.LineTo(x, height)
		cr.Stroke()
	}
	for y := float64(0); y <= height; y += spacing {
		cr.MoveTo(0, y)
		cr.LineTo(width, y)
		cr.Stroke()
	}
}

func (w *ShellWindow) resetCanvasInteractionState() {
	if w == nil {
		return
	}
	w.draggingCanvasNodeID = ""
	w.edgeDraftSourceNodeID = ""
	w.activeCanvasNodeID = ""
	w.activeCanvasEdgeID = ""
	w.dragCanvasOffsetX = 0
	w.dragCanvasOffsetY = 0
}

func addReadOnlyTextPanel(container *gotk3gtk.Box, label string, minHeight int) (*gotk3gtk.TextView, error) {
	_, textView, err := addReadOnlyTextPanelWithContainer(container, label, minHeight)
	return textView, err
}

func addReadOnlyTextPanelWithContainer(container *gotk3gtk.Box, label string, minHeight int) (*gotk3gtk.ScrolledWindow, *gotk3gtk.TextView, error) {
	heading, err := gotk3gtk.LabelNew(label)
	if err != nil {
		return nil, nil, err
	}
	heading.SetHAlign(gotk3gtk.ALIGN_START)
	container.Add(heading)

	scroller, err := gotk3gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, nil, err
	}
	scroller.SetVExpand(true)
	scroller.SetMinContentHeight(minHeight)

	textView, err := gotk3gtk.TextViewNew()
	if err != nil {
		return nil, nil, err
	}
	textView.SetEditable(false)
	textView.SetWrapMode(gotk3gtk.WRAP_WORD_CHAR)
	textView.SetMonospace(true)

	scroller.Add(textView)
	container.Add(scroller)
	return scroller, textView, nil
}

func consolePanel(container *gotk3gtk.Box) (*gotk3gtk.TextView, *gotk3gtk.Box, error) {
	consoleView, err := gotk3gtk.TextViewNew()
	if err != nil {
		return nil, nil, err
	}
	consoleView.SetEditable(false)
	consoleView.SetWrapMode(gotk3gtk.WRAP_WORD_CHAR)
	consoleView.SetMonospace(true)

	consoleBox, err := gotk3gtk.BoxNew(gotk3gtk.ORIENTATION_VERTICAL, 4)
	if err != nil {
		return nil, nil, err
	}

	consoleHeading, err := gotk3gtk.LabelNew("Event Console")
	if err != nil {
		return nil, nil, err
	}
	consoleHeading.SetHAlign(gotk3gtk.ALIGN_START)
	consoleBox.Add(consoleHeading)

	consoleScroller, err := gotk3gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return nil, nil, err
	}
	consoleScroller.SetVExpand(true)
	consoleScroller.SetMinContentHeight(140)
	consoleScroller.Add(consoleView)
	consoleBox.Add(consoleScroller)
	consoleBox.SetVisible(false)
	container.Add(consoleBox)

	return consoleView, consoleBox, nil
}

type shellExplorerPanelState struct {
	Container         *gotk3gtk.Box
	Tables            *gotk3gtk.ComboBoxText
	Views             *gotk3gtk.ComboBoxText
	Canvases          *gotk3gtk.ComboBoxText
	openTableButton   *gotk3gtk.Button
	openViewButton    *gotk3gtk.Button
	openCanvasButton  *gotk3gtk.Button
	openTableHandler  func(string)
	openViewHandler   func(string)
	openCanvasHandler func(string)
}

func shellExplorerPanel() (*shellExplorerPanelState, error) {
	panel, err := gotk3gtk.BoxNew(gotk3gtk.ORIENTATION_VERTICAL, 4)
	if err != nil {
		return nil, err
	}
	panel.SetMarginTop(2)
	panel.SetMarginBottom(2)
	panel.SetMarginStart(2)
	panel.SetMarginEnd(2)
	panel.SetSizeRequest(190, -1)

	header, err := gotk3gtk.LabelNew("Explorer")
	if err != nil {
		return nil, err
	}
	header.SetHAlign(gotk3gtk.ALIGN_START)
	panel.Add(header)

	tables, tablesOpen, err := explorerPickerPanel(panel, "Tables")
	if err != nil {
		return nil, err
	}
	views, viewsOpen, err := explorerPickerPanel(panel, "Views")
	if err != nil {
		return nil, err
	}
	canvases, canvasesOpen, err := explorerPickerPanel(panel, "Canvases")
	if err != nil {
		return nil, err
	}

	return &shellExplorerPanelState{
		Container:        panel,
		Tables:           tables,
		Views:            views,
		Canvases:         canvases,
		openTableButton:  tablesOpen,
		openViewButton:   viewsOpen,
		openCanvasButton: canvasesOpen,
	}, nil
}

func explorerPickerPanel(parent *gotk3gtk.Box, section string) (*gotk3gtk.ComboBoxText, *gotk3gtk.Button, error) {
	header, err := gotk3gtk.LabelNew(section)
	if err != nil {
		return nil, nil, err
	}
	header.SetHAlign(gotk3gtk.ALIGN_START)
	parent.Add(header)

	combo, err := gotk3gtk.ComboBoxTextNew()
	if err != nil {
		return nil, nil, err
	}
	combo.SetSizeRequest(180, -1)
	combo.SetWrapWidth(1)
	parent.Add(combo)

	button, err := gotk3gtk.ButtonNewWithLabel("Open " + section)
	if err != nil {
		return nil, nil, err
	}
	parent.Add(button)
	button.SetSensitive(false)
	return combo, button, nil
}

func (panel *shellExplorerPanelState) SetOpenTableHandler(handler func(string)) {
	panel.openTableHandler = handler
	panel.openTableButton.Connect("clicked", func() {
		if panel.openTableHandler == nil {
			return
		}
		if panel.Tables == nil {
			return
		}
		name := strings.TrimSpace(panel.Tables.GetActiveText())
		if name == "" || name == "(none)" {
			return
		}
		panel.openTableHandler(name)
	})
}

func (panel *shellExplorerPanelState) SetOpenViewHandler(handler func(string)) {
	panel.openViewHandler = handler
	panel.openViewButton.Connect("clicked", func() {
		if panel.openViewHandler == nil {
			return
		}
		if panel.Views == nil {
			return
		}
		name := strings.TrimSpace(panel.Views.GetActiveText())
		if name == "" || name == "(none)" {
			return
		}
		panel.openViewHandler(name)
	})
}

func (panel *shellExplorerPanelState) SetOpenCanvasHandler(handler func(string)) {
	panel.openCanvasHandler = handler
	panel.openCanvasButton.Connect("clicked", func() {
		if panel.openCanvasHandler == nil {
			return
		}
		if panel.Canvases == nil {
			return
		}
		name := strings.TrimSpace(panel.Canvases.GetActiveText())
		if name == "" || name == "(none)" {
			return
		}
		panel.openCanvasHandler(name)
	})
}

func setShellExplorerPanel(panel *shellExplorerPanelState, projection appstate.ShellProjection) {
	setShellExplorerOptions(panel.Tables, panel.openTableButton, projection.CatalogTables)
	setShellExplorerOptions(panel.Views, panel.openViewButton, projection.CatalogViews)
	setShellExplorerOptions(panel.Canvases, panel.openCanvasButton, projection.CatalogCanvases)
}

func setShellExplorerOptions(combo *gotk3gtk.ComboBoxText, openButton *gotk3gtk.Button, items []string) {
	if combo == nil {
		return
	}
	combo.RemoveAll()
	if len(items) == 0 {
		combo.AppendText("(none)")
		combo.SetActive(0)
		combo.SetSensitive(false)
		if openButton != nil {
			openButton.SetSensitive(false)
		}
		return
	}

	combo.SetSensitive(true)
	if openButton != nil {
		openButton.SetSensitive(true)
	}
	for _, item := range items {
		combo.AppendText(item)
	}
	combo.SetActive(0)
}
