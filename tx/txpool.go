package tx

import (
	"blockchain/common"
	"blockchain/mpt"
	"fmt"
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
		queued: TxQueue{
			txs: make(map[common.Address][]*Transaction),
		},
		mpt: mpt,
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
		fmt.Println("getaccountnonce failed")
		return err
	}

	txs := txPool.queued.Enqueue(tx, nonce)
	if txs[0].GetNonce() == nonce+1 {
		txPool.pending.addtxs(txs)
		fmt.Println("addtxs success")
	}
	return nil
}

// 公共方法，供测试使用

// GetAccountNonce 获取账户nonce
func (txPool *TxPool) GetAccountNonce(addr common.Address) (uint64, error) {
	return txPool.getaccountnonce(addr)
}

// AddTx 添加交易
func (txPool *TxPool) AddTx(tx *Transaction) error {
	txPool.addtx(tx)
	return nil
}

// Execute 执行交易
func (txPool *TxPool) Execute(executor TransactionExecutor, num int) error {
	return txPool.execute(executor, num)
}

// ExecuteTxBox 执行单个盒子
func (txPool *TxPool) ExecuteTxBox(txbox *TxBox, executor TransactionExecutor, remaining int) (int, error) {
	return txPool.executeTxBox(txbox, executor, remaining)
}

// GetPendingTxBoxes 获取pending pool的盒子
func (txPool *TxPool) GetPendingTxBoxes() []*TxBox {
	return txPool.pending.GetTxBoxes()
}

// SortPendingPool 排序pending pool
func (txPool *TxPool) SortPendingPool() {
	txPool.pending.sortByGasPrice()
}

// MergePendingBoxes 合并pending pool的盒子
func (txPool *TxPool) MergePendingBoxes() {
	txPool.pending.MergeBoxesByGasPrice()
}
