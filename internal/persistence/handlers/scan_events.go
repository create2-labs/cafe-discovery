package handlers

import (
	"context"
	"encoding/json"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/persistence/storage"
	"cafe-discovery/pkg/nats"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type ScanEventHandler struct {
	tlsWriter    *storage.TLSWriter
	walletWriter *storage.WalletWriter
	redisCache   *storage.RedisCache
	natsConn     nats.Connection // optional: when set, publish scan.ready after writing to Redis so API can return result on GET
}

const IGNORE_SCAN_MSG = "unknown scan kind, ignoring"

func NewScanEventHandler(tlsWriter *storage.TLSWriter, walletWriter *storage.WalletWriter, redisCache *storage.RedisCache, natsConn nats.Connection) *ScanEventHandler {
	return &ScanEventHandler{
		tlsWriter:    tlsWriter,
		walletWriter: walletWriter,
		redisCache:   redisCache,
		natsConn:     natsConn,
	}
}

func (h *ScanEventHandler) HandleStarted(msg *nats.ScanStartedMessage) error {
	// TODO: tracing integration
	log.Info().
		Str("scan_id", msg.ScanID.String()).
		Str("kind", msg.Kind).
		Str("user_id", msg.UserID.String()).
		Msg("scan.started")

	switch msg.Kind {
	case "tls":
		var userID *uuid.UUID
		if msg.UserID != uuid.Nil {
			userID = &msg.UserID
		}
		if err := h.tlsWriter.OnStarted(msg.ScanID, userID, msg.Endpoint); err != nil {
			log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: OnStarted TLS failed")
			return err
		}
	case "wallet":
		if err := h.walletWriter.OnStarted(msg.ScanID, msg.UserID, msg.Address); err != nil {
			log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: OnStarted wallet failed")
			return err
		}
	default:
		log.Warn().Str("kind", msg.Kind).Msg(IGNORE_SCAN_MSG)
	}
	return nil
}

const subjectScanCompleted = "scan.completed"

func (h *ScanEventHandler) HandleCompleted(msg *nats.ScanCompletedMessage) error {
	log.Info().
		Str("scan_id", msg.ScanID.String()).
		Str("kind", msg.Kind).
		Str("user_id", msg.UserID.String()).
		Msg("scan.completed")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch msg.Kind {
	case "tls":
		return h.handleTLSCompleted(ctx, msg)
	case "wallet":
		return h.handleWalletCompleted(ctx, msg)
	default:
		log.Warn().Str("kind", msg.Kind).Msg(IGNORE_SCAN_MSG)
		return nil
	}
}

func (h *ScanEventHandler) handleTLSCompleted(ctx context.Context, msg *nats.ScanCompletedMessage) error {
	current, _ := h.tlsWriter.GetStatus(msg.ScanID)
	if current == "" {
		log.Warn().
			Str("scan_id", msg.ScanID.String()).
			Str("subject", subjectScanCompleted).
			Msg("persistence: missing scan on completed (no row yet), will upsert")
	}
	if !scan.ValidTransition(current, scan.StateSUCCESS) {
		log.Warn().
			Str("scan_id", msg.ScanID.String()).
			Str("current_status", current).
			Str("attempted_status", scan.StateSUCCESS).
			Str("subject", subjectScanCompleted).
			Msg("persistence: invalid transition or duplicate completion, ignoring")
		return nil
	}
	var result domain.TLSScanResult
	if err := h.decodeResult(msg.Result, &result); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: decode TLS result failed")
		return err
	}
	userID := &msg.UserID
	if msg.UserID == uuid.Nil {
		userID = nil
	}
	entity := domain.FromTLSScanResult(userID, &result, result.Default)
	entity.ID = msg.ScanID
	if err := h.tlsWriter.OnCompleted(msg.ScanID, entity); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: OnCompleted TLS failed")
		return err
	}
	if err := h.redisCache.SaveTLSScan(ctx, msg.UserID, msg.Endpoint, &result); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: Redis TLS write failed")
		return err
	}
	h.publishScanReady(msg.UserID, "tls", msg.Endpoint, "", "success")
	return nil
}

func (h *ScanEventHandler) handleWalletCompleted(ctx context.Context, msg *nats.ScanCompletedMessage) error {
	log.Debug().
		Str("scan_id", msg.ScanID.String()).
		Str("user_id", msg.UserID.String()).
		Str("address", msg.Address).
		Msg("persistence: wallet scan.completed received, checking status")
	current, _ := h.walletWriter.GetStatus(msg.ScanID)
	log.Debug().
		Str("scan_id", msg.ScanID.String()).
		Str("current_status", current).
		Msg("persistence: wallet GetStatus result")
	if current == "" {
		log.Warn().
			Str("scan_id", msg.ScanID.String()).
			Str("subject", subjectScanCompleted).
			Msg("persistence: missing scan on completed (no row yet), will upsert")
	}
	if !scan.ValidTransition(current, scan.StateSUCCESS) {
		log.Warn().
			Str("scan_id", msg.ScanID.String()).
			Str("current_status", current).
			Str("attempted_status", scan.StateSUCCESS).
			Str("subject", subjectScanCompleted).
			Msg("persistence: invalid transition or duplicate completion, ignoring")
		return nil
	}
	var result domain.ScanResult
	if err := h.decodeResult(msg.Result, &result); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: decode wallet result failed")
		return err
	}
	log.Debug().Str("scan_id", msg.ScanID.String()).Msg("persistence: wallet result decoded OK")
	entity := domain.FromScanResult(msg.UserID, &result)
	entity.ID = msg.ScanID
	if err := h.walletWriter.OnCompleted(msg.ScanID, entity); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: OnCompleted wallet failed")
		return err
	}
	log.Info().
		Str("scan_id", msg.ScanID.String()).
		Str("address", msg.Address).
		Msg("persistence: wallet Postgres write OK")
	if err := h.redisCache.SaveWalletScan(ctx, msg.UserID, msg.Address, &result); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: Redis wallet write failed")
		return err
	}
	log.Info().
		Str("scan_id", msg.ScanID.String()).
		Str("address", msg.Address).
		Str("user_id", msg.UserID.String()).
		Msg("persistence: wallet Redis write OK")
	h.publishScanReady(msg.UserID, "wallet", "", msg.Address, "success")
	return nil
}

