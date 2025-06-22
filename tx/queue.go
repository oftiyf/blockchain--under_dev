package tx

import (
	"blockchain/common"
	"sort"
)

type TxQueue struct {
	
	//[address]=>txs
	txs map[common.Address][]*Transaction
}

func (q *TxQueue) Enqueue(tx *Transaction,nownonce uint64) ([]*Transaction) {
	addr,err:=tx.GetSender()
	if err!=nil{
		return nil//tx is not valid
	}
	
	txs:=q.txs[addr]
	q.txs[addr]=append(txs,tx)
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].GetNonce() < txs[j].GetNonce()
	})
	ret_txs:=q.checkpendingpool(addr,nownonce)
	
	return ret_txs
	
}

func (q *TxQueue) checkpendingpool(addr common.Address,nownonce uint64) ([]*Transaction) {
	//在这里检查是否连续的nonce，是否在pendingpool中
	txs:=q.txs[addr]
	for i:=0;i<len(txs)-1;i++{
		if txs[i].GetNonce()+1!=txs[i+1].GetNonce(){
			//如果nonce不连续，则返回前面连续的txs
			return txs[:i+1]
		}
	}
	return txs
}