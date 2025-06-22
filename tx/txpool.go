package tx

import (
	"blockchain/common"
	"blockchain/mpt"
)

// TransactionExecutor defines the interface for executing transactions
type TransactionExecutor interface {
	ExecuteTransaction(tx *Transaction) error
}

type TxPool struct {
	pending PendingPool
	queued  TxQueue
	mpt     *mpt.MPT
}

func NewTxPool(mpt *mpt.MPT) *TxPool {
	return &TxPool{
		pending: PendingPool{},
		queued:  TxQueue{},
		mpt:     mpt,
	}
}

// 执行单个txbox中的交易
func (txPool *TxPool) executeTxBox(txbox *TxBox, executor TransactionExecutor, remaining int) (int, error) {
	executed := 0
	for txbox.Getlength() > 0 && executed < remaining {
		tx := txbox.Dequeue()
		if tx == nil {
			break
		}

		err := executor.ExecuteTransaction(tx)
		if err != nil {
			continue
		}
		executed++
	}
	return executed, nil
}

// 执行指定数量的交易
func (txPool *TxPool) execute(executor TransactionExecutor, num int) error {
	// 先按gas price排序
	txPool.pending.sortByGasPrice()

	executedCount := 0
	remainingTxBoxes := []*TxBox{}

	// 遍历执行交易
	for _, txbox := range txPool.pending.GetTxBoxes() {
		if executedCount >= num {
			remainingTxBoxes = append(remainingTxBoxes, txbox)
			continue
		}

		executed, _ := txPool.executeTxBox(txbox, executor, num-executedCount)
		executedCount += executed

		if txbox.Getlength() > 0 {
			remainingTxBoxes = append(remainingTxBoxes, txbox)
		}
	}

	txPool.pending.SetTxBoxes(remainingTxBoxes)
	return nil
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
	accountvalue := common.Reserialize(accountBytes)
	nonce := accountvalue.Nonce
	return nonce, nil
}

func (txPool *TxPool) addtx(tx *Transaction) error {
	addr, err := tx.GetSender()
	if err != nil {
		return err
	}
	nonce, err := txPool.getaccountnonce(addr)
	if err != nil {
		return err
	}
	if tx.GetNonce() != nonce {
		return err
	}
	txs := txPool.queued.Enqueue(tx, nonce)
	if txs[0].GetNonce() == nonce+1 {
		txPool.pending.addtxs(txs)
	}
	return nil
}
