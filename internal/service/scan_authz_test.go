package service

import (
	"context"
	"errors"
	"testing"

	"cafe-discovery/internal/authz"
	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
)

// stubScanResultRepository is a minimal in-memory ScanResultRepository used
// to drive ScanAuthorizationService tests without touching Postgres.
type stubScanResultRepository struct {
	byID    map[uuid.UUID]*domain.ScanResultEntity
	findErr error
}

func (s *stubScanResultRepository) Create(*domain.ScanResultEntity) error {
	return errors.New("not implemented")
}

func (s *stubScanResultRepository) FindByUserID(uuid.UUID, int, int) ([]*domain.ScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubScanResultRepository) FindByID(id uuid.UUID) (*domain.ScanResultEntity, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if entity, ok := s.byID[id]; ok {
		return entity, nil
	}
	return nil, nil
}

func (s *stubScanResultRepository) FindByUserIDAndAddress(uuid.UUID, string) (*domain.ScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubScanResultRepository) FindOwnedWalletScanByID(uuid.UUID, uuid.UUID) (*domain.ScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubScanResultRepository) ListOwnerWalletScansDiscoveryV1(uuid.UUID, string, int, int) ([]*domain.ScanResultEntity, int64, error) {
	return nil, 0, errors.New("not implemented")
}

