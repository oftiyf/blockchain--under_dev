package stateDB

import (
	"blockchain/common"
)

type StateDB interface {
	GetAccount(common.Address) (*common.Account, error)
	
	SetAccount(common.Address, *common.Account) error

	Root() ([]byte, error)	

	
}

