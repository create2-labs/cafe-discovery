package main

/*
This command is used to get the public key from a transaction data.
It uses the Moralis API to get the transaction data and recover the public key.

It uses the following environment variables:
- MORALIS_API_KEY
- MORALIS_API_URL

It uses the following command line arguments:
- address: the address to get the public key from
- network: the network to get the public key from
*/

import (
	"context"
	"log"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"

	"github.com/spf13/viper"
)

func initConfig() {
	for configName, defaultValue := range config.GetDefaultConfigValues() {
		viper.SetDefault(configName, defaultValue)
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Config file not found, using defaults and environment variables: %v", err)
	}
	viper.AutomaticEnv()
}

func main() {
	initConfig()

	client := evm.NewClient("https://ethereum-rpc.publicnode.com", "eth")

	// Get Moralis API key from environment variable
	moralisAPIKey := viper.GetString(config.MoralisAPIKey)
	if moralisAPIKey == "" {
		log.Fatal("MORALIS_API_KEY environment variable is required")
	}

	moralisAPIURL := viper.GetString(config.MoralisAPIURL)
	moralisClient := moralis.NewMoralisClient(moralisAPIKey, moralisAPIURL)

	transactions, err := moralisClient.GetTransactionsByAddress("0x1ab4973a48dc892cd9971ece8e01dcc7688f8f23", "eth")
	if err != nil {
		log.Fatalf("failed to get transactions: %v", err)
	}

	if len(transactions) > 0 {
		txHash := transactions[0].Hash
		log.Println("txHash", txHash)
		services := service.NewDiscoveryService(map[string]*evm.Client{"eth": evm.NewClient("https://ethereum-rpc.publicnode.com", "eth")}, nil, nil, nil)
		txData, err := client.GetTransactionByHash(context.Background(), txHash)
		if err != nil {
			log.Fatalf("failed to get transaction: %v", err)
		}
		log.Println("txData", string(txData))

		recoveredKey, _, err := services.RecoverPublicKeyFromTransactionData(context.Background(), client, txData, txHash)
		if err != nil {
			log.Fatalf("failed to recover public key: %v", err)
		}
		log.Println("recoveredKey", recoveredKey)
	}

}
