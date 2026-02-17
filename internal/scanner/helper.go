package scanner

import (
	"log"
	"time"

	natslib "github.com/nats-io/nats.go"
)

// ProcessWithConcurrency runs fn with semaphore and standardized logging.
// name: scanner name (e.g. "Wallet"); kind: plugin kind (e.g. "wallet"); subject: NATS subject.
// After fn returns, logs duration and success or error.
func ProcessWithConcurrency(name, kind, subject string, sem chan struct{}, msg *natslib.Msg, fn func() error) error {
	sem <- struct{}{}
	defer func() { <-sem }()

	start := time.Now()
	err := fn()
	duration := time.Since(start)

	if err != nil {
		log.Printf("[%s] %s scan failed | subject=%s duration=%v error=%v", name, kind, subject, duration, err)
		return err
	}
	log.Printf("[%s] %s scan completed | subject=%s duration=%v", name, kind, subject, duration)
	return nil
}
