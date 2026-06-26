package handler

import (
	"context"
	"net/url"
	"strconv"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/persistence/scanread"

	"github.com/google/uuid"
)

// RepoScanReadStub adapts ScanResultRepository read methods for handler tests (PERS-D6a-read).
type RepoScanReadStub struct {
	repo scanResultReader
	cfg  *config.ChainConfig
}

type scanResultReader interface {
	FindOwnedWalletScanByID(userID, scanID uuid.UUID) (*domain.ScanResultEntity, error)
	ListOwnerWalletScansDiscoveryV1(userID uuid.UUID, address string, limit, offset int) ([]*domain.ScanResultEntity, int64, error)
	ListOwnerWalletScansByAddress(userID uuid.UUID, address string) ([]*domain.ScanResultEntity, error)
}

var _ scanread.Store = (*RepoScanReadStub)(nil)

// NewRepoScanReadStub returns a scanread.Store backed by an in-memory or sqlite ScanResultRepository stub.
func NewRepoScanReadStub(repo scanResultReader, cfg *config.ChainConfig) scanread.Store {
	return &RepoScanReadStub{repo: repo, cfg: cfg}
}

func (s *RepoScanReadStub) ListWalletScans(_ context.Context, userID uuid.UUID, _ string, query url.Values) ([]*domain.ScanResultEntity, int64, int, int, error) {
	limit, _ := strconv.Atoi(query.Get("limit"))
	offset, _ := strconv.Atoi(query.Get("offset"))
	if limit <= 0 {
		limit = 20
	}
	addr := query.Get("address")
	latest := query.Get("latest") == "true"
	chainQ := query.Get("chain_id")

	if latest && addr != "" {
		all, err := s.repo.ListOwnerWalletScansByAddress(userID, addr)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		for _, ent := range all {
			if walletEntityIsCompleted(ent) {
				if chainQ != "" {
					var chainID int64
					if v, perr := strconv.ParseInt(chainQ, 10, 64); perr == nil {
						chainID = v
						if !walletEntityMatchesChainID(ent, chainID, s.cfg) {
							continue
						}
					}
				}
				return []*domain.ScanResultEntity{ent}, 1, limit, offset, nil
			}
		}
		return nil, 0, limit, offset, nil
	}
	if addr != "" && chainQ != "" {
		all, err := s.repo.ListOwnerWalletScansByAddress(userID, addr)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		chainID, _ := strconv.ParseInt(chainQ, 10, 64)
		filtered := make([]*domain.ScanResultEntity, 0, len(all))
		for _, ent := range all {
			if walletEntityMatchesChainID(ent, chainID, s.cfg) {
				filtered = append(filtered, ent)
			}
		}
		return paginateWalletScanEntities(filtered, limit, offset), int64(len(filtered)), limit, offset, nil
	}
	entities, total, err := s.repo.ListOwnerWalletScansDiscoveryV1(userID, addr, limit, offset)
	return entities, total, limit, offset, err
}

func (s *RepoScanReadStub) GetWalletScan(_ context.Context, userID uuid.UUID, _ string, scanID uuid.UUID) (*domain.ScanResultEntity, error) {
	return s.repo.FindOwnedWalletScanByID(userID, scanID)
}

func (s *RepoScanReadStub) ListTLSScans(context.Context, uuid.UUID, string, int, int) ([]*domain.TLSScanResultEntity, int64, error) {
	return nil, 0, nil
}

func (s *RepoScanReadStub) ListTLSDefaultScans(context.Context, uuid.UUID, string) ([]*domain.TLSScanResultEntity, error) {
	return nil, nil
}

func (s *RepoScanReadStub) GetTLSScan(context.Context, uuid.UUID, string, uuid.UUID) (*domain.TLSScanResultEntity, error) {
	return nil, nil
}
