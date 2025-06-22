package tx

import (
	"sort"
)

type TxQueue struct {
	txs []*Transaction
}

func (q *TxQueue) Enqueue(tx *Transaction) {