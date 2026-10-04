// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/ctx42/testing/pkg/assert"
)

// gateWriter is an io.Writer that signals when its first Write begins and
// blocks there until release is closed.
type gateWriter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *gateWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(p), nil
}

func Test_withProgress(t *testing.T) {
	// --- Given ---
	// TestMain disables progress package-wide; re-enable a short threshold so
	// the progress goroutine fires within this test.
	prev := progressThreshold
	progressThreshold = 10 * time.Millisecond
	t.Cleanup(func() { progressThreshold = prev })

	w := &gateWriter{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}

	returned := make(chan struct{})

	// --- When ---
	// fn outlives progressThreshold so the progress goroutine writes the
	// message; it returns once that write has begun (and is now blocked in
	// gateWriter.Write).
	go func() {
		_ = withProgress(w, "working...", func() error {
			<-w.entered
			return nil
		})
		close(returned)
	}()

	// --- Then ---
	<-w.entered // The progress goroutine has begun writing and is blocked.
	select {
	case <-returned:
		t.Fatal("withProgress returned before the progress goroutine " +
			"finished writing to stderr")
	case <-time.After(100 * time.Millisecond):
		// Correct: withProgress is still waiting for the goroutine.
	}

	close(w.release) // Let the progress write complete.
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("withProgress did not return after the progress write " +
			"completed")
	}
}

func Test_withProgress_panic(t *testing.T) {
	// --- Given ---
	prev := progressThreshold
	progressThreshold = 10 * time.Millisecond
	t.Cleanup(func() { progressThreshold = prev })

	var mu sync.Mutex
	buf := &bytes.Buffer{}
	w := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	})

	// --- When ---
	func() {
		defer func() { _ = recover() }()
		_ = withProgress(w, "working...", func() error { panic("boom") })
	}()

	// --- Then ---
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "", buf.String())
}

// writerFunc adapts a function to [io.Writer].
type writerFunc func(p []byte) (int, error)

func (fn writerFunc) Write(p []byte) (int, error) { return fn(p) }
