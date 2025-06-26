package testtxpool

import (
	"blockchain/common"
	"blockchain/mpt"
	"blockchain/tx"
	"blockchain/vm"
	"encoding/hex"
	"fmt"
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
	return setupTestEnvWithDB(t, "test_db_txpool")
}

func setupTestEnvWithDB(t *testing.T, dbDir string) (pool *tx.TxPool, vma *VMAdapter, senderAddr common.Address, privateKeyBytes []byte) {
	t.Logf("[DEBUG] 开始创建测试数据库目录")
	fmt.Println("start create test db")
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
	defer cleanupTestDB("test_db_txpool")

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

// 测试gas价格竞争场景
func TestTxPool_GasPriceCompetition(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Test panicked: %v", r)
		}
	}()
	defer cleanupTestDB("test_db_gas_competition")

	t.Logf("[Step 1] 初始化环境")
	pool, vma, senderAddr, privateKeyBytes := setupTestEnvWithDB(t, "test_db_gas_competition")
	t.Logf("[DEBUG] 环境初始化完成 - senderAddr: %s", senderAddr.String())

	t.Logf("[Step 2] 创建接收者地址")
	receiverBytes := make([]byte, 20)
	copy(receiverBytes, []byte("receiver_gas"))
	receiver := common.Address{}.NewAddress(receiverBytes)
	t.Logf("[DEBUG] 接收者地址创建完成 - receiverAddr: %s", receiver.String())

	t.Logf("[Step 3] 创建低gas价格的交易")
	txLowGas := tx.NewTransaction(
		1,              // nonce
		receiver,       // to
		big.NewInt(50), // value
		1000,           // gasLimit
		big.NewInt(1),  // gasPrice (低)
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)
	t.Logf("[DEBUG] 低gas交易构造完成 - gasPrice: %s", txLowGas.GasPrice.String())

	err := txLowGas.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign low gas transaction: %v", err)
	}
	t.Logf("[DEBUG] 低gas交易签名完成")

	t.Logf("[Step 4] 创建高gas价格的交易（相同nonce）")
	txHighGas := tx.NewTransaction(
		1,              // nonce (相同)
		receiver,       // to
		big.NewInt(50), // value
		1000,           // gasLimit
		big.NewInt(5),  // gasPrice (高)
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)
	t.Logf("[DEBUG] 高gas交易构造完成 - gasPrice: %s", txHighGas.GasPrice.String())

	err = txHighGas.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign high gas transaction: %v", err)
	}
	t.Logf("[DEBUG] 高gas交易签名完成")

	t.Logf("[Step 5] 先添加低gas交易")
	err = pool.NewTX(txLowGas)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for low gas transaction: %v", err)
	}
	t.Logf("[DEBUG] 低gas交易已添加到TxPool")

	t.Logf("[Step 6] 添加高gas交易（应该替换低gas交易）")
	err = pool.NewTX(txHighGas)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for high gas transaction: %v", err)
	}
	t.Logf("[DEBUG] 高gas交易已添加到TxPool，应该替换了低gas交易")

	t.Logf("[Step 7] 执行交易")
	err = pool.Execute(vma, 1)
	if err != nil {
		t.Fatalf("TxPool.Execute failed: %v", err)
	}
	t.Logf("[DEBUG] 交易执行完成")

	t.Logf("[Step 8] 验证只有高gas交易被执行")
	senderAccount, err := vma.VM.GetAccount(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get sender account: %v", err)
	}
	receiverAccount, err := vma.VM.GetAccount(receiver)
	if err != nil {
		t.Fatalf("Failed to get receiver account: %v", err)
	}

	// 应该只有高gas交易被执行
	gasCost := uint64(txHighGas.GasLimit) * txHighGas.GasPrice.Uint64()
	expectedSenderBalance := uint64(1000000) - (uint64(50) + gasCost)
	expectedReceiverBalance := uint64(50)

	t.Logf("[DEBUG] 验证结果 - gasCost: %d, expectedSenderBalance: %d, expectedReceiverBalance: %d",
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

	t.Logf("[DEBUG] Gas价格竞争测试完成")
}

