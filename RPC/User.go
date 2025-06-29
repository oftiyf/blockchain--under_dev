package rpc

import (
	"blockchain/common"
	"blockchain/maker"
	"blockchain/tx"
	"fmt"
)

// 获取一个maker，然后往里面的交易池里面加东西即可，
// 理论上来说，这个应该的执行应该是节点进行执行，只要传入Transaction即可
// 但是这里为了方便演示，所以直接在rpc中执行
func UserRPC_transaction(maker *maker.BlockMaker, tx *tx.Transaction) {
	err := maker.Txpool.NewTX(tx)
	if err != nil {
		fmt.Println("UserRPC error:", err)
	}
}

func UserRPC_balance(maker *maker.BlockMaker, addr common.Address) uint64 {
	// 从状态数据库中获取账户信息
	addrBytes := addr.Bytes()
	accountBytes, err := maker.State.Get(addrBytes)
	if err != nil {
		fmt.Println("获取账户信息失败:", err)
		return 0
	}

	// 如果账户不存在，返回0余额
	if accountBytes == nil {
		fmt.Println("账户不存在，余额为0")
		return 0
	}

	// 反序列化账户信息
	account := common.Reserialize(accountBytes)
	fmt.Printf("账户 %s 的余额为: %d\n", addr.String(), account.Balance)

	return account.Balance
}
