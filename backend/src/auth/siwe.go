package auth

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// RecoverAddress recovers the signer address from an EIP-191 personal_sign message + signature.
func RecoverAddress(message, hexSignature string) (common.Address, error) {
	sigBytes, err := hexutil.Decode(hexSignature)
	if err != nil {
		return common.Address{}, fmt.Errorf("decode signature: %w", err)
	}
	if len(sigBytes) != 65 {
		return common.Address{}, fmt.Errorf("invalid signature length: %d", len(sigBytes))
	}

	// EIP-191 prefix
	prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := crypto.Keccak256Hash([]byte(prefixed))

	// Normalise recovery ID (Ethereum uses 27/28, ecrecover needs 0/1)
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	pubKeyBytes, err := crypto.Ecrecover(hash.Bytes(), sigBytes)
	if err != nil {
		return common.Address{}, fmt.Errorf("ecrecover: %w", err)
	}
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		return common.Address{}, fmt.Errorf("unmarshal pubkey: %w", err)
	}
	return crypto.PubkeyToAddress(*pubKey), nil
}

// BuildSIWEMessage constructs the EIP-4361 message that the frontend must sign.
func BuildSIWEMessage(domain, address, nonce, issuedAt string) string {
	return strings.Join([]string{
		domain + " wants you to sign in with your Ethereum account:",
		address,
		"",
		"Sign in to PulsarFi",
		"",
		"URI: https://" + domain,
		"Version: 1",
		"Chain ID: 421614",
		"Nonce: " + nonce,
		"Issued At: " + issuedAt,
	}, "\n")
}
