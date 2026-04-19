package appstate

import (
	"errors"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

type vectorizeArtifactWriteRecorder struct {
	calls    int
	input    string
	metadata terminal.TerminalVectorizeMetadata
	err      error
}

func (r *vectorizeArtifactWriteRecorder) Write(input string, metadata terminal.TerminalVectorizeMetadata) error {
	r.calls++
	r.input = input
	r.metadata = metadata
	return r.err
}

func TestShellCommandBusVectorRunWritesVectorOperationArtifactSpec(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindQuery,
			RowCount: 1,
			Vectorize: &terminal.TerminalVectorizeMetadata{
				TableName:    "docs",
				SourceColumn: "body",
				TargetColumn: "body_vec",
				Filter:       "id <= 2",
				FieldID:      "vf-docs-body",
				Built:        true,
				BatchSize:    64,
				VectorCount:  2,
				SkipReason:   "",
			},
		},
	}
	writer := &vectorizeArtifactWriteRecorder{}
	bus := NewShellCommandBusWithVectorWriter(runner, NewShellState(nil), writer.Write)

	if err := bus.Dispatch(Action{
		Kind:    ActionRunTerminal,
		Payload: "UPDATE docs VECTORIZE body AS body_vec WHERE id <= 2",
	}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want 1", writer.calls)
	}
	if writer.input != "UPDATE docs VECTORIZE body AS body_vec WHERE id <= 2" {
		t.Fatalf("writer input = %q, want full command", writer.input)
	}
	if writer.metadata.VectorCount != 2 {
		t.Fatalf("vector count = %d, want 2", writer.metadata.VectorCount)
	}
}

func TestShellCommandBusVectorRunPropagatesWriterError(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindQuery,
			Vectorize: &terminal.TerminalVectorizeMetadata{
				TableName:    "docs",
				SourceColumn: "body",
				TargetColumn: "body_vec",
				FieldID:      "vf-docs-body",
			},
		},
	}
	expErr := errors.New("artifact write failed")
	writer := &vectorizeArtifactWriteRecorder{err: expErr}
	bus := NewShellCommandBusWithVectorWriter(runner, NewShellState(nil), writer.Write)

	err := bus.Dispatch(Action{
		Kind:    ActionRunVectorize,
		Payload: "UPDATE docs VECTORIZE body AS body_vec",
	})
	if err == nil {
		t.Fatal("expected writer failure to be returned")
	}
	if err != expErr {
		t.Fatalf("err = %v, want %v", err, expErr)
	}
}

func TestShellCommandBusRunVectorActionWithoutMetadataDoesNotWrite(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindQuery,
			RowCount: 1,
		},
	}
	writer := &vectorizeArtifactWriteRecorder{}
	bus := NewShellCommandBusWithVectorWriter(runner, NewShellState(nil), writer.Write)

	if err := bus.Dispatch(Action{
		Kind:    ActionRunVectorize,
		Payload: "SELECT * FROM docs",
	}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if writer.calls != 0 {
		t.Fatalf("writer calls = %d, want 0", writer.calls)
	}
}
