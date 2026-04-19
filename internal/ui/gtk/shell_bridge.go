package gtk

import (
	"fmt"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/ui/shell"
)

// ShellBridge is a temporary non-UI bridge that will be bound to GTK widgets.
// It keeps UI-layer concerns (key/submit wiring) separate from core shell state.
type ShellBridge struct {
	Presenter *shell.ShellPresenter
}

// NewShellBridge constructs a bridge around a UI-ready presenter.
func NewShellBridge(presenter *shell.ShellPresenter) *ShellBridge {
	return &ShellBridge{Presenter: presenter}
}

// SubmitTerminalInput sends terminal text into the presenter.
func (b *ShellBridge) SubmitTerminalInput(input string) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.SubmitTerminalCommand(input)
}

// HandleKey forwards shortcuts (for example F12).
func (b *ShellBridge) HandleKey(keyName string) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.HandleShortcut(keyName)
}

// SetActiveCanvas selects and loads a canvas into the panel state.
func (b *ShellBridge) SetActiveCanvas(name string) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.SetActiveCanvas(name)
}

// SetCanvasDraft updates the current in-memory canvas draft for the panel.
func (b *ShellBridge) SetCanvasDraft(spec string) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.SetCanvasDraft(spec)
}

// MoveCanvasNode moves a node in the active canvas draft.
func (b *ShellBridge) MoveCanvasNode(nodeID string, x, y float64) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.MoveCanvasNode(nodeID, x, y)
}

// SetCanvasNodeFields updates selected fields on a node in active canvas draft.
func (b *ShellBridge) SetCanvasNodeFields(nodeID string, fields []string) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.SetCanvasNodeFields(nodeID, fields)
}

// AddCanvasNode inserts a new node into the active canvas draft.
func (b *ShellBridge) AddCanvasNode(node query.CanvasNode) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.AddCanvasNode(node)
}

// AddCanvasEdge adds an edge to the active canvas draft.
func (b *ShellBridge) AddCanvasEdge(edge query.CanvasEdge) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.AddCanvasEdge(edge)
}

// PatchCanvasEdge updates an edge in the active canvas draft.
func (b *ShellBridge) PatchCanvasEdge(edge query.CanvasEdge) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.PatchCanvasEdge(edge)
}

// DeleteCanvasEdge deletes an edge from the active canvas draft.
func (b *ShellBridge) DeleteCanvasEdge(edgeID string) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.DeleteCanvasEdge(edgeID)
}

// CreateCanvas creates a new canvas and forwards to presenter.
func (b *ShellBridge) CreateCanvas(name string) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.CreateCanvas(name)
}

// RenameCanvas renames a canvas artifact and forwards to presenter.
func (b *ShellBridge) RenameCanvas(oldName, newName string) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.RenameCanvas(oldName, newName)
}

// DeleteCanvas deletes a canvas artifact and forwards to presenter.
func (b *ShellBridge) DeleteCanvas(name string) error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.DeleteCanvas(name)
}

// SaveCanvas persists the active canvas draft using the model save path.
func (b *ShellBridge) SaveCanvas() error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.SaveActiveCanvas()
}

// RunActiveCanvas executes the active canvas SQL preview.
func (b *ShellBridge) RunActiveCanvas() error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.RunActiveCanvas()
}

// RevertCanvas restores the active canvas draft to persisted content.
func (b *ShellBridge) RevertCanvas() error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.RevertCanvas()
}

// ClearCanvasSelection clears active canvas panel state.
func (b *ShellBridge) ClearCanvasSelection() error {
	if b == nil || b.Presenter == nil {
		return fmt.Errorf("shell bridge has no presenter")
	}
	return b.Presenter.ClearCanvas()
}

// Projection provides the latest projection for rendering code.
func (b *ShellBridge) Projection() appstate.ShellProjection {
	if b == nil || b.Presenter == nil {
		return appstate.ShellProjection{LastStatus: "idle"}
	}
	return b.Presenter.Projection()
}