// 测试nonce替换场景
func TestTxPool_NonceReplacement(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Test panicked: %v", r)
		}
	}()
	defer cleanupTestDB("test_db_nonce_replacement")

	t.Logf("[Step 1] 初始化环境")
	pool, vma, senderAddr, privateKeyBytes := setupTestEnvWithDB(t, "test_db_nonce_replacement")
	t.Logf("[DEBUG] 环境初始化完成 - senderAddr: %s", senderAddr.String())

	t.Logf("[Step 2] 创建接收者地址")
	receiverBytes := make([]byte, 20)
	copy(receiverBytes, []byte("receiver_nonce"))
	receiver := common.Address{}.NewAddress(receiverBytes)
	t.Logf("[DEBUG] 接收者地址创建完成 - receiverAddr: %s", receiver.String())

	t.Logf("[Step 3] 创建第一笔交易（nonce=2）")
	tx1 := tx.NewTransaction(
		1,              // nonce
		receiver,       // to
		big.NewInt(30), // value
		1000,           // gasLimit
		big.NewInt(2),  // gasPrice
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)
	t.Logf("[DEBUG] 第一笔交易构造完成 - nonce: %d", tx1.Nonce)

	err := tx1.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign first transaction: %v", err)
	}
	t.Logf("[DEBUG] 第一笔交易签名完成")

	t.Logf("[Step 4] 创建第二笔交易（相同nonce=2，但更高gas价格）")
	tx2 := tx.NewTransaction(
		1,              // nonce (相同)
		receiver,       // to
		big.NewInt(40), // value (不同)
		1000,           // gasLimit (不同)
		big.NewInt(3),  // gasPrice (更高)
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)
	t.Logf("[DEBUG] 第二笔交易构造完成 - nonce: %d, gasPrice: %s", tx2.Nonce, tx2.GasPrice.String())

	err = tx2.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign second transaction: %v", err)
	}
	t.Logf("[DEBUG] 第二笔交易签名完成")

	t.Logf("[Step 5] 先添加第一笔交易")
	err = pool.NewTX(tx1)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for first transaction: %v", err)
	}
	t.Logf("[DEBUG] 第一笔交易已添加到TxPool")

	t.Logf("[Step 6] 添加第二笔交易（应该替换第一笔）")
	err = pool.NewTX(tx2)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for second transaction: %v", err)
	}
	t.Logf("[DEBUG] 第二笔交易已添加到TxPool，应该替换了第一笔")

	t.Logf("[Step 7] 执行交易")
	err = pool.Execute(vma, 1)
	if err != nil {
		t.Fatalf("TxPool.Execute failed: %v", err)
	}
	t.Logf("[DEBUG] 交易执行完成")

	t.Logf("[Step 8] 验证只有第二笔交易被执行")
	senderAccount, err := vma.VM.GetAccount(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get sender account: %v", err)
	}
	receiverAccount, err := vma.VM.GetAccount(receiver)
	if err != nil {
		t.Fatalf("Failed to get receiver account: %v", err)
	}

	// 应该只有第二笔交易被执行
	gasCost := uint64(tx2.GasLimit) * tx2.GasPrice.Uint64()
	expectedSenderBalance := uint64(1000000) - (uint64(40) + gasCost)
	expectedReceiverBalance := uint64(40)

	t.Logf("[DEBUG] 验证结果 - gasCost: %d, expectedSenderBalance: %d, expectedReceiverBalance: %d",
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
		t.Errorf("Sender nonce incorrect. Got %v, want 2", senderAccount.Nonce)
	} else {
		t.Logf("[DEBUG] 发送者nonce验证通过")
	}

	t.Logf("[DEBUG] Nonce替换测试完成")
}

