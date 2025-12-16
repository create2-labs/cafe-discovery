package main

import (
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"
	"context"
	"log"
)

func main() {
	client := evm.NewClient("https://ethereum-rpc.publicnode.com", "eth")
	moralisClient := moralis.NewMoralisClient("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJub25jZSI6ImFlY2RmMTcxLTc1MDgtNDhhNi04ZmViLTQ1YTVhMGEzYWNjMSIsIm9yZ0lkIjoiNDgxMjkwIiwidXNlcklkIjoiNDk1MTUzIiwidHlwZUlkIjoiMTljZDQwNWUtMTA5Yi00MzY1LWI2YzktYzY0ODViMWZiN2VkIiwidHlwZSI6IlBST0pFQ1QiLCJpYXQiOjE3NjMxMDU2NjUsImV4cCI6NDkxODg2NTY2NX0.EvyUJ0IwiO7lHTQaWeGnJNWx8Mti8j1-NUO3tKSqQi4", "https://deep-index.moralis.io")

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
