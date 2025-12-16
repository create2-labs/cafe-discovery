package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Blockchain represents a blockchain network configuration
type Blockchain struct {
	Name             string `yaml:"name"`
	RPC              string `yaml:"rpc"`
	MoralisChainName string `yaml:"moralis_chain_name"`
}

// Config represents the application configuration
type ChainConfig struct {
	Blockchains []Blockchain `yaml:"blockchains"`
}

// Load reads and parses the configuration file
func LoadChainConfig(configPath string) (*ChainConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ChainConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}