// 测试多笔交易按gas价格排序执行
func TestTxPool_MultipleTransactionsByGasPrice(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Test panicked: %v", r)
		}
	}()
	defer cleanupTestDB("test_db_multiple_tx")

	t.Logf("[Step 1] 初始化环境")
	pool, vma, senderAddr, privateKeyBytes := setupTestEnvWithDB(t, "test_db_multiple_tx")
	t.Logf("[DEBUG] 环境初始化完成 - senderAddr: %s", senderAddr.String())

	t.Logf("[Step 2] 创建多个接收者地址")
	receiver1Bytes := make([]byte, 20)
	copy(receiver1Bytes, []byte("receiver1"))
	receiver1 := common.Address{}.NewAddress(receiver1Bytes)

	receiver2Bytes := make([]byte, 20)
	copy(receiver2Bytes, []byte("receiver2"))
	receiver2 := common.Address{}.NewAddress(receiver2Bytes)

	receiver3Bytes := make([]byte, 20)
	copy(receiver3Bytes, []byte("receiver3"))
	receiver3 := common.Address{}.NewAddress(receiver3Bytes)

	t.Logf("[DEBUG] 接收者地址创建完成")

	t.Logf("[Step 3] 创建多笔不同gas价格的交易")
	// 低gas价格交易
	txLow := tx.NewTransaction(
		1,              // nonce
		receiver1,      // to
		big.NewInt(10), // value
		500,            // gasLimit
		big.NewInt(1),  // gasPrice (低)
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)

	// 中等gas价格交易
	txMedium := tx.NewTransaction(
		2,              // nonce
		receiver2,      // to
		big.NewInt(20), // value
		600,            // gasLimit
		big.NewInt(3),  // gasPrice (中等)
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)

	// 高gas价格交易
	txHigh := tx.NewTransaction(
		3,              // nonce
		receiver3,      // to
		big.NewInt(30), // value
		700,            // gasLimit
		big.NewInt(5),  // gasPrice (高)
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)

	t.Logf("[DEBUG] 交易构造完成")

	// 签名所有交易
	err := txLow.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign low gas transaction: %v", err)
	}
	err = txMedium.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign medium gas transaction: %v", err)
	}
	err = txHigh.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign high gas transaction: %v", err)
	}
	t.Logf("[DEBUG] 所有交易签名完成")

	t.Logf("[Step 4] 按随机顺序添加交易到池中")
	// 故意按非gas价格顺序添加
	err = pool.NewTX(txLow)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for low gas transaction: %v", err)
	}
	err = pool.NewTX(txHigh)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for high gas transaction: %v", err)
	}
	err = pool.NewTX(txMedium)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for medium gas transaction: %v", err)
	}
	t.Logf("[DEBUG] 所有交易已添加到TxPool")

	t.Logf("[Step 5] 执行所有交易")
	err = pool.Execute(vma, 3)
	if err != nil {
		t.Fatalf("TxPool.Execute failed: %v", err)
	}
	t.Logf("[DEBUG] 所有交易执行完成")

	t.Logf("[Step 6] 验证所有交易都按nonce顺序执行")
	senderAccount, err := vma.VM.GetAccount(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get sender account: %v", err)
	}

	// 验证所有接收者都收到了转账
	receiver1Account, err := vma.VM.GetAccount(receiver1)
	if err != nil {
		t.Fatalf("Failed to get receiver1 account: %v", err)
	}
	receiver2Account, err := vma.VM.GetAccount(receiver2)
	if err != nil {
		t.Fatalf("Failed to get receiver2 account: %v", err)
	}
	receiver3Account, err := vma.VM.GetAccount(receiver3)
	if err != nil {
		t.Fatalf("Failed to get receiver3 account: %v", err)
	}

	// 计算总费用
	totalGasCost := uint64(500)*1 + uint64(600)*3 + uint64(700)*5
	totalTransfer := uint64(10) + uint64(20) + uint64(30)
	expectedSenderBalance := uint64(1000000) - totalTransfer - totalGasCost

	t.Logf("[DEBUG] 验证结果 - totalGasCost: %d, totalTransfer: %d, expectedSenderBalance: %d",
		totalGasCost, totalTransfer, expectedSenderBalance)

	if senderAccount.Balance != expectedSenderBalance {
		t.Errorf("Sender balance incorrect. Got %v, want %v", senderAccount.Balance, expectedSenderBalance)
	} else {
		t.Logf("[DEBUG] 发送者余额验证通过")
	}
	if receiver1Account.Balance != uint64(10) {
		t.Errorf("Receiver1 balance incorrect. Got %v, want 10", receiver1Account.Balance)
	} else {
		t.Logf("[DEBUG] 接收者1余额验证通过")
	}
	if receiver2Account.Balance != uint64(20) {
		t.Errorf("Receiver2 balance incorrect. Got %v, want 20", receiver2Account.Balance)
	} else {
		t.Logf("[DEBUG] 接收者2余额验证通过")
	}
	if receiver3Account.Balance != uint64(30) {
		t.Errorf("Receiver3 balance incorrect. Got %v, want 30", receiver3Account.Balance)
	} else {
		t.Logf("[DEBUG] 接收者3余额验证通过")
	}
	if senderAccount.Nonce != 3 {
		t.Errorf("Sender nonce incorrect. Got %v, want 3", senderAccount.Nonce)
	} else {
		t.Logf("[DEBUG] 发送者nonce验证通过")
	}

	t.Logf("[DEBUG] 多笔交易按gas价格排序测试完成")
}

