package common

import (
	"encoding/hex"
)

const (
	HashLength = 32
)

type Hash [HashLength]byte

func (h Hash) NewHash(data []byte) Hash {
	if len(data) != HashLength {
		panic("data length must be 32")
	}
	var hash Hash
	copy(hash[:], data)
	return hash
}

func (h Hash) Bytes() []byte {
	return h[:]
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}
