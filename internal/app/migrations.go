package app

import (
	"fmt"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	postgresdb "cafe-discovery/pkg/postgres"
)

// scanTablesOwnedByPersistence lists Postgres tables whose DDL is owned by cafe-persistence (PERS-D2b).
var scanTablesOwnedByPersistence = []string{
	"scan_results",
	"tls_scan_results",
	"scan_usage_events",
}

// runMigrations runs identity-plane database migrations only.
// Scan table DDL is owned by cafe-persistence; see ensureScanTablesPresent.
func runMigrations(db postgresdb.PostgreSQLConnection) error {
	if err := db.GetDB().AutoMigrate(
		&domain.Plan{},
		&domain.User{},
		&domain.CafeWallet{},
	); err != nil {
		return err
	}

	planRepo := repository.NewPlanRepository(db.GetDB())

	freePlan, err := ensurePlanExists(planRepo, domain.PlanTypeFree, &domain.Plan{
		Name:              "Free Plan",
		Type:              domain.PlanTypeFree,
		WalletScanLimit:   5,
		EndpointScanLimit: 5,
		Price:             0,
		IsActive:          true,
	})
	if err != nil {
		return err
	}

	_, err = ensurePlanExists(planRepo, domain.PlanTypePremium, &domain.Plan{
		Name:              "CAFEIN Premium Plan",
		Type:              domain.PlanTypePremium,
		WalletScanLimit:   0,
		EndpointScanLimit: 0,
		Price:             29.99,
		IsActive:          false,
	})
	if err != nil {
		return err
	}

	if err := assignPlanToUsersWithoutPlan(db, freePlan); err != nil {
		return err
	}

	return ensureScanTablesPresent(db)
}

// ensureScanTablesPresent verifies scan tables exist before serving scan API traffic.
// cafe-persistence must run MigrateScanSchema before the backend starts (compose/orchestrator gate).
func ensureScanTablesPresent(db postgresdb.PostgreSQLConnection) error {
	migrator := db.GetDB().Migrator()
	for _, table := range scanTablesOwnedByPersistence {
		if !migrator.HasTable(table) {
			return fmt.Errorf("scan table %q missing: start cafe-persistence first (PERS-D2b boot order)", table)
		}
	}
	return nil
}
