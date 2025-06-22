package tx

import (
	
	"math/big"
)

// 这里采用队列的逻辑，先进先出
type TxBox struct {
	txs        []*Transaction
	firstnonce uint64
	lastnonce  uint64
	gasprice   *big.Int
}

// NewTxBox creates a new transaction queue
func NewTxBox(txs []*Transaction) *TxBox {
	return &TxBox{
		txs:        txs,
		firstnonce: txs[0].GetNonce(),
		lastnonce:  txs[len(txs)-1].GetNonce(),
		gasprice:   txs[0].GetGasPrice(),
	}
}

// Enqueue adds a transaction to the end of the queue
func (txbox *TxBox) Enqueue(tx *Transaction) error {
	txbox.txs = append(txbox.txs, tx)
	//check gasprice same
	// 检查新交易的gas价格是否与txbox中已有交易的gas价格相同
	// 如果不同则panic,因为同一个txbox中的所有交易gas价格必须相同
	if tx.GetGasPrice().Cmp(txbox.gasprice) != 0 {
		panic("gasprice not same")
	}
	//check nonce same
	if tx.GetNonce() != txbox.lastnonce+1 {
		panic("nonce not same")
	}
	txbox.lastnonce = tx.GetNonce()
	return nil
}

// Dequeue removes and returns the first transaction in the queue
func (txbox *TxBox) Dequeue() *Transaction {
	if len(txbox.txs) == 0 {
		return nil
	}
	tx := txbox.txs[0]
	txbox.txs = txbox.txs[1:]

	// 只有当队列中还有交易时才更新firstnonce
	if len(txbox.txs) > 0 {
		txbox.firstnonce = txbox.txs[0].GetNonce()
	}

	return tx
}

func (txbox *TxBox) Getlength() int {
	return len(txbox.txs)
}

// GetGasPrice returns the gas price of the transaction box
func (txbox *TxBox) GetGasPrice() *big.Int {
	return txbox.gasprice
}
