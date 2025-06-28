package maker

import (
	"blockchain/block"
	"blockchain/common"
	"blockchain/mpt"
	"blockchain/tx"
	"blockchain/vm"
	"time"
)
type ChainConfig struct {
	Duration time.Duration//最长打包时间
	coinbase common.Address//矿工地址
	
}
type BlockMaker struct {
	txpool *tx.TxPool
	state *mpt.MPT
	vm *vm.VM
	chainConfig ChainConfig//这里记录一些链的配置信息
	chain *block.Blockchain//初始化应该为空
	nextHeader *block.Header//区块头，用于生成区块
	nextBody *block.Body//区块体，用于生成区块

	receiptions [] *common.Hash//理论上应该是交易log之类的，但是我感觉是vm层里面的东西，所以说用这个暂时代替一下

	interrupt chan bool
}

func NewBlockMaker(txpool *tx.TxPool, state *mpt.MPT) *BlockMaker {
	
	return &BlockMaker{txpool: txpool, state: state}
}


func (maker *BlockMaker) NewBlock() error {
	//这里设置了body和header
	maker.nextBody = block.NewBlock()
	maker.nextHeader = block.NewHeader(maker.chain.CurrentHeader)
	maker.nextHeader.Coinbase = maker.chainConfig.coinbase
	return nil
}

func (maker *BlockMaker) Pack() error {
	end:=time.After(maker.chainConfig.Duration)//这里是为了使得，超过最长打包时间，就停止打包
	for {
		select {
		case <-maker.interrupt:
			break
		case <-end:
			break
		default:
			maker.pack()
		}
	}
	return nil
}


func (maker *BlockMaker) pack()  {
	//小写的只取一个交易进行打包
	maker.vm=vm.NewVM(maker.state)//这里创建一个vm，用于执行交易
	tx := maker.txpool.Pop()
	maker.vm.ExecuteTransaction(tx)//注意，这里的mpt树等状态是在vm创建中的，所以这里不需要传入mpt树
	maker.nextBody.Transactions = append(maker.nextBody.Transactions, *tx)
	maker.receiptions = append(maker.receiptions, tx.Hash())//理论上应该是交易log之类的，但是我感觉是vm层里面的东西，所以说用这个暂时代替一下
}

func (maker *BlockMaker) Interupt() {
	maker.interrupt <- true
}

func (maker *BlockMaker) Finshlist() (*block.Header, *block.Body){
	//给minner调用的
	maker.nextHeader.Timestamp = uint64(time.Now().Unix())//理论上应该再封装，此处省略
	maker.nextHeader.Nonce = 0
	//下面循环使得新nonce合格
	nonce := maker.nextHeader.Nonce
	for {
		nonce++
		maker.nextHeader.Nonce = nonce
		if maker.validNonce(maker.nextHeader.Hash()) {
			break
		}
	}
	return maker.nextHeader, maker.nextBody
}

func (maker *BlockMaker) validNonce(hash common.Hash) bool {
	//这里理论上来说要写一些检查逻辑，对它的哈希逻辑做出检查，这里省略
	return true
}


func (maker *BlockMaker) MinnerRPC(minner common.Address,state *mpt.MPT) {
	//设置coinbase

	maker.chainConfig.coinbase = minner//设置交易
	maker.chainConfig.Duration = 3 * time.Second//设置打包时间
	maker.vm.Mint(minner)//奖励矿工
	maker.NewBlock()//生成新的区块

	//先执行
	maker.Pack()

	//然后打包
	header, body := maker.Finshlist()
	maker.chain.AddBlock(header, body,state,maker.txpool)

	//然后广播
	maker.chain.Broadcast(header, body)
}




