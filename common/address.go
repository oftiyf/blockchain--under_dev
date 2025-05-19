package common

import (
	"encoding/hex"
)

const (
	AddressLength = 20
)

type Address [AddressLength]byte

func (a Address) NewAddress(data []byte) Address {
	if len(data) != AddressLength {
		panic("data length must be 20")
	}
	var address Address
	copy(address[:], data)
	return address
}

func (a Address) Bytes() []byte {
	return a[:]
}

func (a Address) String() string {
	return hex.EncodeToString(a[:])
}


