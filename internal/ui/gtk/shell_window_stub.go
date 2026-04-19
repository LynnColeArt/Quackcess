//go:build !gtk3
// +build !gtk3

package gtk

import (
	"errors"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
)

// ErrGTKUnavailable is returned when no GUI runtime is linked.
var ErrGTKUnavailable = errors.New("GTK UI is unavailable in this build; recompile with -tags gtk3")

// ShellWindow is a lightweight, non-UI shell binding used for contract verification.
type ShellWindow struct {
	bridge       *ShellBridge
	projection   appstate.ShellProjection
	onProjection func(appstate.ShellProjection)
}

// NewShellWindow creates a shell window handle bound to a presenter bridge.
// In non-GTK builds this returns an in-memory shell binding that executes state transitions.
func NewShellWindow(bridge *ShellBridge, onProjection func(appstate.ShellProjection)) (*ShellWindow, error) {
	if bridge == nil {
		return nil, errors.New("shell window requires a bridge")
	}
	window := &ShellWindow{
		bridge:       bridge,
		onProjection: onProjection,
	}
	window.refreshProjection()
	return window, nil
}

// SubmitTerminalCommand forwards text input into the shell presenter.
func (w *ShellWindow) SubmitTerminalCommand(input string) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.SubmitTerminalInput(input); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// SetActiveCanvas selects an active canvas in the shell projection.
func (w *ShellWindow) SetActiveCanvas(name string) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.SetActiveCanvas(name); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// SetCanvasDraft updates canvas draft text for active canvas state.
func (w *ShellWindow) SetCanvasDraft(spec string) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.SetCanvasDraft(spec); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// MoveCanvasNode updates a node position in the active canvas draft.
func (w *ShellWindow) MoveCanvasNode(nodeID string, x, y float64) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.MoveCanvasNode(nodeID, x, y); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// SetCanvasNodeFields updates selected fields for an active canvas node.
func (w *ShellWindow) SetCanvasNodeFields(nodeID string, fields []string) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.SetCanvasNodeFields(nodeID, fields); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// AddCanvasNode adds a node to the active canvas draft.
func (w *ShellWindow) AddCanvasNode(node query.CanvasNode) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.AddCanvasNode(node); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// AddCanvasEdge adds an edge to the active canvas draft.
func (w *ShellWindow) AddCanvasEdge(edge query.CanvasEdge) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.AddCanvasEdge(edge); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// PatchCanvasEdge updates an edge in the active canvas draft.
func (w *ShellWindow) PatchCanvasEdge(edge query.CanvasEdge) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.PatchCanvasEdge(edge); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// DeleteCanvasEdge deletes a draft edge.
func (w *ShellWindow) DeleteCanvasEdge(edgeID string) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.DeleteCanvasEdge(edgeID); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// SaveCanvas persists the active canvas draft.
func (w *ShellWindow) SaveCanvas() error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.SaveCanvas(); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// RunActiveCanvas executes the active canvas SQL preview.
func (w *ShellWindow) RunActiveCanvas() error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.RunActiveCanvas(); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// CreateCanvas persists a new canvas definition.
func (w *ShellWindow) CreateCanvas(name string) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.CreateCanvas(name); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// RenameCanvas renames a canvas and refreshes projection.
func (w *ShellWindow) RenameCanvas(oldName, newName string) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.RenameCanvas(oldName, newName); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// DeleteCanvas deletes a canvas and refreshes projection.
func (w *ShellWindow) DeleteCanvas(name string) error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.DeleteCanvas(name); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// RevertCanvas restores active canvas draft to repository-backed content.
func (w *ShellWindow) RevertCanvas() error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.RevertCanvas(); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// ClearCanvasSelection resets active canvas panel state.
func (w *ShellWindow) ClearCanvasSelection() error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	if err := w.bridge.ClearCanvasSelection(); err != nil {
		w.refreshProjection()
		return err
	}
	w.refreshProjection()
	return nil
}

// HandleKey forwards key actions into the shell presenter.
func (w *ShellWindow) HandleKey(name string) error {
	if w == nil {
		return errors.New("shell window is nil")
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

// Run starts the shell event loop. In non-GTK builds this reports unavailability.
func (w *ShellWindow) Run() error {
	if w == nil {
		return errors.New("shell window is nil")
	}
	return ErrGTKUnavailable
}

func (w *ShellWindow) refreshProjection() {
	if w == nil || w.bridge == nil {
		return
	}
	w.projection = w.bridge.Projection()
	if w.onProjection != nil {
		w.onProjection(w.projection)
	}
}
