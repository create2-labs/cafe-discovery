package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"cafe-discovery/internal/metrics"
	"cafe-discovery/internal/persistence/scanhttp"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	walletLifecycleAttempts = 2
	walletLifecycleTimeout  = 15 * time.Second
	walletLifecycleGap      = time.Second
	walletLifecycleNakDelay = 2 * time.Minute

	walletLifecycleAlert = "wallet lifecycle alert"

	walletLifecycleMetadata   = "metadata"
	walletLifecycleInvalid    = "invalid"
	walletLifecycleRejected   = "rejected"
	walletLifecycleRetryable  = "retryable"
	walletLifecycleTransport  = "transport"
	walletLifecycleProjection = "projection"
)

// WalletLifecycleMessage is one JetStream delivery.
// jetstream.Msg implements it.
type WalletLifecycleMessage interface {
	Subject() string
	Data() []byte
	Metadata() (*jetstream.MsgMetadata, error)
	Ack() error
	Term() error
	NakWithDelay(delay time.Duration) error
}

type walletApply func(ctx context.Context, userID, scanID uuid.UUID, body []byte) (string, string, bool, error)

type walletDeliverySettler struct {
	apply  walletApply
	wait   func(ctx context.Context, d time.Duration) error
	record func(reason string)
	log    zerolog.Logger
}

// SettleWalletLifecycle settles one delivery.
// It calls ApplyWalletTransition at most twice and does not start the consumer.
func SettleWalletLifecycle(ctx context.Context, client *scanhttp.Client, msg WalletLifecycleMessage) {
	walletDeliverySettler{
		apply:  client.ApplyWalletTransition,
		wait:   waitWalletLifecycle,
		record: metrics.Get().RecordWalletLifecycle,
		log:    log.Logger,
	}.settle(ctx, msg)
}

func waitWalletLifecycle(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s walletDeliverySettler) settle(ctx context.Context, msg WalletLifecycleMessage) {
	meta, err := msg.Metadata()
	if err != nil || meta == nil {
		if err == nil {
			err = errors.New("jetstream metadata is missing")
		}
		s.log.Error().Err(err).Msg("wallet lifecycle metadata unavailable")
		s.record(walletLifecycleMetadata)
		s.finish(msg.NakWithDelay(walletLifecycleNakDelay), "nak")
		return
	}

	cmd, err := WalletTransitionCommand(msg.Subject(), msg.Data(), meta.Stream, meta.Sequence.Stream)
	if err != nil {
		if errors.Is(err, ErrNotWalletEvent) {
			s.finish(msg.Ack(), "ack")
			return
		}
		s.log.Warn().Err(err).Msg("wallet lifecycle invalid")
		s.record(walletLifecycleInvalid)
		s.finish(msg.Term(), "term")
		return
	}

	body, err := json.Marshal(cmd)
	if err != nil {
		s.log.Warn().Err(err).Msg("wallet lifecycle invalid")
		s.record(walletLifecycleInvalid)
		s.finish(msg.Term(), "term")
		return
	}

	transport := false
	for attempt := 0; attempt < walletLifecycleAttempts; attempt++ {
		if attempt > 0 {
			if err := s.wait(ctx, walletLifecycleGap); err != nil {
				s.log.Warn().Err(err).Msg("wallet lifecycle delivery interrupted")
				return
			}
		}
		outcome, ready, callErr := s.post(ctx, cmd, body)
		if callErr != nil {
			transport = true
			continue
		}
		switch outcome {
		case "applied", "already_applied":
			if !scan.IsTerminal(cmd.TargetState) || ready {
				s.finish(msg.Ack(), "ack")
				return
			}
			s.alert(walletLifecycleProjection)
			s.finish(msg.NakWithDelay(walletLifecycleNakDelay), "nak")
			return
		case "already_running", "already_terminal":
			s.finish(msg.Ack(), "ack")
			return
		case "rejected":
			s.log.Warn().Str("outcome", outcome).Msg("wallet lifecycle rejected")
			s.record(walletLifecycleRejected)
			s.finish(msg.Term(), "term")
			return
		case "retryable_error":
			continue
		default:
			transport = true
		}
	}

	reason := walletLifecycleRetryable
	if transport {
		reason = walletLifecycleTransport
	}
	s.alert(reason)
	s.finish(msg.NakWithDelay(walletLifecycleNakDelay), "nak")
}

func (s walletDeliverySettler) post(ctx context.Context, cmd walletCommand, body []byte) (string, bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, walletLifecycleTimeout)
	defer cancel()
	outcome, _, ready, err := s.apply(callCtx, cmd.UserID, cmd.ScanID, body)
	return outcome, ready, err
}

func (s walletDeliverySettler) alert(reason string) {
	s.log.Error().Str("reason", reason).Msg(walletLifecycleAlert)
	s.record(reason)
}

func (s walletDeliverySettler) finish(err error, action string) {
	if err != nil {
		s.log.Error().Err(err).Str("action", action).Msg("wallet lifecycle acknowledgement failed")
	}
}
