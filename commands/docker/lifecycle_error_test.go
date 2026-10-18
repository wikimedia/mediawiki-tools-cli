package docker

import (
	"errors"
	"strings"
	"testing"
)

func TestLifecycleErrorRetainsCause(t *testing.T) {
	failure := errors.New("Address already in use")
	for _, action := range []string{"start", "stop", "restart"} {
		if err := lifecycleError(action, nil); err != nil {
			t.Fatal(err)
		}
		err := lifecycleError(action, failure)
		if !errors.Is(err, failure) || !strings.Contains(err.Error(), action) {
			t.Fatal(err)
		}
	}
	for _, command := range []func() bool{
		func() bool { c := NewMwddStartCmd(); return c.RunE != nil && c.Run == nil },
		func() bool { c := NewMwddStopCmd(); return c.RunE != nil && c.Run == nil },
		func() bool { c := NewMwddRestartCmd(); return c.RunE != nil && c.Run == nil },
	} {
		if !command() {
			t.Fatal("lifecycle handler must return errors to Cobra")
		}
	}
}
