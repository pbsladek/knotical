package provider

import (
	"fmt"
	"sync"
	"testing"
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
