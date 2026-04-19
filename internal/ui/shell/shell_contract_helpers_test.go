package shell

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

type fakeShellCatalogExplorer struct {
	canvases map[string]string
	tables   []string
	views    []string
}

func (f *fakeShellCatalogExplorer) ListTables() ([]string, error) {
	return f.tables, nil
}

func (f *fakeShellCatalogExplorer) ListViews() ([]string, error) {
	return f.views, nil
}

func (f *fakeShellCatalogExplorer) ListCanvases() ([]string, error) {
	names := make([]string, 0, len(f.canvases))
	for name := range f.canvases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (f *fakeShellCatalogExplorer) LoadCanvasSpec(name string) (string, error) {
	if spec, ok := f.canvases[strings.TrimSpace(name)]; ok {
		return spec, nil
	}
	return "", fmt.Errorf("canvas not found: %s", name)
}

type shellFakeRunner struct {
	calls  int
	last   string
	result terminal.TerminalResult
	err    error
}

func (r *shellFakeRunner) RunCommand(input string) (terminal.TerminalResult, error) {
	r.calls++
	r.last = input
	if r.err != nil {
		return terminal.TerminalResult{}, r.err
	}
	r.result.Input = input
	return r.result, nil
}
