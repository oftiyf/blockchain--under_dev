package tx

import (
	"blockchain/common"
	"blockchain/mpt"
	"fmt"
	"sort"
)

// TransactionExecutor defines the interface for executing transactions
type TransactionExecutor interface {
	ExecuteTransaction(tx *Transaction) error
}

type TxPool struct {
	pending map[common.Address][]*TxBox
	queue   map[common.Address][]*TxBox
	mpt     *mpt.MPT
}

func NewTxPool(mpt *mpt.MPT) *TxPool {
	return &TxPool{
		pending: make(map[common.Address][]*TxBox),
		queue:   make(map[common.Address][]*TxBox),
		mpt:     mpt,
	}
}

// 这个是单纯的添加，但是它是不会自动合并盒子
func (txpool *TxPool) AddTx(tx *Transaction) error {
	address, err := tx.GetSender()
	if err != nil {
		return err
	}
	txbox := NewTxBox(address, []*Transaction{tx})
	txpool.queue[address] = append(txpool.queue[address], txbox)
	return nil
}

// --------------------------------------------------------------------------------------
// ----------------------------------以下是执行---------------------------------------------
// --------------------------------------------------------------------------------------
func (txpool *TxPool) execute(vm TransactionExecutor, num int) error {
	for i := 0; i < num; i++ {
		txpool.execute_one_tx(vm)
	}
	return nil
}

func (txpool *TxPool) execute_one_tx(vm TransactionExecutor) error {
	//这里要完成下面很多方法的调用，最后得到一个东西，进行执行
	//
	txpool.merge_qeue()                   //先执行qeue的合并
	txpool.merge_all_address_to_pending() //再执行qeue=>pending的合并
	txpool.merge_pending()                //再执行pending的合并
	//--------------------------------------------------------------------------------------
	//-------------------理论上，上面为了性能应该再继续封装，但是为了可读性保留----------------------
	//--------------------------------------------------------------------------------------
	to_execute_box := txpool.pending_Max_gas_box() //再根据每个地址第一个盒子当中，拿出来gasprice最大的盒子
	to_execute_tx := to_execute_box.Dequeue()      //这里会自动弹出盒子里面的第一笔交易

	if to_execute_tx == nil { //考虑到可能可能盒子内部执行完毕，所以需要删除
		needdlete_addr := to_execute_box.address
		txpool.queue[needdlete_addr] = txpool.queue[needdlete_addr][1:]
		txpool.execute_one_tx(vm)
		return nil
	}
	vm.ExecuteTransaction(to_execute_tx) //执行
	return nil
}

// --------------------------------------------------------------------------------------
// ----------------------------------以下是将box放到pending中------------------------------
// --------------------------------------------------------------------------------------
// 先是封装版本，自动去把所有能到pending版本里的地址的盒子往里面丢
func (txpool *TxPool) merge_all_address_to_pending() error {
	for addr, _ := range txpool.queue {
		txpool.address_queue_to_pending(addr)
	}
	return nil
}

// 完全版本
func (txpool *TxPool) address_queue_to_pending(address common.Address) error {
	//返回的是可能要插入的box
	//采用的是增加删除的方式
	inbox := txpool.queue_check(address)
	firstbox := inbox[0]
	firstnonce := firstbox.firstnonce
	nownonce := uint64(0) //为了下面做检查
	if txpool.pending[address] == nil {
		nownonce, _ = txpool.getaccountnonce(address)

	} else {
		nownonce = txpool.pending[address][len(txpool.pending[address])-1].lastnonce
	}
	if firstnonce == nownonce+1 {
		//实现合并本身
		for _, box := range inbox {
			txpool.pending[address] = append(txpool.pending[address], box)
		}
		//先获取要删除的数量，再进行删除
		todeletenum := len(txpool.queue_check(address))
		fmt.Println("todeletenumawsdwas", todeletenum)
		txpool.queue[address] = txpool.queue[address][todeletenum:]
	} else {
		fmt.Println("now nonce can't be add in pending")
	}
	return nil
}
func (txpool *TxPool) queue_check(address common.Address) []*TxBox {
	txpool.sortqueue(address)
	//检查某一地址上的queue中的box是否连续
	//这里不会放置我对原队列的修改，不检查最初的box的nonce
	//这里要求必须先调用排序，再进行检查
	boxes := txpool.queue[address]
	returnboxes := make([]*TxBox, 0)
	returnboxes = append(returnboxes, boxes[0])
	for i, box := range boxes {
		if i == 0 {
			fmt.Println("box.firstnoncawsdwae11", box.firstnonce,boxes[i].lastnonce+1)
			continue
		}
		if box.firstnonce != boxes[i-1].lastnonce+1 {
			fmt.Println("box.firstnoncawsdwae", box.firstnonce,boxes[i-1].lastnonce+1)
			break
		}
		returnboxes = append(returnboxes, box)
		fmt.Println("returnboxesawsdwasawdslhawiusd")
		
	}
	fmt.Println("returnboxesawdwasdwasdad", len(returnboxes))
	return returnboxes
}

