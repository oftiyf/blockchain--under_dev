package tx

import (
	"math/big"
)

// txList 用于管理同一账户下的多笔交易（按Nonce排序）
type txList struct {
	txs       []*Transaction // 交易列表，按Nonce升序排列
	isPending bool           // 是否为pending队列
}

// newTxList 创建新的txList
func newTxList(isPending bool) *txList {
	return &txList{
		txs:       []*Transaction{},
		isPending: isPending,
	}
}

// Add 添加交易，若同Nonce已存在则按GasPrice替换
// 返回是否插入、被替换的旧交易
func (l *txList) Add(tx *Transaction, priceBump uint64) (bool, *Transaction) {
	for i, old := range l.txs {
		if old.Nonce == tx.Nonce {
			// GasPrice足够高才替换
			bump := new(big.Int).Mul(old.GasPrice, big.NewInt(int64(100+priceBump)))
			bump.Div(bump, big.NewInt(100))
			if tx.GasPrice.Cmp(bump) > 0 {
				l.txs[i] = tx
				return true, old
			}
			return false, nil
		}
	}
	// 插入新交易
	l.txs = append(l.txs, tx)
	l.sort()
	return true, nil
}

// Remove 移除指定交易，返回是否移除、被影响的后续交易
func (l *txList) Remove(tx *Transaction) (bool, []*Transaction) {
	for i, t := range l.txs {
		if t == tx {
			invalids := l.txs[i+1:]
			l.txs = l.txs[:i]
			return true, invalids
		}
	}
	return false, nil
}

// Forward 移除Nonce小于指定值的交易，返回被移除的交易
func (l *txList) Forward(nonce uint64) []*Transaction {
	var removed []*Transaction
	var i int
	for i = 0; i < len(l.txs); i++ {
		if l.txs[i].Nonce >= nonce {
			break
		}
		removed = append(removed, l.txs[i])
	}
	l.txs = l.txs[i:]
	return removed
}

// Filter 过滤余额不足或Gas超限的交易，返回被移除和无效的交易
func (l *txList) Filter(balance *big.Int, maxGas *big.Int) (drops, invalids []*Transaction) {
	var keep []*Transaction
	for _, tx := range l.txs {
		if tx.GasPrice.Cmp(maxGas) > 0 || tx.Value.Cmp(balance) > 0 {
			drops = append(drops, tx)
		} else {
			keep = append(keep, tx)
		}
	}
	l.txs = keep
	return
}

// Ready 返回Nonce连续、余额足够的可执行交易
func (l *txList) Ready(nonce uint64) []*Transaction {
	var ready []*Transaction
	for _, tx := range l.txs {
		if tx.Nonce == nonce {
			ready = append(ready, tx)
			nonce++
		} else {
			break
		}
	}
	return ready
}

// Cap 限制队列长度，超出部分返回并移除
func (l *txList) Cap(limit int) []*Transaction {
	if len(l.txs) <= limit {
		return nil
	}
	exceed := l.txs[limit:]
	l.txs = l.txs[:limit]
	return exceed
}

// Flatten 返回所有交易
func (l *txList) Flatten() []*Transaction {
	return l.txs
}

// Len 返回交易数量
func (l *txList) Len() int {
	return len(l.txs)
}

// Empty 判断队列是否为空
func (l *txList) Empty() bool {
	return len(l.txs) == 0
}

// Overlaps 判断是否有同Nonce交易
func (l *txList) Overlaps(tx *Transaction) bool {
	for _, t := range l.txs {
		if t.Nonce == tx.Nonce {
			return true
		}
	}
	return false
}

// sort 按Nonce升序排序
func (l *txList) sort() {
	// 简单插入排序，因交易量不大
	for i := 1; i < len(l.txs); i++ {
		j := i
		for j > 0 && l.txs[j-1].Nonce > l.txs[j].Nonce {
			l.txs[j-1], l.txs[j] = l.txs[j], l.txs[j-1]
			j--
		}
	}
}