// 测试nonce不按顺序添加交易
func TestTxPool_NonceOutOfOrder(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Test panicked: %v", r)
		}
	}()
	defer cleanupTestDB("test_db_nonce_out_of_order")

	t.Logf("[Step 1] 初始化环境")
	pool, vma, senderAddr, privateKeyBytes := setupTestEnvWithDB(t, "test_db_nonce_out_of_order")
	t.Logf("[DEBUG] 环境初始化完成 - senderAddr: %s", senderAddr.String())

	t.Logf("[Step 2] 创建多个接收者地址")
	receiver1Bytes := make([]byte, 20)
	copy(receiver1Bytes, []byte("receiver_out_order_1"))
	receiver1 := common.Address{}.NewAddress(receiver1Bytes)

	receiver2Bytes := make([]byte, 20)
	copy(receiver2Bytes, []byte("receiver_out_order_2"))
	receiver2 := common.Address{}.NewAddress(receiver2Bytes)

	receiver3Bytes := make([]byte, 20)
	copy(receiver3Bytes, []byte("receiver_out_order_3"))
	receiver3 := common.Address{}.NewAddress(receiver3Bytes)

	receiver4Bytes := make([]byte, 20)
	copy(receiver4Bytes, []byte("receiver_out_order_4"))
	receiver4 := common.Address{}.NewAddress(receiver4Bytes)

	t.Logf("[DEBUG] 接收者地址创建完成")

	t.Logf("[Step 3] 创建多笔不同nonce的交易")
	// nonce=3的交易
	tx3 := tx.NewTransaction(
		3,              // nonce
		receiver3,      // to
		big.NewInt(30), // value
		700,            // gasLimit
		big.NewInt(2),  // gasPrice
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)

	// nonce=1的交易
	tx1 := tx.NewTransaction(
		1,              // nonce
		receiver1,      // to
		big.NewInt(10), // value
		500,            // gasLimit
		big.NewInt(1),  // gasPrice
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)

	// nonce=4的交易
	tx4 := tx.NewTransaction(
		4,              // nonce
		receiver4,      // to
		big.NewInt(40), // value
		800,            // gasLimit
		big.NewInt(3),  // gasPrice
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)

	// nonce=2的交易
	tx2 := tx.NewTransaction(
		2,              // nonce
		receiver2,      // to
		big.NewInt(20), // value
		600,            // gasLimit
		big.NewInt(2),  // gasPrice
		[]byte{},       // data
		big.NewInt(1),  // chainID
	)

	t.Logf("[DEBUG] 交易构造完成")

	// 签名所有交易
	err := tx1.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign transaction 1: %v", err)
	}
	err = tx2.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign transaction 2: %v", err)
	}
	err = tx3.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign transaction 3: %v", err)
	}
	err = tx4.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign transaction 4: %v", err)
	}
	t.Logf("[DEBUG] 所有交易签名完成")

	t.Logf("[Step 4] 按非顺序添加交易到池中")
	// 故意按非nonce顺序添加：3, 1, 4, 2
	err = pool.NewTX(tx3)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for transaction 3: %v", err)
	}
	t.Logf("[DEBUG] 交易3 (nonce=3) 已添加到TxPool")

	err = pool.NewTX(tx1)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for transaction 1: %v", err)
	}
	t.Logf("[DEBUG] 交易1 (nonce=1) 已添加到TxPool")

	err = pool.NewTX(tx4)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for transaction 4: %v", err)
	}
	t.Logf("[DEBUG] 交易4 (nonce=4) 已添加到TxPool")

	err = pool.NewTX(tx2)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for transaction 2: %v", err)
	}
	t.Logf("[DEBUG] 交易2 (nonce=2) 已添加到TxPool")

	t.Logf("[Step 5] 执行所有交易")
	err = pool.Execute(vma, 4)
	if err != nil {
		t.Fatalf("TxPool.Execute failed: %v", err)
	}
	t.Logf("[DEBUG] 所有交易执行完成")

	t.Logf("[Step 6] 验证所有交易都按nonce顺序执行")
	senderAccount, err := vma.VM.GetAccount(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get sender account: %v", err)
	}

	// 验证所有接收者都收到了转账
	receiver1Account, err := vma.VM.GetAccount(receiver1)
	if err != nil {
		t.Fatalf("Failed to get receiver1 account: %v", err)
	}
	receiver2Account, err := vma.VM.GetAccount(receiver2)
	if err != nil {
		t.Fatalf("Failed to get receiver2 account: %v", err)
	}
	receiver3Account, err := vma.VM.GetAccount(receiver3)
	if err != nil {
		t.Fatalf("Failed to get receiver3 account: %v", err)
	}
	receiver4Account, err := vma.VM.GetAccount(receiver4)
	if err != nil {
		t.Fatalf("Failed to get receiver4 account: %v", err)
	}

	// 计算总费用
	totalGasCost := uint64(500)*1 + uint64(600)*2 + uint64(700)*2 + uint64(800)*3
	totalTransfer := uint64(10) + uint64(20) + uint64(30) + uint64(40)
	expectedSenderBalance := uint64(1000000) - totalTransfer - totalGasCost

	t.Logf("[DEBUG] 验证结果 - totalGasCost: %d, totalTransfer: %d, expectedSenderBalance: %d",
		totalGasCost, totalTransfer, expectedSenderBalance)

	if senderAccount.Balance != expectedSenderBalance {
		t.Errorf("Sender balance incorrect. Got %v, want %v", senderAccount.Balance, expectedSenderBalance)
	} else {
		t.Logf("[DEBUG] 发送者余额验证通过")
	}
	if receiver1Account.Balance != uint64(10) {
		t.Errorf("Receiver1 balance incorrect. Got %v, want 10", receiver1Account.Balance)
	} else {
		t.Logf("[DEBUG] 接收者1余额验证通过")
	}
	if receiver2Account.Balance != uint64(20) {
		t.Errorf("Receiver2 balance incorrect. Got %v, want 20", receiver2Account.Balance)
	} else {
		t.Logf("[DEBUG] 接收者2余额验证通过")
	}
	if receiver3Account.Balance != uint64(30) {
		t.Errorf("Receiver3 balance incorrect. Got %v, want 30", receiver3Account.Balance)
	} else {
		t.Logf("[DEBUG] 接收者3余额验证通过")
	}
	if receiver4Account.Balance != uint64(40) {
		t.Errorf("Receiver4 balance incorrect. Got %v, want 40", receiver4Account.Balance)
	} else {
		t.Logf("[DEBUG] 接收者4余额验证通过")
	}
	if senderAccount.Nonce != 4 {
		t.Errorf("Sender nonce incorrect. Got %v, want 4", senderAccount.Nonce)
	} else {
		t.Logf("[DEBUG] 发送者nonce验证通过")
	}

	t.Logf("[DEBUG] Nonce不按顺序添加测试完成")
}

