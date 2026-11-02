package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"cafe-discovery/pkg/scan"

	lifecyclev1 "github.com/create2-labs/cafe-contracts/discovery/scanlifecycle/v1"
	"github.com/google/uuid"
)

const (
	walletCommandScanID = "6f1c0c1e-9b2a-4d3e-8f7a-1c2b3d4e5f60"
	walletCommandUserID = "11111111-1111-4111-8111-111111111111"
	walletCommandStream = "SCAN_LIFECYCLE"
	walletCommandWhen   = "2026-10-20T12:00:00Z"
	walletCommandAddrIn = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	walletCommandAddr   = "0x742d35cc6634c0532925a3b844bc454e4438f44e"
	walletCommandSeq    = uint64(12345)
	walletCommandResult = `{"type":"unknown","address":"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}`
)

func TestWalletCommandStarted(t *testing.T) {
	t.Parallel()
	payload := lifecyclePayload(t, map[string]any{
		"scan_id": walletCommandScanID, "kind": scan.KindWallet, "user_id": walletCommandUserID,
		"started_at": walletCommandWhen, "address": walletCommandAddrIn,
	})
	cmd := mustWalletCommand(t, lifecyclev1.SubjectScanStarted, payload)
	again := mustWalletCommand(t, lifecyclev1.SubjectScanStarted, payload)
	if !reflect.DeepEqual(cmd, again) {
		t.Fatalf("same inputs = %#v and %#v", cmd, again)
	}
	want := walletCommand{
		ScanID: uuid.MustParse(walletCommandScanID), TransitionID: "SCAN_LIFECYCLE:12345",
		UserID: uuid.MustParse(walletCommandUserID), Address: walletCommandAddr,
		SourceStates: []string{scan.StatePENDING}, TargetState: scan.StateRUNNING, QuotaAction: walletQuotaNone,
	}
	if !reflect.DeepEqual(cmd, want) {
		t.Fatalf("command = %#v, want %#v", cmd, want)
	}
	assertAbsent(t, cmd, "result", "error")
}

func TestWalletCommandCompletedKeepsUnknownResult(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"scan_id":"` + walletCommandScanID + `","kind":"wallet","user_id":"` + walletCommandUserID +
		`","completed_at":"` + walletCommandWhen + `","address":"` + walletCommandAddrIn + `","result":` + walletCommandResult + `}`)
	cmd := mustWalletCommand(t, lifecyclev1.SubjectScanCompleted, payload)
	if cmd.TargetState != scan.StateSUCCESS || cmd.QuotaAction != walletQuotaKeep {
		t.Fatalf("target %s quota %s", cmd.TargetState, cmd.QuotaAction)
	}
	if !slices.Equal(cmd.SourceStates, []string{scan.StatePENDING, scan.StateRUNNING}) {
		t.Fatalf("sources = %v", cmd.SourceStates)
	}
	if cmd.Error != "" || !bytes.Equal(cmd.Result, []byte(walletCommandResult)) {
		t.Fatalf("error %q result %s", cmd.Error, cmd.Result)
	}
	if cmd.Address != walletCommandAddr {
		t.Fatalf("address = %s", cmd.Address)
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(walletCommandResult)) {
		t.Fatalf("marshaled result changed: %s", raw)
	}
	assertAbsent(t, cmd, "error")
}

func TestWalletCommandFailed(t *testing.T) {
	t.Parallel()
	payload := lifecyclePayload(t, map[string]any{
		"scan_id": walletCommandScanID, "kind": scan.KindWallet, "user_id": walletCommandUserID,
		"error": "indexer timeout", "completed_at": walletCommandWhen, "address": walletCommandAddrIn,
	})
	cmd := mustWalletCommand(t, lifecyclev1.SubjectScanFailed, payload)
	if cmd.TargetState != scan.StateFAILED || cmd.QuotaAction != walletQuotaRelease || cmd.Error != "indexer timeout" {
		t.Fatalf("command = %#v", cmd)
	}
	if !slices.Equal(cmd.SourceStates, []string{scan.StatePENDING, scan.StateRUNNING}) || len(cmd.Result) != 0 {
		t.Fatalf("sources %v result %s", cmd.SourceStates, cmd.Result)
	}
	if cmd.TransitionID != walletCommandStream+":12345" {
		t.Fatalf("transition_id = %s", cmd.TransitionID)
	}
	assertAbsent(t, cmd, "result")
}

func TestWalletCommandInvalid(t *testing.T) {
	t.Parallel()
	started := lifecyclePayload(t, map[string]any{
		"scan_id": walletCommandScanID, "kind": scan.KindWallet, "user_id": walletCommandUserID,
		"started_at": walletCommandWhen, "address": "0x1234",
	})
	cases := []struct {
		name    string
		subject string
		payload []byte
	}{
		{name: "malformed", subject: lifecyclev1.SubjectScanStarted, payload: []byte("{")},
		{name: "undecodable", subject: lifecyclev1.SubjectScanStarted, payload: []byte(`{"kind":"wallet"}`)},
		{name: "bad address", subject: lifecyclev1.SubjectScanStarted, payload: started},
		{name: "unknown subject", subject: lifecyclev1.SubjectScanReady, payload: []byte(`{"kind":"tls"}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd, err := WalletTransitionCommand(tc.subject, tc.payload, walletCommandStream, walletCommandSeq)
			if !reflect.DeepEqual(cmd, walletCommand{}) {
				t.Fatalf("command = %#v", cmd)
			}
			if !errors.Is(err, ErrInvalidWalletEvent) || errors.Is(err, ErrNotWalletEvent) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestWalletCommandNotWallet(t *testing.T) {
	t.Parallel()
	payload := lifecyclePayload(t, map[string]any{
		"scan_id": walletCommandScanID, "kind": scan.KindTLS, "user_id": walletCommandUserID,
		"started_at": walletCommandWhen, "endpoint": "example.com",
	})
	cmd, err := WalletTransitionCommand(lifecyclev1.SubjectScanStarted, payload, walletCommandStream, walletCommandSeq)
	if !reflect.DeepEqual(cmd, walletCommand{}) {
		t.Fatalf("command = %#v", cmd)
	}
	if !errors.Is(err, ErrNotWalletEvent) || errors.Is(err, ErrInvalidWalletEvent) {
		t.Fatalf("error = %v", err)
	}
}

func mustWalletCommand(t *testing.T, subject string, payload []byte) walletCommand {
	t.Helper()
	cmd, err := WalletTransitionCommand(subject, payload, walletCommandStream, walletCommandSeq)
	if err != nil {
		t.Fatalf("command: %v", err)
	}
	return cmd
}

func lifecyclePayload(t *testing.T, fields map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertAbsent(t *testing.T, cmd walletCommand, keys ...string) {
	t.Helper()
	raw, err := json.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if _, ok := doc[key]; ok {
			t.Fatalf("body has %s: %s", key, raw)
		}
	}
}
