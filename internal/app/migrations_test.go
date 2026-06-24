package app

import (
	"testing"

	"cafe-discovery/internal/domain"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunMigrationsIdentityOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(
		&domain.Plan{},
		&domain.User{},
		&domain.CafeWallet{},
		&domain.ScanResultEntity{},
		&domain.TLSScanResultEntity{},
		&domain.ScanUsageEventEntity{},
	); err != nil {
		t.Fatalf("pre-migrate scan tables: %v", err)
	}

	conn := &sqliteConn{db: db}
	if err := runMigrations(conn); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	for _, table := range []string{"plans", "users", "cafe_wallets"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected identity table %q after runMigrations", table)
		}
	}
}

func TestEnsureScanTablesPresentFailsWhenMissing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.Plan{}); err != nil {
		t.Fatalf("migrate plan: %v", err)
	}

	conn := &sqliteConn{db: db}
	if err := ensureScanTablesPresent(conn); err == nil {
		t.Fatal("ensureScanTablesPresent should fail when scan tables are absent")
	}
}

type sqliteConn struct {
	db *gorm.DB
}

func (s *sqliteConn) GetDB() *gorm.DB { return s.db }

func (s *sqliteConn) IsConnected() bool { return s.db != nil }

func (s *sqliteConn) Run() error { return nil }

func (s *sqliteConn) Shutdown() {}
