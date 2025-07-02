package tx

import (
	"blockchain/common"
	"blockchain/mpt"
	"errors"
	"fmt"
	"sort"
)

type vm_execute interface {
	Execute(tx *Transaction) error
}

type TxPool struct {
	StatDB      *mpt.MPT
	pending     map[common.Address][]*TxBox
	queue       map[common.Address]map[uint64]*Transaction
	Sortedboxes boxes
}

// -------------------------------------------------------------------------------------
// ---------------------------------对外主要方法------------------------------------------
// -------------------------------------------------------------------------------------
func (pool *TxPool) NewTX(tx *Transaction) error {
	//如果pool里的切片是空的，则make出来
	if pool.Sortedboxes == nil {
		pool.Sortedboxes = make(boxes, 0)
	}
	if pool.pending == nil {
		pool.pending = make(map[common.Address][]*TxBox)
	}
	if pool.queue == nil {
		pool.queue = make(map[common.Address]map[uint64]*Transaction)
	}
	address, err := tx.GetSender()
	if err != nil {
		return err
	}
	account_nonce, err := pool.getaccountnonce(address)
	if err != nil {
		return err
	}
	//第一个情况，如果当前的nonce比要添加的大，自动返回错误
	if account_nonce >= tx.Nonce {
		return errors.New("nonce error")
	}

	//下面要处理，tx.nonce>=account.nonce+1
	nonce := account_nonce
	boxes := pool.pending[address]
	if len(boxes) > 0 {
		last := boxes[len(boxes)-1]
		nonce = last.lastnonce
	}
	// tx.nonce>account.nonce+1, 说明要添加到queue中
	if tx.Nonce > nonce+1 {
		pool.addQueueTx(tx)
		return nil
	} else if tx.Nonce == nonce+1 { // 说明要添加到pending中
		// push
		pool.addPendingTx(tx)
		return nil
	} else { // tx.nonce<=account.nonce说明要替换pending中的交易
		// 替换
		pool.replacePendingTx(tx)
		return nil
	}
}

func (pool *TxPool) Execute(vm vm_execute, num int) error {
	for i := 0; i < num; i++ {
		err := pool.execute_one(vm)
		if err != nil {
			return err
		}
	}
	return nil
}

//-------------------------------------------------------------------------------------
//---------------------------------一次封装---------------------------------------------
//-------------------------------------------------------------------------------------

func (pool *TxPool) execute_one(vm vm_execute) error {
	tx := pool.Pop()
	if tx == nil {
		return nil
	}
	err := vm.Execute(tx)
	if err != nil {
		return err
	}
	return nil
}
func (pool *TxPool) Pop() *Transaction {
	boxes := pool.pending[pool.Sortedboxes[0].GetAddress()]
	if len(boxes) == 0 {
		return nil
	}

	tx := boxes[0].Dequeue()
	if len(boxes[0].txs) == 0 {
		pool.pending[pool.Sortedboxes[0].GetAddress()] = boxes[1:]
	}
	return tx
}

//-------------------------------------------------------------------------------------
//---------------------------------具体实现---------------------------------------------
//-------------------------------------------------------------------------------------

func (pool *TxPool) addQueueTx(tx *Transaction) error {
	address, err := tx.GetSender()
	if err != nil {
		return err
	}
	list := pool.queue[address]
	if list == nil {
		list = make(map[uint64]*Transaction)
	}
	list[tx.Nonce] = tx
	pool.queue[address] = list
	return nil
}

