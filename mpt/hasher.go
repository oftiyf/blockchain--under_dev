package mpt

import (
	"blockchain/common"
	"crypto/sha256"
)

func sha3_256(data []byte) common.Hash {
	hasher := sha256.New()
	hasher.Write(data)
	var hash common.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}
