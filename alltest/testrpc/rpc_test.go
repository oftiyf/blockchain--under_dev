package testrpc

import (
	rpc "blockchain/RPC"
	"blockchain/common"
	"blockchain/maker"
	"blockchain/mpt"
	"blockchain/tx"

	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
)

// 测试用的私钥（用于生成地址）
var testPrivateKeys = []string{
	"1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
	"7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456",
}

// 测试用的矿工私钥
const minerPrivateKey = "1111111111111111111111111111111111111111111111111111111111111111"

// 从十六进制字符串解码私钥并生成地址
func hexToAddress(hexKey string) common.Address {
	publicKeyBytes, err := common.PrivateKeyToPublicKey(hexKey)
	if err != nil {
		panic(fmt.Sprintf("生成地址失败: %v", err))
	}
	return common.Address{}.PublicKeyToAddress(publicKeyBytes)
}

var minerAddress = hexToAddress(minerPrivateKey)

const receiverPrivateKey = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

var receiverAddress = hexToAddress(receiverPrivateKey)

func get_block_maker() *maker.BlockMaker {
	dbDir := "test_db_rpc_complete"
	db, err := mpt.NewDB(dbDir)
	if err != nil {
		panic(fmt.Sprintf("创建数据库失败: %v", err))
	}
	state := mpt.NewMPT(db)
	txpool := tx.NewTxPool(state)
	blockMaker := maker.NewBlockMaker(txpool, state)
	return blockMaker
}

func TestRPC(t *testing.T) {
	blockMaker := get_block_maker()
	fmt.Println("BlockMaker创建成功")

	rpc.MinnerRPC(blockMaker, minerAddress)
	fmt.Println("第一次挖矿完成")

	minnerBalance := rpc.UserRPC_balance(blockMaker, minerAddress)
	fmt.Println("minnerBalance:", minnerBalance)

	// 接着创建一个交易，并且采用userrpc来执行交易，接着继续用minner来执行MinnerRPC

	// 创建一笔从矿工地址到接收者地址的交易
	value := big.NewInt(10000) // 转账1000000 wei
	gasPrice := big.NewInt(1)  // 20 gwei
	gasLimit := uint64(21000)  // 标准转账gas限制
	chainID := big.NewInt(1)   // 链ID为1

	fmt.Printf("创建交易: 从 %s 到 %s, 金额: %s\n", minerAddress.String(), receiverAddress.String(), value.String())

	// 创建交易
	transaction := tx.NewTransaction(
		1, // nonce从1开始，因为这是第一个交易
		receiverAddress,
		value,
		gasLimit,
		gasPrice,
		nil, // 没有data，这是普通转账
		chainID,
	)

	fmt.Println("交易创建成功")

	// 使用矿工私钥签名交易
	privateKeyBytes, err := hex.DecodeString(minerPrivateKey)
	if err != nil {
		t.Fatalf("解码私钥失败: %v", err)
	}

	fmt.Println("私钥解码成功")

	err = transaction.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("签名交易失败: %v", err)
	}
	fmt.Println("交易签名成功")

	// 使用UserRPC执行交易（将交易添加到交易池）
	rpc.UserRPC_transaction(blockMaker, transaction)
	fmt.Println("交易已添加到交易池")

	// 检查交易池状态
	fmt.Println("交易已成功添加到交易池")

	// 再次使用MinnerRPC打包交易
	rpc.MinnerRPC(blockMaker, minerAddress)
	fmt.Println("第二次挖矿完成")

	// 检查矿工和接收者的余额变化
	newMinnerBalance := rpc.UserRPC_balance(blockMaker, minerAddress)
	receiverBalance := rpc.UserRPC_balance(blockMaker, receiverAddress)

	fmt.Println("打包后矿工余额:", newMinnerBalance)
	fmt.Println("接收者余额:", receiverBalance)
}