func (s *stubScanResultRepository) CountByUserID(uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubScanResultRepository) DeleteOwnedWalletScan(uuid.UUID, uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

type stubTLSScanResultRepository struct {
	byID    map[uuid.UUID]*domain.TLSScanResultEntity
	findErr error
}

func (s *stubTLSScanResultRepository) Create(*domain.TLSScanResultEntity) error {
	return errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) FindByUserID(uuid.UUID, int, int) ([]*domain.TLSScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) FindByUserIDOrDefault(uuid.UUID, int, int) ([]*domain.TLSScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) FindByID(id uuid.UUID) (*domain.TLSScanResultEntity, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if entity, ok := s.byID[id]; ok {
		return entity, nil
	}
	return nil, nil
}

func (s *stubTLSScanResultRepository) FindDefaultTLSScanByID(uuid.UUID) (*domain.TLSScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) FindOwnedUserTLSScanByID(uuid.UUID, uuid.UUID) (*domain.TLSScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) ListOwnerUserTLSScansDiscoveryV1(uuid.UUID, int, int) ([]*domain.TLSScanResultEntity, int64, error) {
	return nil, 0, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) FindByUserIDAndURL(uuid.UUID, string) (*domain.TLSScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) FindByURL(string) (*domain.TLSScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) FindDefaultByURL(string) (*domain.TLSScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) FindAllDefault() ([]*domain.TLSScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) CountByUserID(uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) CountByUserIDOrDefault(uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *stubTLSScanResultRepository) DeleteOwnedUserTLSScan(uuid.UUID, uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

func TestCanReadScan_AllowedOwner_Wallet(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	scanID := uuid.New()
	wallets := &stubScanResultRepository{
		byID: map[uuid.UUID]*domain.ScanResultEntity{
			scanID: {ID: scanID, UserID: owner},
		},
	}
	tls := &stubTLSScanResultRepository{}
	svc := NewScanAuthorizationService(wallets, tls)

	decision, err := svc.CanReadScan(context.Background(), authz.Principal{UserID: owner.String()}, scanID.String())
	if err != nil {
		t.Fatalf("CanReadScan returned unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allowed decision for scan owner, got %+v", decision)
	}
	if decision.ReasonCode != authz.ReasonCodeAllowed {
		t.Fatalf("expected reason code %q, got %q", authz.ReasonCodeAllowed, decision.ReasonCode)
	}
}

func TestCanReadScan_CrossUserDenied_Wallet(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	intruder := uuid.New()
	scanID := uuid.New()
	wallets := &stubScanResultRepository{
		byID: map[uuid.UUID]*domain.ScanResultEntity{
			scanID: {ID: scanID, UserID: owner},
		},
	}
	tls := &stubTLSScanResultRepository{}
	svc := NewScanAuthorizationService(wallets, tls)

	decision, err := svc.CanReadScan(context.Background(), authz.Principal{UserID: intruder.String()}, scanID.String())
	if err != nil {
		t.Fatalf("CanReadScan returned unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatalf("expected deny decision for cross-user access, got %+v", decision)
	}
	if decision.ReasonCode != authz.ReasonCodeForbidden {
		t.Fatalf("expected reason code %q, got %q", authz.ReasonCodeForbidden, decision.ReasonCode)
	}
}

func TestCanReadScan_AllowedOwner_TLS(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	scanID := uuid.New()
	wallets := &stubScanResultRepository{}
	tls := &stubTLSScanResultRepository{
		byID: map[uuid.UUID]*domain.TLSScanResultEntity{
			scanID: {ID: scanID, UserID: &owner},
		},
	}
	svc := NewScanAuthorizationService(wallets, tls)

	decision, err := svc.CanReadScan(context.Background(), authz.Principal{UserID: owner.String()}, scanID.String())
	if err != nil {
		t.Fatalf("CanReadScan returned unexpected error: %v", err)
	}
	if !decision.Allowed || decision.ReasonCode != authz.ReasonCodeAllowed {
		t.Fatalf("expected allowed for TLS scan owner, got %+v", decision)
	}
}

func TestCanReadScan_DefaultTLSScan_AllowedForAnyPrincipal(t *testing.T) {
	t.Parallel()

	scanID := uuid.New()
	wallets := &stubScanResultRepository{}
	tls := &stubTLSScanResultRepository{
		byID: map[uuid.UUID]*domain.TLSScanResultEntity{
			scanID: {ID: scanID, UserID: nil, Default: true},
		},
	}
	svc := NewScanAuthorizationService(wallets, tls)

	decision, err := svc.CanReadScan(context.Background(), authz.Principal{UserID: uuid.New().String()}, scanID.String())
	if err != nil {
		t.Fatalf("CanReadScan returned unexpected error: %v", err)
	}
	if !decision.Allowed || decision.ReasonCode != authz.ReasonCodeAllowed {
		t.Fatalf("expected allowed for default TLS scan, got %+v", decision)
	}
}

func TestCanReadScan_UnknownScanReturnsNotVisible(t *testing.T) {
	t.Parallel()

	svc := NewScanAuthorizationService(&stubScanResultRepository{}, &stubTLSScanResultRepository{})
	decision, err := svc.CanReadScan(context.Background(), authz.Principal{UserID: uuid.New().String()}, uuid.New().String())
	if err != nil {
		t.Fatalf("CanReadScan returned unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatalf("expected deny for unknown scan, got %+v", decision)
	}
	if decision.ReasonCode != authz.ReasonCodeNotVisible {
		t.Fatalf("expected reason code %q, got %q", authz.ReasonCodeNotVisible, decision.ReasonCode)
	}
}

func TestCanReadScan_MalformedScanIDIsDecision(t *testing.T) {
	t.Parallel()

	svc := NewScanAuthorizationService(&stubScanResultRepository{}, &stubTLSScanResultRepository{})
	decision, err := svc.CanReadScan(context.Background(), authz.Principal{UserID: uuid.New().String()}, "not-a-uuid")
	if err != nil {
		t.Fatalf("CanReadScan returned unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatalf("expected deny for malformed scan id, got %+v", decision)
	}
	if decision.ReasonCode != authz.ReasonCodeScanIDMalformed {
		t.Fatalf("expected reason code %q, got %q", authz.ReasonCodeScanIDMalformed, decision.ReasonCode)
	}
}

func TestCanReadScan_MissingPrincipalReturnsRequired(t *testing.T) {
	t.Parallel()

	svc := NewScanAuthorizationService(&stubScanResultRepository{}, &stubTLSScanResultRepository{})
	decision, err := svc.CanReadScan(context.Background(), authz.Principal{}, uuid.New().String())
	if err != nil {
		t.Fatalf("CanReadScan returned unexpected error: %v", err)
	}
	if decision.ReasonCode != authz.ReasonCodePrincipalRequired {
		t.Fatalf("expected reason code %q, got %q", authz.ReasonCodePrincipalRequired, decision.ReasonCode)
	}
}

func TestCanReadScan_RepositoryErrorPropagates(t *testing.T) {
	t.Parallel()

	wallets := &stubScanResultRepository{findErr: errors.New("postgres exploded")}
	tls := &stubTLSScanResultRepository{}
	svc := NewScanAuthorizationService(wallets, tls)

	_, err := svc.CanReadScan(context.Background(), authz.Principal{UserID: uuid.New().String()}, uuid.New().String())
	if err == nil {
		t.Fatalf("expected error when repository fails, got nil")
	}
}

// TestCanReadScan_TenantScopingNotEnforcedYet documents the current Discovery
// scan model: it has no tenant_id column, therefore the tenant header on the
// principal does not change the outcome. CPM still propagates X-Tenant-Id
// for traceability and future enforcement.
//
// TODO(auth-05-tenant): once Discovery scans expose tenant_id, deny when
// principal.tenant_id != scan.tenant_id.
func TestCanReadScan_TenantScopingNotEnforcedYet(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	scanID := uuid.New()
	wallets := &stubScanResultRepository{
		byID: map[uuid.UUID]*domain.ScanResultEntity{
			scanID: {ID: scanID, UserID: owner},
		},
	}
	tls := &stubTLSScanResultRepository{}
	svc := NewScanAuthorizationService(wallets, tls)

	matching, err := svc.CanReadScan(context.Background(), authz.Principal{UserID: owner.String(), TenantID: "tenant-a"}, scanID.String())
	if err != nil {
		t.Fatalf("matching tenant CanReadScan: %v", err)
	}
	if !matching.Allowed {
		t.Fatalf("matching tenant must currently be allowed, got %+v", matching)
	}

	differing, err := svc.CanReadScan(context.Background(), authz.Principal{UserID: owner.String(), TenantID: "tenant-b"}, scanID.String())
	if err != nil {
		t.Fatalf("differing tenant CanReadScan: %v", err)
	}
	if !differing.Allowed {
		t.Fatalf("tenant scoping is not yet part of the Discovery scan model; a different tenant must not change the outcome until the model is extended (see AUTH-05 TODO). got %+v", differing)
	}
}
