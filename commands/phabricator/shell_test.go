package phabricator

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type shellWaitingInput struct {
	io.ReadCloser
	waiting chan struct{}
}

func (r shellWaitingInput) Read(p []byte) (int, error) {
	select {
	case r.waiting <- struct{}{}:
	default:
	}
	return r.ReadCloser.Read(p)
}

func TestShellCancellationWhileWaitingForInput(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	input := shellWaitingInput{ReadCloser: reader, waiting: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var output bytes.Buffer
	result := make(chan error, 1)
	go func() {
		result <- runShell(ctx, input, &output, io.Discard, nil, &PhabConfig{}, "", "")
	}()
	select {
	case <-input.waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("shell did not start waiting for input")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shell remained blocked after cancellation")
	}
	if !strings.Contains(output.String(), "bye") {
		t.Fatalf("missing exit message: %q", output.String())
	}
	if _, err := writer.Write([]byte("unused")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("pending input was not closed: %v", err)
	}
}

func TestShellExplicitExit(t *testing.T) {
	var output bytes.Buffer
	err := runShell(context.Background(), strings.NewReader("exit\n"), &output, io.Discard, nil, &PhabConfig{}, "", "")
	if err != nil || !strings.Contains(output.String(), "bye") {
		t.Fatalf("shell exit: output %q, error %v", output.String(), err)
	}
}