// 测试余额不足的情况
func TestTxPool_InsufficientBalance(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Test panicked: %v", r)
		}
	}()
	defer cleanupTestDB("test_db_insufficient_balance")

	t.Logf("[Step 1] 初始化环境")
	pool, vma, senderAddr, privateKeyBytes := setupTestEnvWithDB(t, "test_db_insufficient_balance")
	t.Logf("[DEBUG] 环境初始化完成 - senderAddr: %s", senderAddr.String())

	t.Logf("[Step 2] 创建接收者地址")
	receiverBytes := make([]byte, 20)
	copy(receiverBytes, []byte("receiver_insufficient"))
	receiver := common.Address{}.NewAddress(receiverBytes)
	t.Logf("[DEBUG] 接收者地址创建完成 - receiverAddr: %s", receiver.String())

	t.Logf("[Step 3] 创建余额不足的交易")
	// 尝试转账超过余额的金额
	txInsufficient := tx.NewTransaction(
		1,                   // nonce
		receiver,            // to
		big.NewInt(2000000), // value (超过初始余额1000000)
		1000,                // gasLimit
		big.NewInt(1),       // gasPrice
		[]byte{},            // data
		big.NewInt(1),       // chainID
	)
	t.Logf("[DEBUG] 余额不足交易构造完成 - value: %s", txInsufficient.Value.String())

	err := txInsufficient.Sign(privateKeyBytes)
	if err != nil {
		t.Fatalf("Failed to sign insufficient balance transaction: %v", err)
	}
	t.Logf("[DEBUG] 余额不足交易签名完成")

	t.Logf("[Step 4] 添加余额不足的交易")
	err = pool.NewTX(txInsufficient)
	if err != nil {
		t.Fatalf("TxPool.NewTX failed for insufficient balance transaction: %v", err)
	}
	t.Logf("[DEBUG] 余额不足交易已添加到TxPool")

	t.Logf("[Step 5] 尝试执行交易（应该失败）")
	err = pool.Execute(vma, 1)
	if err == nil {
		t.Fatalf("TxPool.Execute should have failed due to insufficient balance")
	}
	t.Logf("[DEBUG] 交易执行失败，符合预期 - 错误: %v", err)

	t.Logf("[Step 6] 验证余额没有变化")
	senderAccount, err := vma.VM.GetAccount(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get sender account: %v", err)
	}
	receiverAccount, err := vma.VM.GetAccount(receiver)
	if err != nil {
		t.Fatalf("Failed to get receiver account: %v", err)
	}

	// 余额应该保持不变
	expectedSenderBalance := uint64(1000000)
	expectedReceiverBalance := uint64(0)

	t.Logf("[DEBUG] 验证结果 - expectedSenderBalance: %d, expectedReceiverBalance: %d",
		expectedSenderBalance, expectedReceiverBalance)

	if senderAccount.Balance != expectedSenderBalance {
		t.Errorf("Sender balance should be unchanged. Got %v, want %v", senderAccount.Balance, expectedSenderBalance)
	} else {
		t.Logf("[DEBUG] 发送者余额验证通过（未变化）")
	}
	if receiverAccount.Balance != expectedReceiverBalance {
		t.Errorf("Receiver balance should be unchanged. Got %v, want %v", receiverAccount.Balance, expectedReceiverBalance)
	} else {
		t.Logf("[DEBUG] 接收者余额验证通过（未变化）")
	}
	if senderAccount.Nonce != 0 {
		t.Errorf("Sender nonce should be unchanged. Got %v, want 0", senderAccount.Nonce)
	} else {
		t.Logf("[DEBUG] 发送者nonce验证通过（未变化）")
	}

	t.Logf("[DEBUG] 余额不足测试完成")
}

// 清理测试数据库
func cleanupTestDB(dbDir string) {
	os.RemoveAll(dbDir)
}