func (pool *TxPool) addPendingTx(tx *Transaction) error {
	address, err := tx.GetSender()
	if err != nil {
		return err
	}
	boxes := pool.pending[address]
	if len(boxes) == 0 {
		//加到pending中
		box := NewTxBox(address, []*Transaction{tx})
		pool.pending[address] = append(pool.pending[address], box)
		pool.Sortedboxes = append(pool.Sortedboxes, *box)
		sort.Sort(pool.Sortedboxes)
	} else {
		last := boxes[len(boxes)-1]
		if tx.GasPrice.Cmp(last.GetGasPrice()) >= 0 { //这里是，如果tx.gas>=last.gas，说明要加到最后一个box里面
			//加到pending中
			last.Enqueue(tx)
		} else {
			//反之则说明，要新建一个txbox在pending中
			box := NewTxBox(address, []*Transaction{tx})
			pool.pending[address] = append(pool.pending[address], box)
			pool.Sortedboxes = append(pool.Sortedboxes, *box)
			sort.Sort(pool.Sortedboxes)
		}
	}

	//看Queue(map类型)中是否要更新出来交易到pending中，其中Nonce值是tx.Nonce+1，并且有个循环检查是否把符合的输出完全
	list := pool.queue[address]
	if list != nil {
		nonce := tx.Nonce + 1 //这里就是理论上现在的pending里面的最后一个nonce了
		for {
			if tx, ok := list[nonce]; ok {
				delete(list, nonce)
				pool.queue[address] = list
				pool.addPendingTx(tx)
				nonce++
			} else {
				break
			}
		}
	}
	return nil
}

func (pool *TxPool) replacePendingTx(tx *Transaction) error {
	address, err := tx.GetSender()
	if err != nil {
		return err
	}
	oldtxboxes := pool.pending[address]
	var flag int
	newboxes := []*TxBox{}
	for i, txbox := range oldtxboxes {
		if txbox.firstnonce == tx.Nonce { //找到要替换的txbox
			fmt.Println("txbox[0]", txbox.txs[0])
			fmt.Println("tx", tx)
			lastbox := oldtxboxes[:i] //这里模拟的就是一开始lastbox
			txbox.txs[0] = tx
			fmt.Println("lastboxdwasdaw", lastbox)
			for _, tx := range txbox.txs {
				//这里模拟一下pending=>queue的逻辑，假设tx.box为queue中的，而lastbox一开始为pending中的
				lastbox = pool.cutPending(lastbox, tx)
				fmt.Println("lastbox[0]123", lastbox[0].txs[0])
			}
			lastbox = append(lastbox, oldtxboxes[i:]...)
			newboxes = append(newboxes, lastbox...)
			fmt.Println("newboxes")
			
			flag = i
			break
		} 
	}
	newboxes = append(newboxes, oldtxboxes[flag:]...)
	pool.pending[address] = newboxes
	//删除一下Sortedboxes里面地址等于address的txbox
	for i, txbox := range pool.Sortedboxes {
		if txbox.GetAddress() == address {
			// 删除第i个元素
			pool.Sortedboxes = append(pool.Sortedboxes[:i], pool.Sortedboxes[i+1:]...)
			
		}
	}
	//添加一下newboxes到Sortedboxes中
	for _, box := range newboxes {
		pool.Sortedboxes = append(pool.Sortedboxes, *box)
	}
	sort.Sort(pool.Sortedboxes)
	return nil
}

func (pool *TxPool) cutPending(boxes []*TxBox, tx *Transaction) []*TxBox {
	//这里是模仿的queue=>pending的逻辑，但是这里不进行递归调用，外层for循环来便利
	address, err := tx.GetSender()
	if err != nil {
		return nil
	}
	if len(boxes) == 0 {
		//加到pending中
		box := NewTxBox(address, []*Transaction{tx})
		fmt.Println("txincutpending", tx)
		boxes = append(boxes, box)
		fmt.Println("boxes123123123asdawsda123", boxes)
		return boxes
	} else {
		last := boxes[len(boxes)-1]
		if tx.GasPrice.Cmp(last.GetGasPrice()) >= 0 { //这里是，如果tx.gas>=last.gas，说明要加到最后一个box里面
			//加到pending中
			last.Enqueue(tx)
		} else {
			//反之则说明，要新建一个txbox在pending中
			box := NewTxBox(address, []*Transaction{tx})
			boxes = append(boxes, box)
		}
	}
	return boxes
}

//--------------------------------------------------------------------------------------
//----------------------------------基础方法---------------------------------------------
//--------------------------------------------------------------------------------------

func (txPool *TxPool) getaccountnonce(addr common.Address) (uint64, error) {
	addrBytes := addr.Bytes()
	accountBytes, err := txPool.StatDB.Get(addrBytes)
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
