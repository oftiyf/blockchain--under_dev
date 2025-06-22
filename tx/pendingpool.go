package tx

import (
	"fmt"
	"sort"
)

// 一次执行之后，剩余的tx，是放在内存里还是放在数据库里？放在内存里，占用资源特别大
//nonce约束应该放在vm层还是哪？

// 是否要实现，同一个nonce，出现多次不同的情况？比如box的分裂等
type PendingPool struct {
	txboxes []*TxBox
}

// 按gas price对txboxes进行排序
func (pendingpool *PendingPool) sortByGasPrice() {
	sort.Slice(pendingpool.txboxes, func(i, j int) bool {
		return pendingpool.txboxes[i].GetGasPrice().Cmp(pendingpool.txboxes[j].GetGasPrice()) > 0
	})
}

// GetTxBoxes 获取txboxes
func (pendingpool *PendingPool) GetTxBoxes() []*TxBox {
	return pendingpool.txboxes
}

// SetTxBoxes 设置txboxes
func (pendingpool *PendingPool) SetTxBoxes(txboxes []*TxBox) {
	pendingpool.txboxes = txboxes
}

// AddTxBoxes 向PendingPool添加盒子
func (pendingpool *PendingPool) AddTxBoxes(txboxes []*TxBox) error {
	// 直接添加所有盒子到pendingpool
	for _, txbox := range txboxes {
		if txbox == nil || txbox.Getlength() == 0 {
			continue
		}
		pendingpool.txboxes = append(pendingpool.txboxes, txbox)
	}

	// 使用MergeBoxesByGasPrice来合并相同gas price的盒子并排序
	pendingpool.MergeBoxesByGasPrice()
	return nil
}

// MergeBoxesByGasPrice 按gas price合并盒子
func (pendingpool *PendingPool) MergeBoxesByGasPrice() {
	if len(pendingpool.txboxes) <= 1 {
		return
	}

	// 按gas price分组
	gasPriceGroups := make(map[string][]*TxBox)

	for _, txbox := range pendingpool.txboxes {
		gasPriceStr := txbox.GetGasPrice().String()
		gasPriceGroups[gasPriceStr] = append(gasPriceGroups[gasPriceStr], txbox)
	}

	// 合并相同gas price的盒子
	var mergedBoxes []*TxBox
	for _, groupBoxes := range gasPriceGroups {
		if len(groupBoxes) == 0 {
			continue
		}

		if len(groupBoxes) == 1 {
			mergedBoxes = append(mergedBoxes, groupBoxes[0])
			continue
		}

		// 合并多个盒子
		mergedBox := groupBoxes[0]
		for i := 1; i < len(groupBoxes); i++ {
			// 将其他盒子的交易合并到第一个盒子
			for groupBoxes[i].Getlength() > 0 {
				tx := groupBoxes[i].Dequeue()
				if err := mergedBox.Enqueue(tx); err != nil {
					// 如果合并失败，跳过这个交易
					continue
				}
			}
		}
		mergedBoxes = append(mergedBoxes, mergedBox)
	}

	pendingpool.txboxes = mergedBoxes
	// 重新排序
	pendingpool.sortByGasPrice()
}

// AddTransactions 向PendingPool添加交易（简化版本）
func (pendingpool *PendingPool) AddTransactions(txs []*Transaction) error {
	// 为每个交易创建一个独立的盒子
	for _, tx := range txs {
		if tx == nil {
			continue
		}

		// 获取发送者地址
		sender, err := tx.GetSender()
		if err != nil {
			return fmt.Errorf("failed to get sender address: %v", err)
		}

		// 为单个交易创建盒子
		txbox := NewTxBox(sender, []*Transaction{tx})
		pendingpool.txboxes = append(pendingpool.txboxes, txbox)
	}

	// 统一合并相同gas price的盒子
	pendingpool.MergeBoxesByGasPrice()
	return nil
}

func (pendingpool *PendingPool) addtxs(txs []*Transaction) error {
	return pendingpool.AddTransactions(txs)
}
