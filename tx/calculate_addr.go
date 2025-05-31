package tx

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// RecoverAddressFromSignature recovers the sender's address from r, s, v values
// r, s: signature components
// v: recovery id (27 or 28)
// hash: the hash of the message that was signed
func RecoverAddressFromSignature(r, s string, v uint8, hash []byte) (common.Address, error) {
	// Convert r and s from hex strings to big integers
	rBytes, err := hex.DecodeString(r)
	if err != nil {
		return common.Address{}, err
	}
	sBytes, err := hex.DecodeString(s)
	if err != nil {
		return common.Address{}, err
	}

	// Create the signature
	sig := make([]byte, 65)
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	sig[64] = v

	// Recover the public key
	pubKey, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return common.Address{}, err
	}

	// Get the address from the public key
	address := crypto.PubkeyToAddress(*pubKey)
	return address, nil
}
