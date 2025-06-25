package testtxpool

import (
	"blockchain/common"
	"blockchain/mpt"
	"blockchain/tx"
	"blockchain/vm"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

// 固定测试私钥
var testPrivateKeyHex = "0000000000000000000000000000000000000000000000000000000000000001"

// VM适配器，实现txpool的vm_execute接口
// 适配 Execute(tx *Transaction) error
// 实际调用 VM.ExecuteTransaction

type VMAdapter struct {
	VM *vm.VM
}

func (v *VMAdapter) Execute(txn *tx.Transaction) error {
	return v.VM.ExecuteTransaction(txn)
}

func setupTestEnv(t *testing.T) (pool *tx.TxPool, vma *VMAdapter, senderAddr common.Address, privateKeyBytes []byte) {
	t.Logf("[DEBUG] 开始创建测试数据库目录")
	dbDir := "test_db_txpool"
	os.MkdirAll(dbDir, 0755)
	t.Logf("[DEBUG] 数据库目录创建完成: %s", dbDir)

	t.Logf("[DEBUG] 开始创建MPT数据库")
	db, err := mpt.NewDB(filepath.Join(dbDir, "MPT_shared"))
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	t.Logf("[DEBUG] MPT数据库创建完成")

	t.Logf("[DEBUG] 开始创建状态数据库")
	stateDB := mpt.NewMPT(db)
	t.Logf("[DEBUG] 状态数据库创建完成")

	t.Logf("[DEBUG] 开始创建交易池")
	pool = &tx.TxPool{
		StatDB: stateDB,
	}
	t.Logf("[DEBUG] 交易池创建完成")

	t.Logf("[DEBUG] 开始创建VM适配器")
	vma = &VMAdapter{VM: vm.NewVM(stateDB)}
	t.Logf("[DEBUG] VM适配器创建完成")

	t.Logf("[DEBUG] 开始解析私钥")
	privateKeyBytes, err = hex.DecodeString(testPrivateKeyHex)
	if err != nil {
		t.Fatalf("Failed to decode private key: %v", err)
	}
	t.Logf("[DEBUG] 私钥解析完成")

	t.Logf("[DEBUG] 开始从私钥生成公钥")
	publicKey, err := common.PrivateKeyToPublicKey(testPrivateKeyHex)
	if err != nil {
		t.Fatalf("Failed to get public key: %v", err)
	}
	t.Logf("[DEBUG] 公钥生成完成")

	t.Logf("[DEBUG] 开始生成发送者地址")
	hash := common.Hash{}.NewHash(publicKey)
	senderAddr = common.Address{}.NewAddress(hash[:20])
	t.Logf("[DEBUG] 发送者地址生成完成: %s", senderAddr.String())

	t.Logf("[DEBUG] 开始给发送者mint代币")
	err = vma.VM.Mint(senderAddr)
	if err != nil {
		t.Fatalf("Failed to mint tokens: %v", err)
	}
	t.Logf("[DEBUG] 代币mint完成")

	return
}

func TestTxPool_NewTX_and_Execute(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Test panicked: %v", r)
		}
	}()

	t.Logf("[Step 1] 初始化环境")
	pool, vma, senderAddr, privateKeyBytes := setupTestEnv(t)
	t.Logf("[DEBUG] 环境初始化完成 - senderAddr: %s", senderAddr.String())

	t.Logf("[Step 2] 创建接收者地址")
	receiverBytes := make([]byte, 20)
	copy(receiverBytes, []byte("receiver"))
	receiver := common.Address{}.NewAddress(receiverBytes)
	t.Logf("[DEBUG] 接收者地址创建完成 - receiverAddr: %s", receiver.String())

	t.Logf("[Step 3] 构造并签名交易")
	tx1 := tx.NewTransaction(
		1,              // nonce
		receiver,       // to
		big.NewInt(50), // value
		1000,           // gasLimit
		big.NewInt(1),  // gasPrice
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)
	t.Logf("[DEBUG] 交易构造完成 - nonce: %d, to: %s, value: %s", tx1.Nonce, tx1.To.String(), tx1.Value.String())

	err := tx1.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}
	txHash, err := tx1.GetHash()
	if err != nil {
		t.Fatalf("Failed to get transaction hash: %v", err)
	}
	t.Logf("[DEBUG] 交易签名完成 - hash: %s", txHash.String())

	t.Logf("[Step 4] 添加到TxPool.NewTX")
	err = pool.NewTX(tx1)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed: %v", err)
	}
	t.Logf("[DEBUG] 交易已添加到TxPool")

	t.Logf("[Step 5] 执行TxPool.Execute")
	err = pool.Execute(vma, 1)
	if err != nil {
		t.Fatalf("TxPool.Execute failed: %v", err)
	}
	t.Logf("[DEBUG] TxPool.Execute执行完成")

	t.Logf("[Step 6] 验证余额变化")
	t.Logf("[DEBUG] 开始获取发送者账户信息")
	senderAccount, err := vma.VM.GetAccount(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get sender account: %v", err)
	}
	t.Logf("[DEBUG] 发送者账户获取成功 - balance: %d, nonce: %d", senderAccount.Balance, senderAccount.Nonce)

	t.Logf("[DEBUG] 开始获取接收者账户信息")
	receiverAccount, err := vma.VM.GetAccount(receiver)
	if err != nil {
		t.Fatalf("Failed to get receiver account: %v", err)
	}
	t.Logf("[DEBUG] 接收者账户获取成功 - balance: %d, nonce: %d", receiverAccount.Balance, receiverAccount.Nonce)

	gasCost := uint64(tx1.GasLimit) * tx1.GasPrice.Uint64()
	expectedSenderBalance := uint64(1000000) - (uint64(50) + gasCost)
	expectedReceiverBalance := uint64(50)

	t.Logf("[DEBUG] 计算预期值 - gasCost: %d, expectedSenderBalance: %d, expectedReceiverBalance: %d",
		gasCost, expectedSenderBalance, expectedReceiverBalance)

	if senderAccount.Balance != expectedSenderBalance {
		t.Errorf("Sender balance incorrect. Got %v, want %v", senderAccount.Balance, expectedSenderBalance)
	} else {
		t.Logf("[DEBUG] 发送者余额验证通过")
	}
	if receiverAccount.Balance != expectedReceiverBalance {
		t.Errorf("Receiver balance incorrect. Got %v, want %v", receiverAccount.Balance, expectedReceiverBalance)
	} else {
		t.Logf("[DEBUG] 接收者余额验证通过")
	}
	if senderAccount.Nonce != 1 {
		t.Errorf("Sender nonce incorrect. Got %v, want 1", senderAccount.Nonce)
	} else {
		t.Logf("[DEBUG] 发送者nonce验证通过")
	}

	t.Logf("[DEBUG] 所有验证完成，测试通过")

	// 添加更多交易测试
	t.Logf("[Step 7] 添加第二笔交易")
	tx2 := tx.NewTransaction(
		2,              // nonce
		receiver,       // to
		big.NewInt(30), // value
		800,            // gasLimit
		big.NewInt(1),  // gasPrice
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)
	t.Logf("[DEBUG] 第二笔交易构造完成 - nonce: %d, to: %s, value: %s", tx2.Nonce, tx2.To.String(), tx2.Value.String())

	err = tx2.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign second transaction: %v", err)
	}
	tx2Hash, err := tx2.GetHash()
	if err != nil {
		t.Fatalf("Failed to get second transaction hash: %v", err)
	}
	t.Logf("[DEBUG] 第二笔交易签名完成 - hash: %s", tx2Hash.String())

	err = pool.NewTX(tx2)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for second transaction: %v", err)
	}
	t.Logf("[DEBUG] 第二笔交易已添加到TxPool")

	t.Logf("[Step 8] 添加第三笔交易")
	tx3 := tx.NewTransaction(
		3,              // nonce
		receiver,       // to
		big.NewInt(20), // value
		600,            // gasLimit
		big.NewInt(1),  // gasPrice
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)
	t.Logf("[DEBUG] 第三笔交易构造完成 - nonce: %d, to: %s, value: %s", tx3.Nonce, tx3.To.String(), tx3.Value.String())

	err = tx3.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign third transaction: %v", err)
	}
	tx3Hash, err := tx3.GetHash()
	if err != nil {
		t.Fatalf("Failed to get third transaction hash: %v", err)
	}
	t.Logf("[DEBUG] 第三笔交易签名完成 - hash: %s", tx3Hash.String())

	err = pool.NewTX(tx3)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for third transaction: %v", err)
	}
	t.Logf("[DEBUG] 第三笔交易已添加到TxPool")

	t.Logf("[Step 9] 执行所有交易")
	err = pool.Execute(vma, 3)
	if err != nil {
		t.Fatalf("TxPool.Execute failed for all transactions: %v", err)
	}
	t.Logf("[DEBUG] 所有交易执行完成")

	t.Logf("[Step 10] 验证最终余额变化")
	senderAccount, err = vma.VM.GetAccount(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get sender account: %v", err)
	}
	receiverAccount, err = vma.VM.GetAccount(receiver)
	if err != nil {
		t.Fatalf("Failed to get receiver account: %v", err)
	}

	// 计算所有交易的gas费用
	gasCost1 := uint64(tx1.GasLimit) * tx1.GasPrice.Uint64()
	gasCost2 := uint64(tx2.GasLimit) * tx2.GasPrice.Uint64()
	gasCost3 := uint64(tx3.GasLimit) * tx3.GasPrice.Uint64()
	totalGasCost := gasCost1 + gasCost2 + gasCost3

	// 计算转账总额
	totalTransfer := uint64(50) + uint64(30) + uint64(20)

	expectedSenderBalance = uint64(1000000) - totalTransfer - totalGasCost
	expectedReceiverBalance = totalTransfer

	t.Logf("[DEBUG] 最终计算 - totalGasCost: %d, totalTransfer: %d, expectedSenderBalance: %d, expectedReceiverBalance: %d",
		totalGasCost, totalTransfer, expectedSenderBalance, expectedReceiverBalance)

	if senderAccount.Balance != expectedSenderBalance {
		t.Errorf("Final sender balance incorrect. Got %v, want %v", senderAccount.Balance, expectedSenderBalance)
	} else {
		t.Logf("[DEBUG] 最终发送者余额验证通过")
	}
	if receiverAccount.Balance != expectedReceiverBalance {
		t.Errorf("Final receiver balance incorrect. Got %v, want %v", receiverAccount.Balance, expectedReceiverBalance)
	} else {
		t.Logf("[DEBUG] 最终接收者余额验证通过")
	}
	if senderAccount.Nonce != 3 {
		t.Errorf("Final sender nonce incorrect. Got %v, want 3", senderAccount.Nonce)
	} else {
		t.Logf("[DEBUG] 最终发送者nonce验证通过")
	}

	t.Logf("[DEBUG] 多笔交易测试完成，所有验证通过")
}


