package evm

import (
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// recoverPubKeyFromTx: returns uncompressed pubkey hex (0x04...) and tx hash
// yParity is optional and used for EIP-1559 transactions
func RecoverPubKeyFromTx(tx *types.Transaction, signer types.Signer, chainID *big.Int, yParity *big.Int) (string, string, error) {
	r, s, v := tx.RawSignatureValues()
	if r == nil || s == nil || v == nil {
		return "", "", errors.New("no signature values")
	}

	hash := signer.Hash(tx)

	// For EIP-1559, we'll try both recovery IDs (0 and 1)
	// For legacy transactions, calculate the recovery ID from v
	startRecid := byte(0)
	v64 := v.Uint64()
	if v64 == 27 || v64 == 28 {
		// Legacy transaction (pre-EIP-155)
		startRecid = byte(v64 - 27)
	} else if yParity != nil {
		// Use yParity directly for EIP-1559, but still try both
		recidVal := yParity.Uint64()
		if recidVal > 1 {
			return "", "", errors.New("invalid yParity: must be 0 or 1")
		}
		startRecid = byte(recidVal)
	} else {
		// Calculate from v for EIP-155
		if chainID == nil {
			return "", "", errors.New("chainID required")
		}
		// v = chainID * 2 + 35 + yParity
		// recid = yParity = v - (chainID * 2 + 35)
		expected := new(big.Int).Mul(chainID, big.NewInt(2))
		expected.Add(expected, big.NewInt(35))
		recidBig := new(big.Int).Sub(v, expected)
		recidVal := recidBig.Uint64()

		// Recovery ID must be 0 or 1
		if recidVal > 1 {
			return "", "", errors.New("invalid signature recovery id: must be 0 or 1")
		}
		startRecid = byte(recidVal)
	}

	rb := r.FillBytes(make([]byte, 32))
	sb := s.FillBytes(make([]byte, 32))

	// Try both recovery IDs (0 and 1) - one of them should work
	// Start with the calculated one, then try the other
	for offset := byte(0); offset <= 1; offset++ {
		recidTry := (startRecid + offset) % 2
		sig := append(append(rb, sb...), recidTry)

		// Try Ecrecover first
		pubkeyBytes, err := crypto.Ecrecover(hash.Bytes(), sig)
		if err == nil && pubkeyBytes != nil && len(pubkeyBytes) > 0 {
			pubkey, err := crypto.UnmarshalPubkey(pubkeyBytes)
			if err == nil && pubkey != nil {
				uncompressed := crypto.FromECDSAPub(pubkey)
				return "0x" + hex.EncodeToString(uncompressed), tx.Hash().Hex(), nil
			}
		}

		// Try alternative method SigToPub
		pubkey, err2 := crypto.SigToPub(hash.Bytes(), sig)
		if err2 == nil && pubkey != nil {
			uncompressed := crypto.FromECDSAPub(pubkey)
			return "0x" + hex.EncodeToString(uncompressed), tx.Hash().Hex(), nil
		}
	}

	return "", "", errors.New("recovery failed: tried both recovery IDs (0 and 1) but neither worked")
}

// helper to confirm derived address if needed
func PubKeyToAddressHex(uncompressedPubKey []byte) string {
	pub, err := crypto.UnmarshalPubkey(uncompressedPubKey)
	if err != nil {
		return ""
	}
	addr := crypto.PubkeyToAddress(*pub)
	return addr.Hex()
}