func (txpool *TxPool) sortqueue(address common.Address) error {
	// 排序queue中的box，按照firstnonce排序
	boxes := txpool.queue[address]
	sort.Slice(boxes, func(i, j int) bool {
		return boxes[i].firstnonce < boxes[j].firstnonce
	})
	txpool.queue[address] = boxes
	return nil
}

//--------------------------------------------------------------------------------------
//----------------------------------合并box----------------------------------------------
//--------------------------------------------------------------------------------------
//-------------------这里还要再封装一下，就是合并某个池子里面所有的box--------------------------

// 封装下面所有池子的合并
func (txpool *TxPool) merge_all() error {
	txpool.merge_qeue()
	txpool.merge_pending()
	return nil
}

// 先是封装各个池子的合并，再是具体的
func (txpool *TxPool) merge_qeue() error {
	for addr, _ := range txpool.queue {
		txpool.merge_queue_box(addr)
	}
	return nil
}

func (txpool *TxPool) merge_pending() error {
	for addr, _ := range txpool.pending {
		txpool.merge_pending_box(addr)
	}
	return nil
}

func (txpool *TxPool) merge_queue_box(address common.Address) error {
	//此函数是两个池子都兼容的合并模式
	//采用的是代替的方式进行
	boxes := txpool.queue[address]
	if len(boxes) == 0 {
		fmt.Println("no box in queue")
		return nil
	}
	lastbox := boxes[0]
	newboxes := make([]*TxBox, 0)
	newboxes = append(newboxes, lastbox)
	for _, box := range boxes {
		if len(box.txs) == 0 {
			continue
		}
		//这里应该是检查，上个盒子的lastnonce是否等于当前盒子的firstnonce-1
		//此外还要检查，是否满足当前的盒子的gasprice是否大于等于上个盒子
		//如果满足，那么就合并盒子

		lastBox := newboxes[len(newboxes)-1]
		isNextNonce := box.firstnonce == lastBox.lastnonce+1
		fmt.Println("isNedwasdwaxtNoncwasdwaeawsdwasdwasd", isNextNonce,lastBox.lastnonce+1)
		hasHigherGasPrice := box.gasprice.Cmp(lastBox.gasprice) >= 0
		fmt.Println("isNextNoncwasdwaeawsdwasdwasd", isNextNonce,hasHigherGasPrice)
		if isNextNonce && hasHigherGasPrice {
			//合并盒子
			for _, tx := range box.txs {
				newboxes[len(newboxes)-1].Enqueue(tx)
				box.txs = box.txs[1:]
				if len(box.txs) == 0 {
					txpool.queue[address] = txpool.queue[address][1:]
				}
			}
		} else {
			fmt.Println("add box123123dasdwa", box.firstnonce,lastBox.lastnonce+1)
			newboxes = append(newboxes, box)
		}
	}
	txpool.queue[address] = newboxes
	fmt.Println("newboxesawsdwasdwasaawdswadad", len(newboxes))
	return nil
}

