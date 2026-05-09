package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cafe-discovery/internal/authz"
	"cafe-discovery/internal/repository"

	"github.com/google/uuid"
)

// ScanAuthorizationService implements the authoritative scan-visibility check
// consumed by CPM through the AUTH-05 internal endpoint.
//
// The service inspects the wallet and TLS scan repositories to determine
// whether the propagated principal owns or otherwise has visibility on the
// scan identified by scanID. It deliberately does not return scan metadata:
// the caller (CPM) must not learn the owner, address, endpoint, tenant, or
// any other attribute of the scan from the deny path.
type ScanAuthorizationService struct {
	walletScans repository.ScanResultRepository
	tlsScans    repository.TLSScanResultRepository
}

// NewScanAuthorizationService wires the decision service against the wallet
// and TLS scan repositories. Both repositories are required.
func NewScanAuthorizationService(
	walletScans repository.ScanResultRepository,
	tlsScans repository.TLSScanResultRepository,
) *ScanAuthorizationService {
	return &ScanAuthorizationService{
		walletScans: walletScans,
		tlsScans:    tlsScans,
	}
}

// CanReadScan returns the authoritative authorization decision for the
// principal and scanID. The function never returns scan metadata in the
// returned Decision; the only observable is the binary allow/deny outcome
// and the reason code.
//
// Returned errors indicate that the decision could not be resolved (e.g.
// repository failure). The caller is expected to map errors to a 5xx
// response so CPM can fail closed.
func (s *ScanAuthorizationService) CanReadScan(ctx context.Context, principal authz.Principal, scanID string) (authz.Decision, error) {
	if s == nil {
		return authz.Decision{}, errors.New("scan authorization service is not initialized")
	}
	if err := principal.Validate(); err != nil {
		return authz.Decision{
			Allowed:    false,
			ReasonCode: authz.ReasonCodePrincipalRequired,
		}, nil
	}
	if !authz.IsValidScanID(scanID) {
		return authz.Decision{
			Allowed:    false,
			ReasonCode: authz.ReasonCodeScanIDMalformed,
		}, nil
	}
	scanUUID, err := uuid.Parse(strings.TrimSpace(scanID))
	if err != nil {
		// Defensive: IsValidScanID already validated the format. Treat any
		// late parse failure as malformed rather than as a server error.
		return authz.Decision{
			Allowed:    false,
			ReasonCode: authz.ReasonCodeScanIDMalformed,
		}, nil
	}

	principalUserID, principalUserIDOK := parsePrincipalUserID(principal.UserID)

	// Wallet scan path: try to resolve the scan id as a wallet scan first.
	walletEntity, walletErr := s.walletScans.FindByID(scanUUID)
	if walletErr != nil {
		return authz.Decision{}, fmt.Errorf("scan authorization: wallet scan lookup: %w", walletErr)
	}
	if walletEntity != nil {
		if !principalUserIDOK {
			// The principal user id is not a UUID; Discovery's wallet model
			// uses UUID-keyed users, so it cannot match. Treat as forbidden
			// rather than leak that the scan exists.
			return authz.Decision{
				Allowed:    false,
				ReasonCode: authz.ReasonCodeForbidden,
			}, nil
		}
		if walletEntity.UserID == principalUserID {
			return authz.Decision{
				Allowed:    true,
				ReasonCode: authz.ReasonCodeAllowed,
			}, nil
		}
		return authz.Decision{
			Allowed:    false,
			ReasonCode: authz.ReasonCodeForbidden,
		}, nil
	}

	// TLS scan path: same scanID space; default endpoints have a NULL user id
	// and are visible to any authenticated principal.
	tlsEntity, tlsErr := s.tlsScans.FindByID(scanUUID)
	if tlsErr != nil {
		return authz.Decision{}, fmt.Errorf("scan authorization: tls scan lookup: %w", tlsErr)
	}
	if tlsEntity != nil {
		if tlsEntity.Default {
			return authz.Decision{
				Allowed:    true,
				ReasonCode: authz.ReasonCodeAllowed,
			}, nil
		}
		if tlsEntity.UserID == nil {
			// Non-default scans without an owner are not visible to anyone
			// from CPM's perspective.
			return authz.Decision{
				Allowed:    false,
				ReasonCode: authz.ReasonCodeForbidden,
			}, nil
		}
		if !principalUserIDOK {
			return authz.Decision{
				Allowed:    false,
				ReasonCode: authz.ReasonCodeForbidden,
			}, nil
		}
		if *tlsEntity.UserID == principalUserID {
			return authz.Decision{
				Allowed:    true,
				ReasonCode: authz.ReasonCodeAllowed,
			}, nil
		}
		return authz.Decision{
			Allowed:    false,
			ReasonCode: authz.ReasonCodeForbidden,
		}, nil
	}

	// No scan exists for this id. Return NOT_VISIBLE to align with CPM
	// AUTH-02; the response remains 403 (anti-enumeration 404 hardening is
	// out of scope for this PR).
	return authz.Decision{
		Allowed:    false,
		ReasonCode: authz.ReasonCodeNotVisible,
	}, nil
}

func parsePrincipalUserID(raw string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, false
	}
	return parsed, true
}
