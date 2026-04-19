package shell

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
)

// ShellModel is a lightweight UI-facing façade over ShellCommandBus.
type ShellModel struct {
	bus *appstate.ShellCommandBus
}

// NewShellModel constructs a new view model for shell interactions.
func NewShellModel(bus *appstate.ShellCommandBus) *ShellModel {
	return &ShellModel{bus: bus}
}

// Projection exposes the current state snapshot for rendering.
func (m *ShellModel) Projection() appstate.ShellProjection {
	if m == nil || m.bus == nil || m.bus.State() == nil {
		return appstate.ShellProjection{}
	}
	return m.bus.State().Projection()
}

// Execute runs a terminal command through the bus.
func (m *ShellModel) Execute(input string) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionRunTerminal,
		Payload: input,
	})
}

// ToggleConsole toggles the event console visibility.
func (m *ShellModel) ToggleConsole() error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	return m.bus.Dispatch(appstate.Action{Kind: appstate.ActionToggleConsole})
}

// SetConsoleVisible forwards an explicit visibility action.
func (m *ShellModel) SetConsoleVisible(visible bool) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionSetConsoleState,
		Payload: visibilityPayload(visible),
	})
}

// HandleShortcut dispatches shortcut actions (for example F12 to toggle console).
func (m *ShellModel) HandleShortcut(keyName string) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	switch keyName {
	case "Ctrl+S":
		return m.SaveActiveCanvas()
	case "Ctrl+R":
		return m.RevertCanvas()
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionShortcut,
		Payload: keyName,
	})
}

// RunActiveCanvas executes the SQL generated from the active canvas draft.
func (m *ShellModel) RunActiveCanvas() error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	return m.bus.Dispatch(appstate.Action{
		Kind: appstate.ActionRunCanvas,
	})
}

// SetActiveCanvas updates the active canvas and loads its current spec into the projection state.
func (m *ShellModel) SetActiveCanvas(name string) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionSetCanvas,
		Payload: name,
	})
}

// SetCanvasDraft updates the in-memory canvas draft text for the active canvas.
func (m *ShellModel) SetCanvasDraft(spec string) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionSetCanvasDraft,
		Payload: spec,
	})
}

// MoveCanvasNode updates the active canvas node location.
func (m *ShellModel) MoveCanvasNode(nodeID string, x, y float64) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	payload, err := json.Marshal(struct {
		NodeID string  `json:"node_id"`
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
	}{
		NodeID: nodeID,
		X:      x,
		Y:      y,
	})
	if err != nil {
		return err
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionMoveCanvasNode,
		Payload: string(payload),
	})
}

// SetCanvasNodeFields updates selected fields for a node in active canvas draft.
func (m *ShellModel) SetCanvasNodeFields(nodeID string, fields []string) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	payload, err := json.Marshal(struct {
		NodeID string   `json:"node_id"`
		Fields []string `json:"fields"`
	}{
		NodeID: nodeID,
		Fields: fields,
	})
	if err != nil {
		return err
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionSetNodeFields,
		Payload: string(payload),
	})
}

// AddCanvasNode appends a new node into the active canvas draft.
func (m *ShellModel) AddCanvasNode(node query.CanvasNode) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	payload, err := json.Marshal(node)
	if err != nil {
		return err
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionAddCanvasNode,
		Payload: string(payload),
	})
}

// AddCanvasEdge adds a join edge to the active canvas draft.
func (m *ShellModel) AddCanvasEdge(edge query.CanvasEdge) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	payload, err := json.Marshal(edge)
	if err != nil {
		return err
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionAddCanvasEdge,
		Payload: string(payload),
	})
}

// PatchCanvasEdge updates a join edge in the active canvas draft.
func (m *ShellModel) PatchCanvasEdge(edge query.CanvasEdge) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	payload, err := json.Marshal(edge)
	if err != nil {
		return err
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionPatchCanvasEdge,
		Payload: string(payload),
	})
}

// DeleteCanvasEdge removes a join edge from the active canvas draft.
func (m *ShellModel) DeleteCanvasEdge(edgeID string) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	payload, err := json.Marshal(struct {
		EdgeID string `json:"edge_id"`
	}{
		EdgeID: edgeID,
	})
	if err != nil {
		return err
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionDeleteCanvasEdge,
		Payload: string(payload),
	})
}

// SaveActiveCanvas persists the active canvas draft using terminal canvas save command path.
func (m *ShellModel) SaveActiveCanvas() error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	return m.bus.Dispatch(appstate.Action{
		Kind: appstate.ActionSaveCanvas,
	})
}

// RevertCanvas restores the active canvas draft to last persisted spec.
func (m *ShellModel) RevertCanvas() error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	return m.bus.Dispatch(appstate.Action{
		Kind: appstate.ActionRevertCanvas,
	})
}

// ClearCanvas clears the currently active canvas from local workspace state.
func (m *ShellModel) ClearCanvas() error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	return m.bus.Dispatch(appstate.Action{
		Kind: appstate.ActionClearCanvas,
	})
}

// CreateCanvas creates a new empty draft canvas.
func (m *ShellModel) CreateCanvas(name string) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("canvas name is required")
	}
	payload, err := json.Marshal(struct {
		Name string `json:"name"`
	}{
		Name: name,
	})
	if err != nil {
		return err
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionCanvasNew,
		Payload: string(payload),
	})
}

// RenameCanvas renames an existing canvas artifact.
func (m *ShellModel) RenameCanvas(oldName, newName string) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("old and new canvas names are required")
	}
	payload, err := json.Marshal(struct {
		OldName string `json:"old_name"`
		NewName string `json:"new_name"`
	}{
		OldName: oldName,
		NewName: newName,
	})
	if err != nil {
		return err
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionCanvasRename,
		Payload: string(payload),
	})
}

// DeleteCanvas deletes a canvas artifact.
func (m *ShellModel) DeleteCanvas(name string) error {
	if m == nil || m.bus == nil {
		return fmt.Errorf("shell model has no command bus")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("canvas name is required")
	}
	payload, err := json.Marshal(struct {
		Name string `json:"name"`
	}{
		Name: name,
	})
	if err != nil {
		return err
	}
	return m.bus.Dispatch(appstate.Action{
		Kind:    appstate.ActionCanvasDelete,
		Payload: string(payload),
	})
}

func visibilityPayload(visible bool) string {
	if visible {
		return "true"
	}
	return "false"
}
