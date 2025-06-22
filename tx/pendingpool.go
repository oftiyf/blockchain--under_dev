package tx

import (
	"sort"
)

// TransactionExecutor defines the interface for executing transactions
type TransactionExecutor interface {
	ExecuteTransaction(tx *Transaction) error
}

// 一次执行之后，剩余的tx，是放在内存里还是放在数据库里？放在内存里，占用资源特别大
//nonce约束应该放在vm层还是哪？
type PendingPool struct {
	txboxes []*TxBox
}

// 按gas price对txboxes进行排序
func (pendingpool *PendingPool) sortByGasPrice() {
	sort.Slice(pendingpool.txboxes, func(i, j int) bool {
		return pendingpool.txboxes[i].GetGasPrice().Cmp(pendingpool.txboxes[j].GetGasPrice()) > 0
	})
}

// 执行单个txbox中的交易
func (pendingpool *PendingPool) executeTxBox(txbox *TxBox, executor TransactionExecutor, remaining int) (int, error) {
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
func (pendingpool *PendingPool) execute(executor TransactionExecutor, num int) error {
	// 先按gas price排序
	pendingpool.sortByGasPrice()

	executedCount := 0
	remainingTxBoxes := []*TxBox{}

	// 遍历执行交易
	for _, txbox := range pendingpool.txboxes {
		if executedCount >= num {
			remainingTxBoxes = append(remainingTxBoxes, txbox)
			continue
		}

		executed, _ := pendingpool.executeTxBox(txbox, executor, num-executedCount)
		executedCount += executed

		if txbox.Getlength() > 0 {
			remainingTxBoxes = append(remainingTxBoxes, txbox)
		}
	}

	pendingpool.txboxes = remainingTxBoxes
	return nil
}
