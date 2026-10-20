package nats

import (
	"context"
	"errors"
	"fmt"
	"time"

	lifecyclev1 "github.com/create2-labs/cafe-contracts/discovery/scanlifecycle/v1"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
)

const (
	durableDiscoveryWalletLifecycle = "discovery-wallet-lifecycle"
	walletLifecycleAckWait          = 2 * time.Minute
	walletLifecycleMaxDeliver       = -1
	walletLifecycleMaxAckPending    = 64
)

// walletLifecycleConsumerConfig is the durable explicit consumer.
// DeliverAll resumes an unacknowledged message after a restart.
// NakWithDelay, issued by the settler, is what releases an ack slot.
func walletLifecycleConsumerConfig() jetstream.ConsumerConfig {
	return jetstream.ConsumerConfig{
		Durable:       durableDiscoveryWalletLifecycle,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       walletLifecycleAckWait,
		MaxDeliver:    walletLifecycleMaxDeliver,
		MaxAckPending: walletLifecycleMaxAckPending,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		FilterSubjects: []string{
			lifecyclev1.SubjectScanStarted,
			lifecyclev1.SubjectScanCompleted,
			lifecyclev1.SubjectScanFailed,
		},
	}
}

// ConsumeWalletLifecycle binds discovery-wallet-lifecycle on SCAN_LIFECYCLE.
// The consumer stays alive until ctx is cancelled.
func (nc *natsConnection) ConsumeWalletLifecycle(ctx context.Context, handler func(msg jetstream.Msg)) error {
	if nc.conn == nil {
		return errors.New("nats connection is nil")
	}
	js, err := nc.jet()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}
	stream, err := js.Stream(ctx, StreamScanLifecycle)
	if err != nil {
		return fmt.Errorf("stream %s: %w", StreamScanLifecycle, err)
	}
	cfg := walletLifecycleConsumerConfig()
	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("consumer %s: %w", cfg.Durable, err)
	}
	cc, err := consumer.Consume(handler, jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
		log.Error().Err(err).Str("durable", cfg.Durable).Msg("wallet lifecycle consumer error")
	}))
	if err != nil {
		return fmt.Errorf("consume %s: %w", cfg.Durable, err)
	}
	go func() {
		<-ctx.Done()
		cc.Stop()
	}()
	return nil
}
