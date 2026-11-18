package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestChainConfig_Validate_ok(t *testing.T) {
	t.Parallel()
	c := &ChainConfig{
		Blockchains: []Blockchain{
			{Name: "ethereum-mainnet", ChainID: 1},
			{Name: "base", ChainID: 8453},
		},
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestChainConfig_Validate_empty(t *testing.T) {
	t.Parallel()
	const want = "blockchains: at least one chain is required"
	tests := []struct {
		name string
		cfg  *ChainConfig
	}{
		{"absent", &ChainConfig{}},
		{"empty", &ChainConfig{Blockchains: []Blockchain{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if err == nil || err.Error() != want {
				t.Fatalf("got %v, want %q", err, want)
			}
		})
	}
}

// CA2.5: the chain set is the configured set, in configuration order.
func TestChainConfig_ChainIDs(t *testing.T) {
	t.Parallel()
	c := &ChainConfig{
		Blockchains: []Blockchain{
			{Name: "a", ChainID: 10},
			{Name: "b", ChainID: 1},
			{Name: "c", ChainID: 137},
		},
	}
	got := c.ChainIDs()
	want := []int64{10, 1, 137}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestChainConfig_Validate_duplicateName(t *testing.T) {
	t.Parallel()
	c := &ChainConfig{
		Blockchains: []Blockchain{
			{Name: "base", ChainID: 1},
			{Name: "base", ChainID: 2},
		},
	}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), `duplicate name`) {
		t.Fatalf("got %v", err)
	}
}

func TestChainConfig_Validate_duplicateChainID(t *testing.T) {
	t.Parallel()
	c := &ChainConfig{
		Blockchains: []Blockchain{
			{Name: "net-a", ChainID: 1},
			{Name: "net-b", ChainID: 1},
		},
	}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), `duplicate chain_id`) {
		t.Fatalf("got %v", err)
	}
}

func TestChainConfig_Validate_badChainID(t *testing.T) {
	t.Parallel()
	c := &ChainConfig{
		Blockchains: []Blockchain{
			{Name: "x", ChainID: 0},
		},
	}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), `chain_id`) {
		t.Fatalf("got %v", err)
	}
}

func TestLoadChainConfig_repoConfigYAML(t *testing.T) {
	t.Parallel()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	cfgPath := filepath.Join(repoRoot, "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		cfgPath = filepath.Join(repoRoot, "config.local-example.yaml")
	}

	cfg, err := LoadChainConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Blockchains) < 1 {
		t.Fatal("expected blockchains in config.yaml")
	}
	if cfg.ChainIDByNetwork()["ethereum-mainnet"] != 1 {
		t.Fatalf("unexpected mapping: %#v", cfg.ChainIDByNetwork())
	}
}

func TestChainConfig_ChainIDByNetwork(t *testing.T) {
	t.Parallel()
	c := &ChainConfig{
		Blockchains: []Blockchain{
			{Name: "base", ChainID: 8453},
		},
	}
	m := c.ChainIDByNetwork()
	if m["base"] != 8453 || len(m) != 1 {
		t.Fatalf("got %#v", m)
	}
}

func TestLoadChainConfigIgnoresUnreadKeys(t *testing.T) {
	t.Parallel()
	const doc = `blockchains:
  - name: base
    rpc: "https://base-rpc.example"
    moralis_chain_name: "base"
    supports_eip7702: true
    chain_id: 8453
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadChainConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Blockchains) != 1 {
		t.Fatalf("got %d blockchains, want 1", len(cfg.Blockchains))
	}
	got := cfg.Blockchains[0]
	want := Blockchain{Name: "base", RPC: "https://base-rpc.example", ChainID: 8453}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