func (txpool *TxPool) merge_pending_box(address common.Address) error {
	//此函数是两个池子都兼容的合并模式
	//采用的是代替的方式进行
	boxes := txpool.pending[address]
	if len(boxes) == 0 {
		fmt.Println("no box in pending")
		return nil
	}
	lastbox := boxes[0]
	newboxes := make([]*TxBox, 0)
	newboxes = append(newboxes, lastbox)
	for _, box := range boxes {
		//这里应该是检查，上个盒子的lastnonce是否等于当前盒子的firstnonce-1
		//此外还要检查，是否满足当前的盒子的gasprice是否大于等于上个盒子
		//如果满足，那么就合并盒子

		lastBox := newboxes[len(newboxes)-1]
		isNextNonce := box.firstnonce == lastBox.lastnonce+1
		hasHigherGasPrice := box.gasprice.Cmp(lastBox.gasprice) >= 0

		if isNextNonce && hasHigherGasPrice {
			//合并盒子
			for _, tx := range box.txs {
				newboxes[len(newboxes)-1].Enqueue(tx)
				box.txs = box.txs[1:]
				if len(box.txs) == 0 {
					txpool.pending[address] = txpool.pending[address][1:]
				}
			}
		} else {
			newboxes = append(newboxes, box)
		}
	}
	txpool.pending[address] = newboxes
	return nil
}

//--------------------------------------------------------------------------------------
//----------------------------------pending---------------------------------------------
//--------------------------------------------------------------------------------------

func (txpool *TxPool) pending_Max_gas_box() *TxBox {
	//这里实现把pending中最大gasprice的box拿出来（同时参考nonce，所以每个地址只拿第一个做比对）
	//从pending中拿出一个box，并将其从pending中删除
	boxes := txpool.take_first_boxes_out() //每个地址拿出来第一个
	maxbox := txpool.maxgas_in_arr(boxes)
	txpool.pending[maxbox.address] = txpool.pending[maxbox.address][1:]
	return maxbox
}

func (txpool *TxPool) take_first_boxes_out() []*TxBox {
	//把pending中的每个地址里面的第一个box拿出来，不操作原本的
	boxes := txpool.pending
	outboxes := make([]*TxBox, 0)
	for _, box := range boxes {
		outboxes = append(outboxes, box[0])
	}
	return outboxes
}

func (txpool *TxPool) maxgas_in_arr(boxes []*TxBox) *TxBox {
	//输出最大的gasprice的box，不操作原本的
	if len(boxes) == 0 {
		return nil
	}
	maxBox := boxes[0]
	for _, box := range boxes {
		if box.GetGasPrice().Cmp(maxBox.GetGasPrice()) > 0 {
			maxBox = box
		}
	}
	return maxBox
}

//--------------------------------------------------------------------------------------
//----------------------------------基础方法---------------------------------------------
//--------------------------------------------------------------------------------------

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

// GetAccountNonce 获取账户的nonce值
func (txPool *TxPool) GetAccountNonce(addr common.Address) (uint64, error) {
	return txPool.getaccountnonce(addr)
}

// GetPendingTxBoxes 获取pending池中的所有交易盒子
func (txPool *TxPool) GetPendingTxBoxes() []*TxBox {
	var allBoxes []*TxBox
	for _, boxes := range txPool.pending {
		fmt.Println("12332112312312asdwa", boxes)
		allBoxes = append(allBoxes, boxes...)
	}
	return allBoxes
}

// GetQueueTxBoxes 获取queue池中的所有交易盒子
func (txPool *TxPool) GetQueueTxBoxes() []*TxBox {
	var allBoxes []*TxBox
	for _, boxes := range txPool.queue {
		allBoxes = append(allBoxes, boxes...)
	}
	return allBoxes
}

// GetQueueTxBoxesByAddress 获取指定地址在queue中的交易盒子
func (txPool *TxPool) GetQueueTxBoxesByAddress(addr common.Address) []*TxBox {
	return txPool.queue[addr]
}

// Execute 执行指定数量的交易
func (txPool *TxPool) Execute(vm TransactionExecutor, num int) error {
	return txPool.execute(vm, num)
}
