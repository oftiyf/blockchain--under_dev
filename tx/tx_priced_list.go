package tx

import (
	"blockchain/common"
	"container/heap"
)

// txPricedList 按GasPrice排序的交易列表
// 使用最小堆实现，便于快速获取最低价格的交易
type txPricedList struct {
	all    *map[common.Hash]*Transaction // 指向所有交易的映射
	items  *priceHeap                    // 价格堆
	stales int                           // 过期计数器
}

// newTxPricedList 创建新的txPricedList
func newTxPricedList(all *map[common.Hash]*Transaction) *txPricedList {
	return &txPricedList{
		all:   all,
		items: &priceHeap{},
	}
}

// Put 添加交易到价格列表
func (l *txPricedList) Put(tx *Transaction) {
	heap.Push(l.items, tx)
}

// Removed 标记交易被移除，增加过期计数
func (l *txPricedList) Removed() {
	l.stales++
}

// Underpriced 检查交易是否价格过低
func (l *txPricedList) Underpriced(tx *Transaction, locals *accountSet) bool {
	// 本地交易不受价格限制
	from, err := tx.GetSender()
	if err == nil && locals.contains(from) {
		return false
	}

	// 如果列表为空，认为价格合理
	if l.items.Len() == 0 {
		return false
	}

	// 获取最低价格的交易进行比较
	cheapest := (*l.items)[0]
	return tx.GasPrice.Cmp(cheapest.GasPrice) < 0
}

// Discard 丢弃指定数量的最低价格交易
// 返回被丢弃的交易列表
func (l *txPricedList) Discard(count int, locals *accountSet) []*Transaction {
	var drop []*Transaction

	for count > 0 && l.items.Len() > 0 {
		tx := heap.Pop(l.items).(*Transaction)

		// 跳过本地交易
		from, err := tx.GetSender()
		if err == nil && locals.contains(from) {
			continue
		}

		drop = append(drop, tx)
		count--
	}

	return drop
}

// priceHeap 实现heap.Interface的最小堆
type priceHeap []*Transaction

func (h priceHeap) Len() int { return len(h) }

func (h priceHeap) Less(i, j int) bool {
	return h[i].GasPrice.Cmp(h[j].GasPrice) < 0
}

func (h priceHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *priceHeap) Push(x interface{}) {
	*h = append(*h, x.(*Transaction))
}

func (h *priceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
