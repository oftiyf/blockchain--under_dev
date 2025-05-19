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
