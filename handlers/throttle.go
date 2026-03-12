package handlers

import (
	"io"
	"time"
)

// ThrottledReader wraps an io.Reader and limits read throughput.
// This helps prevent overwhelming JetStream's internal pending queue
// when uploading large files to replicated object stores.
type ThrottledReader struct {
	reader      io.Reader
	bytesPerSec int64
	lastRead    time.Time
	bytesRead   int64
}

// NewThrottledReader creates a reader that limits throughput to the specified bytes per second.
// For replicated JetStream clusters, a good starting point is 50-100 MB/s.
func NewThrottledReader(r io.Reader, bytesPerSec int64) *ThrottledReader {
	return &ThrottledReader{
		reader:      r,
		bytesPerSec: bytesPerSec,
		lastRead:    time.Now(),
	}
}

func (tr *ThrottledReader) Read(p []byte) (n int, err error) {
	n, err = tr.reader.Read(p)
	if n > 0 {
		tr.bytesRead += int64(n)

		// Calculate how long we should have taken at our target rate
		expectedDuration := time.Duration(float64(tr.bytesRead) / float64(tr.bytesPerSec) * float64(time.Second))
		actualDuration := time.Since(tr.lastRead)

		// If we're ahead of schedule, sleep to throttle
		if expectedDuration > actualDuration {
			time.Sleep(expectedDuration - actualDuration)
		}
	}
	return n, err
}