func (h *ScanEventHandler) decodeResult(in interface{}, out interface{}) error {
	if in == nil {
		return nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

const subjectScanFailed = "scan.failed"

func (h *ScanEventHandler) HandleFailed(msg *nats.ScanFailedMessage) error {
	log.Info().
		Str("scan_id", msg.ScanID.String()).
		Str("kind", msg.Kind).
		Str("user_id", msg.UserID.String()).
		Str("error", msg.Error).
		Msg("scan.failed")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch msg.Kind {
	case "tls":
		return h.handleTLSFailed(ctx, msg)
	case "wallet":
		return h.handleWalletFailed(ctx, msg)
	default:
		log.Warn().Str("kind", msg.Kind).Msg(IGNORE_SCAN_MSG)
		return nil
	}
}

func (h *ScanEventHandler) handleTLSFailed(ctx context.Context, msg *nats.ScanFailedMessage) error {
	current, err := h.tlsWriter.GetStatus(msg.ScanID)
	if err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: GetStatus failed")
		return err
	}
	if current == "" {
		log.Warn().
			Str("scan_id", msg.ScanID.String()).
			Str("subject", subjectScanFailed).
			Msg("persistence: missing scan on failed (no row yet), will upsert")
	} else if !scan.ValidTransition(current, scan.StateFAILED) {
		log.Warn().
			Str("scan_id", msg.ScanID.String()).
			Str("current_status", current).
			Str("attempted_status", scan.StateFAILED).
			Str("subject", subjectScanFailed).
			Msg("persistence: invalid transition, ignoring")
		return nil
	}
	userID := (*uuid.UUID)(nil)
	if msg.UserID != uuid.Nil {
		userID = &msg.UserID
	}
	if err := h.tlsWriter.OnFailed(msg.ScanID, userID, msg.Endpoint, msg.Error); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: OnFailed TLS failed")
		return err
	}
	if err := h.redisCache.SaveTLSFailure(ctx, msg.UserID, msg.Endpoint, msg.Error); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: Redis TLS failure write failed")
		return err
	}
	h.publishScanReady(msg.UserID, "tls", msg.Endpoint, "", "failed")
	return nil
}

func (h *ScanEventHandler) handleWalletFailed(ctx context.Context, msg *nats.ScanFailedMessage) error {
	current, err := h.walletWriter.GetStatus(msg.ScanID)
	if err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: GetStatus failed")
		return err
	}
	if current == "" {
		log.Warn().
			Str("scan_id", msg.ScanID.String()).
			Str("subject", subjectScanFailed).
			Msg("persistence: missing scan on failed (no row yet), will upsert")
	} else if !scan.ValidTransition(current, scan.StateFAILED) {
		log.Warn().
			Str("scan_id", msg.ScanID.String()).
			Str("current_status", current).
			Str("attempted_status", scan.StateFAILED).
			Str("subject", subjectScanFailed).
			Msg("persistence: invalid transition, ignoring")
		return nil
	}
	if err := h.walletWriter.OnFailed(msg.ScanID, msg.UserID, msg.Address, msg.Error); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: OnFailed wallet failed")
		return err
	}
	if err := h.redisCache.SaveWalletFailure(ctx, msg.UserID, msg.Address, msg.Error); err != nil {
		log.Error().Err(err).Str("scan_id", msg.ScanID.String()).Msg("persistence: Redis wallet failure write failed")
		return err
	}
	h.publishScanReady(msg.UserID, "wallet", "", msg.Address, "failed")
	return nil
}

func (h *ScanEventHandler) publishScanReady(userID uuid.UUID, kind, endpoint, address, status string) {
	if h.natsConn == nil {
		return
	}
	ready := nats.ScanReadyMessage{
		UserID: userID, Kind: kind, Endpoint: endpoint, Address: address, Status: status,
	}
	if err := nats.PublishJSON(h.natsConn, nats.SubjectScanReady, ready); err != nil {
		log.Warn().Err(err).Str("kind", kind).Str("status", status).Msg("persistence: scan.ready publish failed")
	}
}

// Ensure valid state transitions (idempotency: same event twice is safe)
var _ = []interface{}{
	scan.StatePENDING,
	scan.StateRUNNING,
	scan.StateSUCCESS,
	scan.StateFAILED,
	scan.StateTIMEOUT,
	scan.StateUNREACHABLE,
}
