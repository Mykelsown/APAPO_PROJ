package app

import (
	"context"
	"log"
	"time"
)

// StartWorkers runs the background passes until ctx is cancelled: reversal retries and the
// Split-Receive scan, every 10 seconds. It first rescans unfinished payments.
func (w *World) StartWorkers(ctx context.Context, every time.Duration) {
	if n := w.Rescan(); n > 0 {
		log.Printf("startup rescan: %d unfinished payment(s) picked up", n)
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.RetryReversals()
				w.ScanAll()
			}
		}
	}()
}
