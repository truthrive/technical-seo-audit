package sitecrawl

import (
	"testing"
	"time"
)

// fastCrawl speeds up timers for test runs and restores them during cleanup.
func fastCrawl(t *testing.T) {
	t.Helper()
	oldFlush, oldProgress, oldReport, oldGrace := flushEvery, progressEvery, reportEvery, pauseGraceMax
	oldRetry := fetchRetryDelay
	flushEvery = 20 * time.Millisecond
	progressEvery = 20 * time.Millisecond
	reportEvery = 20 * time.Millisecond
	pauseGraceMax = 2 * time.Second
	fetchRetryDelay = 5 * time.Millisecond
	t.Cleanup(func() {
		flushEvery, progressEvery, reportEvery, pauseGraceMax = oldFlush, oldProgress, oldReport, oldGrace
		fetchRetryDelay = oldRetry
	})
}
