package block

import (
	"blockchain/common"
	"blockchain/mpt"
	"blockchain/tx"
	"github.com/ethereum/go-ethereum/rlp"
)

type Header struct {
	Root       common.Hash//状态树的根节点
	ParentHash common.Hash //前一个区块的哈希值
	Height     uint64
	Coinbase   common.Address//矿工地址

	Timestamp  uint64
	Nonce      uint64
}

type Body struct {
	Transactions []tx.Transaction
}

func (header Header) Hash() common.Hash {
	data, err := rlp.EncodeToBytes(header)
	if err != nil {
		return common.Hash{}
	}
	return common.Hash{}.NewHash(data)
}

func NewHeader(parent Header) *Header {
	return &Header{
		Root:       parent.Root,
		ParentHash: parent.Hash(),
		Height:     parent.Height + 1,
	}
}

func NewBlock() *Body {
	return &Body{
		Transactions: make([]tx.Transaction, 0),
	}
}

type Blockchain struct {
	CurrentHeader Header
	Statedb      *mpt.MPT
	Txpool       *tx.TxPool
}




func (chain *Blockchain) AddBlock(header *Header, body *Body,state *mpt.MPT,txpool *tx.TxPool) {
	chain.CurrentHeader = *header
	chain.Statedb=state
	chain.Txpool=txpool
}

func (chain *Blockchain) Broadcast(header *Header, body *Body) error{
	//要广播，但是没实现这里，这里先空着
	return nil
}