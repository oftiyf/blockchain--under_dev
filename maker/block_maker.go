package maker

import (
	"blockchain/block"
	"blockchain/common"
	"blockchain/mpt"
	"blockchain/tx"
	"blockchain/vm"
	"errors"
	"fmt"
	"time"
)

type ChainConfig struct {
	Duration time.Duration  //最长打包时间
	coinbase common.Address //矿工地址

}
type BlockMaker struct {
	Txpool      *tx.TxPool
	State       *mpt.MPT
	vm          *vm.VM
	chainConfig ChainConfig       //这里记录一些链的配置信息
	chain       *block.Blockchain //初始化应该为空
	nextHeader  *block.Header     //区块头，用于生成区块
	nextBody    *block.Body       //区块体，用于生成区块

	receiptions []*common.Hash //理论上应该是交易log之类的，但是我感觉是vm层里面的东西，所以说用这个暂时代替一下

	interrupt chan bool
}

func NewBlockMaker(txpool *tx.TxPool, state *mpt.MPT) *BlockMaker {
	//初始化所有字段
	return &BlockMaker{
		Txpool: txpool,
		State:  state,
		vm:     nil,
		chainConfig: ChainConfig{
			Duration: 10 * time.Second, //默认10秒打包时间
			coinbase: common.Address{}, //默认空地址
		},
		chain:       &block.Blockchain{},
		nextHeader:  nil,
		nextBody:    nil,
		receiptions: make([]*common.Hash, 0),
		interrupt:   make(chan bool),
	}
}

func (maker *BlockMaker) NewBlock() error {
	//这里设置了body和header
	maker.nextBody = block.NewBlock()
	fmt.Println("成功创建空区块体")
	maker.nextHeader = block.NewHeader(maker.chain.CurrentHeader)
	fmt.Println("成功创建空区块头")
	maker.nextHeader.Coinbase = maker.chainConfig.coinbase
	return nil
}

func (maker *BlockMaker) Pack() error {
	end := time.After(maker.chainConfig.Duration) //这里是为了使得，超过最长打包时间，就停止打包
	for i := 0; i < 100; i++ {                    //为了方便测试打印，这里设置为100次，不然会打印很多
		select {
		case <-maker.interrupt:
			break
		case <-end:
			break
		default:
			err := maker.pack()
			if err != nil {
				if err.Error() == "没有交易可打包" {
					fmt.Println("创世区块执行，交易池执行完毕")
					return nil
				} else {
					fmt.Println("第", i, "次打包失败，执行失败")
					return err
				}
			} else {
				fmt.Println("第", i, "次打包成功，执行成功")
			}
		}
	}

	return nil
}

func (maker *BlockMaker) pack() error {
	//小写的只取一个交易进行打包
	maker.vm = vm.NewVM(maker.State) //这里创建一个vm，用于执行交易
	fmt.Println("成功创建vm")
	tx := maker.Txpool.Pop()
	fmt.Println("成功取出交易")
	if tx == nil {
		fmt.Println("没有交易可打包")
		return errors.New("没有交易可打包")
	}
	err := maker.vm.ExecuteTransaction(tx) //注意，这里的mpt树等状态是在vm创建中的，所以这里不需要传入mpt树
	if err != nil {
		return err
	}
	maker.nextBody.Transactions = append(maker.nextBody.Transactions, *tx)
	maker.receiptions = append(maker.receiptions, tx.Hash()) //理论上应该是交易log之类的，但是我感觉是vm层里面的东西，所以说用这个暂时代替一下

	return nil
}

func (maker *BlockMaker) Interupt() {
	maker.interrupt <- true
}

func (maker *BlockMaker) Finshlist() (*block.Header, *block.Body) {
	//给minner调用的
	maker.nextHeader.Timestamp = uint64(time.Now().Unix()) //理论上应该再封装，此处省略
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

func (maker *BlockMaker) minnerRPC(minner common.Address, state *mpt.MPT) {
	//设置coinbase
	fmt.Println("minner", minner, "开始奖励矿工")
	maker.chainConfig.coinbase = minner //设置交易
	fmt.Println("minner", minner, "设置coinbase成功")
	maker.chainConfig.Duration = 1 * time.Second //设置打包时间
	fmt.Println("minner", minner, "设置打包时间成功")
	maker.vm = vm.NewVM(state)
	maker.vm.Mint(minner) //奖励矿工
	fmt.Println("minner", minner, "奖励矿工成功")
	maker.NewBlock() //生成新的初始区块默认值（主要是区块头）
	fmt.Println("minner", minner, "生成新的区块头成功")

	//先执行
	maker.Pack()
	fmt.Println("minner", minner, "打包成功")

	//然后打包
	header, body := maker.Finshlist()
	maker.chain.AddBlock(header, state, maker.Txpool)

	//然后广播
	maker.chain.Broadcast(header, body)
}

func (maker *BlockMaker) MinnerRPC(minner common.Address) uint64 {
	maker.minnerRPC(minner, maker.State)
	// 确保在返回高度之前，CurrentHeader已经被正确设置
	if maker.chain.CurrentHeader.Height == 0 && maker.nextHeader != nil {
		// 如果CurrentHeader还没有被设置，但nextHeader存在，说明这是第一次挖矿
		return 0
	}
	return maker.chain.CurrentHeader.Height
}
