package common

import (
	"encoding/json"
)

type Account struct {
	Nonce    uint64
	Balance  uint64
	CodeHash []byte
	Code     []byte
	Storage  map[string]string
	IsEoa    bool
}

// NewEOA creates a new externally owned account
func NewEOA(pubKey []byte) *Account {
	return &Account{
		Nonce:    0,
		Balance:  0,
		CodeHash: nil,
		Code:     nil,
		Storage:  nil,
		IsEoa:    true,
	}
}

// NewContract creates a new contract account
func NewContract(codeHash []byte, code []byte) *Account {
	return &Account{
		Nonce:    0,
		Balance:  0,
		CodeHash: codeHash,
		Code:     code,
		Storage:  make(map[string]string),
		IsEoa:    false,
	}
}

func NewAccount(codeHash, rootHash []byte) *Account {
	return &Account{
		Nonce:    0,
		Balance:  0,
		CodeHash: codeHash,
		Code:     nil,
		Storage:  make(map[string]string),
	}
}

func (a *Account) Serialize() []byte {
	data, _ := json.Marshal(a)
	return data
}
