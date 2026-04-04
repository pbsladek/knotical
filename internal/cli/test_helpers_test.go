package cli

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/pbsladek/knotical/internal/output"
)

type handlerFailureRecorder struct {
	mu  sync.Mutex
	err error
}

func (r *handlerFailureRecorder) Failf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err == nil {
		r.err = fmt.Errorf(format, args...)
	}
}

func (r *handlerFailureRecorder) Assert(t *testing.T) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		t.Fatal(r.err)
	}
}

func captureDefaultOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buffer bytes.Buffer
	restore := output.SetDefaultPrinter(output.NewPrinter(&buffer))
	t.Cleanup(restore)
	return &buffer
}
