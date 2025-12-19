package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// recIDFromV calcule le "recovery id" (0 ou 1) à partir de v.
// Gère les cas legacy (27/28) et EIP-155 (chainID*2+35/36) et EIP-1559 (0/1).
func recIDFromV(v *big.Int, chainID *big.Int) byte {
	vUint := v.Uint64()

	// EIP-1559 / EIP-2930 : v est déjà 0 ou 1
	if vUint == 0 || vUint == 1 {
		return byte(vUint)
	}

	// Legacy : 27 / 28
	if vUint == 27 || vUint == 28 {
		return byte(vUint - 27)
	}

	// EIP-155 (chainID*2 + 35/36)
	if chainID != nil {
		// recId = (v - (chainID*2 + 35)) % 2
		base := new(big.Int).Mul(chainID, big.NewInt(2))
		base.Add(base, big.NewInt(35))
		tmp := new(big.Int).Sub(v, base)
		return byte(new(big.Int).Mod(tmp, big.NewInt(2)).Uint64())
	}

	// fallback bourrin
	return byte((vUint - 35) % 2)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <rpc-url> <tx-hash>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Exemple:\n  %s https://mainnet.infura.io/v3/YOUR_KEY 0x6c8388...\n", os.Args[0])
		os.Exit(1)
	}

	rpcURL := os.Args[1]
	txHashHex := os.Args[2]

	txHash := common.HexToHash(txHashHex)

	ctx := context.Background()

	// 1) Connexion au node Ethereum
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("Erreur connexion RPC: %v", err)
	}
	defer client.Close()

	// 2) Récupération de la transaction
	tx, isPending, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		log.Fatalf("Erreur TransactionByHash: %v", err)
	}
	if isPending {
		log.Fatalf("La transaction est encore en pending, impossible de récupérer la signature finale.")
	}

	// 3) Récupération du chainID
	chainID, err := client.NetworkID(ctx)
	if err != nil {
		log.Fatalf("Erreur NetworkID: %v", err)
	}

	// 4) Récupérer les valeurs de signature brutes (v, r, s)
	v, r, s := tx.RawSignatureValues()

	// 5) Construire le "hash à signer" (signing hash) avec le bon Signer
	signer := types.LatestSignerForChainID(chainID)
	signHash := signer.Hash(tx) // c'est sur ce hash que l'ECDSA a été fait

	// 6) Reconstituer la signature [R || S || V] au format attendu par crypto.SigToPub
	sig := make([]byte, 65)

	rBytes := r.FillBytes(make([]byte, 32))
	sBytes := s.FillBytes(make([]byte, 32))
	copy(sig[0:32], rBytes)
	copy(sig[32:64], sBytes)

	recID := recIDFromV(v, chainID)
	sig[64] = recID

	// 7) ECDSA public key recovery
	pubKey, err := crypto.SigToPub(signHash.Bytes(), sig)
	if err != nil {
		log.Fatalf("Erreur SigToPub: %v", err)
	}

	pubKeyBytes := crypto.FromECDSAPub(pubKey) // 65 bytes (0x04 + X(32) + Y(32))

	// 8) Vérifier que l'adresse dérivée de la clé publique = sender
	derivedAddr := crypto.PubkeyToAddress(*pubKey)

	// 9) Obtenir le "from" via go-ethereum (pour comparaison)
	fromAddr, err := types.Sender(signer, tx)
	if err != nil {
		log.Fatalf("Erreur types.Sender: %v", err)
	}

	// 10) Affichage
	fmt.Printf("RPC         : %s\n", rpcURL)
	fmt.Printf("Tx hash     : %s\n\n", txHash.Hex())

	fmt.Printf("Chaîne      : %s\n", chainID.String())
	fmt.Printf("v           : 0x%s\n", v.Text(16))
	fmt.Printf("r           : 0x%s\n", r.Text(16))
	fmt.Printf("s           : 0x%s\n\n", s.Text(16))

	fmt.Printf("signHash    : 0x%s\n\n", hex.EncodeToString(signHash.Bytes()))

	fmt.Printf("Clé publique (uncompressed, 65 bytes) :\n0x%s\n\n", hex.EncodeToString(pubKeyBytes))

	fmt.Printf("Adresse dérivée de la clé publique : %s\n", derivedAddr.Hex())
	fmt.Printf("Adresse 'from' retrouvée par go-ethereum : %s\n", fromAddr.Hex())
	if derivedAddr == fromAddr {
		fmt.Println("✅ Vérification OK : la clé publique correspond bien au sender.")
	} else {
		fmt.Println("❌ Attention : l'adresse dérivée ne correspond pas au sender (quelque chose cloche).")
	}
}
