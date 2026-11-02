package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"cafe-discovery/internal/address"
	"cafe-discovery/pkg/scan"

	lifecyclev1 "github.com/create2-labs/cafe-contracts/discovery/scanlifecycle/v1"
	"github.com/google/uuid"
)

const (
	walletQuotaNone    = "none"
	walletQuotaKeep    = "keep"
	walletQuotaRelease = "release"
)

var (
	// ErrInvalidWalletEvent is a lifecycle payload that cannot be decoded,
	// or a wallet event whose address is not a valid Ethereum address.
	ErrInvalidWalletEvent = errors.New("invalid wallet lifecycle event")
	// ErrNotWalletEvent is a decoded lifecycle event whose kind is not wallet.
	ErrNotWalletEvent = errors.New("lifecycle event is not wallet")
)

// walletCommand is the JSON body of ApplyWalletTransition.
// ScanID is the path identifier and is not part of that body.
type walletCommand struct {
	ScanID       uuid.UUID       `json:"-"`
	TransitionID string          `json:"transition_id"`
	UserID       uuid.UUID       `json:"user_id"`
	Address      string          `json:"address"`
	SourceStates []string        `json:"source_states"`
	TargetState  string          `json:"target_state"`
	QuotaAction  string          `json:"quota_action"`
	Result       json.RawMessage `json:"result,omitempty"`
	Error        string          `json:"error,omitempty"`
}

type walletEventIdentity struct {
	scanID  uuid.UUID
	userID  uuid.UUID
	address string
}

// WalletTransitionCommand maps one lifecycle payload to a transition command.
// It does not call HTTP, acknowledge a message, or read pending or wallet state.
// The same inputs return the same command.
func WalletTransitionCommand(subject string, payload []byte, stream string, sequence uint64) (walletCommand, error) {
	switch subject {
	case lifecyclev1.SubjectScanStarted:
		msg, err := lifecyclev1.DecodeStarted(payload)
		if err != nil {
			return walletCommand{}, invalidWalletEvent(err)
		}
		return startedCommand(msg, stream, sequence)
	case lifecyclev1.SubjectScanCompleted:
		msg, err := lifecyclev1.DecodeCompleted(payload)
		if err != nil {
			return walletCommand{}, invalidWalletEvent(err)
		}
		return completedCommand(msg, stream, sequence)
	case lifecyclev1.SubjectScanFailed:
		msg, err := lifecyclev1.DecodeFailed(payload)
		if err != nil {
			return walletCommand{}, invalidWalletEvent(err)
		}
		return failedCommand(msg, stream, sequence)
	default:
		return walletCommand{}, invalidWalletEvent(fmt.Errorf("subject %s", subject))
	}
}

func startedCommand(msg lifecyclev1.Started, stream string, sequence uint64) (walletCommand, error) {
	id, err := walletIdentity(msg.Kind, msg.ScanID, msg.UserID, msg.Address)
	if err != nil {
		return walletCommand{}, err
	}
	return id.command(stream, sequence, []string{scan.StatePENDING}, scan.StateRUNNING, walletQuotaNone), nil
}

func completedCommand(msg lifecyclev1.Completed, stream string, sequence uint64) (walletCommand, error) {
	id, err := walletIdentity(msg.Kind, msg.ScanID, msg.UserID, msg.Address)
	if err != nil {
		return walletCommand{}, err
	}
	cmd := id.command(stream, sequence, []string{scan.StatePENDING, scan.StateRUNNING}, scan.StateSUCCESS, walletQuotaKeep)
	cmd.Result = cloneRaw(msg.Result)
	return cmd, nil
}

func failedCommand(msg lifecyclev1.Failed, stream string, sequence uint64) (walletCommand, error) {
	id, err := walletIdentity(msg.Kind, msg.ScanID, msg.UserID, msg.Address)
	if err != nil {
		return walletCommand{}, err
	}
	cmd := id.command(stream, sequence, []string{scan.StatePENDING, scan.StateRUNNING}, scan.StateFAILED, walletQuotaRelease)
	cmd.Error = msg.Error
	return cmd, nil
}

func walletIdentity(kind, scanID, userID, rawAddress string) (walletEventIdentity, error) {
	if kind != scan.KindWallet {
		return walletEventIdentity{}, ErrNotWalletEvent
	}
	sid, err := uuid.Parse(scanID)
	if err != nil {
		return walletEventIdentity{}, invalidWalletEvent(err)
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return walletEventIdentity{}, invalidWalletEvent(err)
	}
	normalized, err := address.ValidateAndNormalizeAddress(rawAddress)
	if err != nil {
		return walletEventIdentity{}, invalidWalletEvent(err)
	}
	return walletEventIdentity{scanID: sid, userID: uid, address: normalized}, nil
}

func (id walletEventIdentity) command(stream string, sequence uint64, sources []string, target, quota string) walletCommand {
	return walletCommand{
		ScanID:       id.scanID,
		TransitionID: stream + ":" + strconv.FormatUint(sequence, 10),
		UserID:       id.userID,
		Address:      id.address,
		SourceStates: append([]string(nil), sources...),
		TargetState:  target,
		QuotaAction:  quota,
	}
}

func invalidWalletEvent(err error) error {
	return fmt.Errorf("%w: %v", ErrInvalidWalletEvent, err)
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}
