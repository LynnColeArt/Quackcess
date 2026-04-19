package shell

import (
	"fmt"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
)

// ShellPresenter acts as a thin UI presenter over the shell model.
// It emits projection snapshots whenever an action is dispatched.
type ShellPresenter struct {
	model          *ShellModel
	onProjection   func(appstate.ShellProjection)
	lastProjection appstate.ShellProjection
}

// NewShellPresenter creates a presenter that notifies the callback after every dispatch.
func NewShellPresenter(model *ShellModel, onProjection func(appstate.ShellProjection)) *ShellPresenter {
	return &ShellPresenter{
		model:        model,
		onProjection: onProjection,
		lastProjection: appstate.ShellProjection{
			LastStatus: "idle",
		},
	}
}

// Projection returns the latest known projection.
func (p *ShellPresenter) Projection() appstate.ShellProjection {
	if p == nil || p.model == nil {
		return appstate.ShellProjection{LastStatus: "idle"}
	}
	p.lastProjection = p.model.Projection()
	return p.lastProjection
}

func (p *ShellPresenter) emitProjection() {
	if p == nil || p.model == nil {
		return
	}
	p.lastProjection = p.model.Projection()
	if p.onProjection != nil {
		p.onProjection(p.lastProjection)
	}
}

// SubmitTerminalCommand sends a terminal input through the model and emits a projection.
func (p *ShellPresenter) SubmitTerminalCommand(input string) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.Execute(input); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// HandleShortcut dispatches a key shortcut action and emits a projection.
func (p *ShellPresenter) HandleShortcut(keyName string) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.HandleShortcut(keyName); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// RunActiveCanvas executes the current canvas SQL preview through terminal runner and emits a projection.
func (p *ShellPresenter) RunActiveCanvas() error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.RunActiveCanvas(); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// SetActiveCanvas selects and loads a canvas into shell projection state.
func (p *ShellPresenter) SetActiveCanvas(name string) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.SetActiveCanvas(name); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// SetCanvasDraft updates the canvas draft in shell projection state.
func (p *ShellPresenter) SetCanvasDraft(spec string) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.SetCanvasDraft(spec); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// MoveCanvasNode moves a canvas node in the active draft.
func (p *ShellPresenter) MoveCanvasNode(nodeID string, x, y float64) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.MoveCanvasNode(nodeID, x, y); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// SetCanvasNodeFields updates selected fields for an active canvas node.
func (p *ShellPresenter) SetCanvasNodeFields(nodeID string, fields []string) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.SetCanvasNodeFields(nodeID, fields); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// AddCanvasNode appends a canvas node in the active draft.
func (p *ShellPresenter) AddCanvasNode(node query.CanvasNode) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.AddCanvasNode(node); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// AddCanvasEdge adds a canvas join edge in draft mode.
func (p *ShellPresenter) AddCanvasEdge(edge query.CanvasEdge) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.AddCanvasEdge(edge); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// PatchCanvasEdge updates a join edge in draft mode.
func (p *ShellPresenter) PatchCanvasEdge(edge query.CanvasEdge) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.PatchCanvasEdge(edge); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// DeleteCanvasEdge removes a join edge from active canvas draft.
func (p *ShellPresenter) DeleteCanvasEdge(edgeID string) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.DeleteCanvasEdge(edgeID); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// SaveActiveCanvas persists the active canvas draft.
func (p *ShellPresenter) SaveActiveCanvas() error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.SaveActiveCanvas(); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// RevertCanvas restores the persisted canvas spec into draft state.
func (p *ShellPresenter) RevertCanvas() error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.RevertCanvas(); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// ClearCanvas clears active canvas panel state.
func (p *ShellPresenter) ClearCanvas() error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.ClearCanvas(); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// CreateCanvas creates a new canvas and emits projection updates.
func (p *ShellPresenter) CreateCanvas(name string) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.CreateCanvas(name); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// RenameCanvas renames a canvas and emits projection updates.
func (p *ShellPresenter) RenameCanvas(oldName, newName string) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.RenameCanvas(oldName, newName); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}

// DeleteCanvas deletes a canvas and emits projection updates.
func (p *ShellPresenter) DeleteCanvas(name string) error {
	if p == nil || p.model == nil {
		return fmt.Errorf("shell presenter has no model")
	}
	if err := p.model.DeleteCanvas(name); err != nil {
		p.emitProjection()
		return err
	}
	p.emitProjection()
	return nil
}
