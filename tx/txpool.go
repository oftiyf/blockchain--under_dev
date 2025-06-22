package tx

import (
	"blockchain/common"
	"blockchain/mpt"
)

type TxPool struct {
	pending map[common.Address][]*Transaction
	queued  map[common.Address][]*Transaction
	mpt     *mpt.MPT
}

func NewTxPool(mpt *mpt.MPT) *TxPool {
	return &TxPool{
		pending: make(map[common.Address][]*Transaction),
		queued:  make(map[common.Address][]*Transaction),
		mpt:     mpt,
	}
}

func (txPool *TxPool) getaccountnonce(addr common.Address) (uint64, error) {
	addrBytes := addr.Bytes()
	accountBytes, err := txPool.mpt.Get(addrBytes)
	if err != nil {
		return 0, err
	}
	if accountBytes == nil {
		return 0, nil // New account starts with nonce 0
	}
	accountvalue:=common.Reserialize(accountBytes)
	nonce:=accountvalue.Nonce
	return nonce, nil
}
