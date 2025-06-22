package common

import (
	"encoding/json"
)

type Account struct {
	Nonce    uint64	`json:"nonce"`
	Balance  uint64	`json:"balance"`
	CodeHash []byte	`json:"codeHash"`
	Code     []byte	`json:"code"`
	Storage  map[string]string	`json:"storage"`
	IsEoa    bool	`json:"isEoa"`
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
func GetAccount(Nonce uint64, Balance uint64, CodeHash []byte, Code []byte, Storage map[string]string, IsEoa bool) *Account {
	return &Account{
		Nonce:    Nonce,
		Balance:  Balance,
		CodeHash: CodeHash,
		Code:     Code,
		Storage:  Storage,
		IsEoa:    IsEoa,
	}
}

func (a *Account) Serialize() []byte {
	data, _ := json.Marshal(a)
	return data
}

func Reserialize(data []byte) *Account {
	var account Account
	json.Unmarshal(data, &account)
	return &account
}