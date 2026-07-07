// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"io"
	"time"
)

// progressThreshold is how long an action may run before withProgress emits
// its message. It is a var, not a const, so tests can disable timing-based
// progress output and stay deterministic regardless of compilation speed.
var progressThreshold = 500 * time.Millisecond

// withProgress runs fn and prints msg to w if fn takes longer than
// progressThreshold (500ms).
func withProgress(w io.Writer, msg string, action func() error) error {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		t := time.NewTimer(progressThreshold)
		defer t.Stop()
		select {
		case <-done:
		case <-t.C:
			_, _ = fmt.Fprintln(w, msg)
		}
	}()
	err := action()
	close(done)
	<-stopped // Ensure the progress goroutine cannot write after we return.
	return err
}
