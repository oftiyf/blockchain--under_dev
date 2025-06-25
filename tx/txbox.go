package tx

import (
	"blockchain/common"
	"math/big"
)

// 这里采用队列的逻辑，先进先出
type TxBox struct {
	address    common.Address
	txs        []*Transaction
	firstnonce uint64   //为了方便queue=>pending
	lastnonce  uint64   //为了盒子中连接等操作
	gasprice   *big.Int //默认是第一个交易的gasprice
}

// NewTxBox creates a new transaction queue
func NewTxBox(address common.Address, txs []*Transaction) *TxBox {
	return &TxBox{
		address:    address,
		txs:        txs,
		firstnonce: txs[0].GetNonce(),
		lastnonce:  txs[len(txs)-1].GetNonce(),
		gasprice:   txs[0].GetGasPrice(),
	}
}

// 交易的进入
func (txbox *TxBox) Enqueue(tx *Transaction) error {
	if tx.GetGasPrice().Cmp(txbox.gasprice) < 0 {
		panic("gasprice too low")
	}
	txbox.txs = append(txbox.txs, tx)
	//check nonce same
	if tx.GetNonce() != txbox.lastnonce+1 {
		panic("nonce not in order")
	}
	txbox.lastnonce = txbox.lastnonce + 1
	return nil
}

// 执行交易的时候进行操作
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

func (txbox *TxBox) replace(tx *Transaction) []TxBox {
	//要考虑2中情况，如果tx<txbox.gasprice，则分裂，如果tx>txbox.gasprice，则替换
	return nil
}

// ------------------------------------------------------
//
//	get 方法
//
// --------------------------------------------------
func (txbox *TxBox) Getlength() int {
	return len(txbox.txs)
}

// GetGasPrice returns the gas price of the transaction box
func (txbox *TxBox) GetGasPrice() *big.Int {
	return txbox.gasprice
}

func (txbox *TxBox) GetAddress() common.Address {
	return txbox.address
}

type boxes []TxBox

func (b boxes) Len() int {
	return len(b)
}

func (b boxes) Less(i, j int) bool {
	return b[i].GetGasPrice().Cmp(b[j].GetGasPrice()) < 0
}

func (b boxes) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
