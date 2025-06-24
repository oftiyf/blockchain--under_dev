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

// MockTransactionExecutor 模拟交易执行器
type MockTransactionExecutor struct {
	executedTxs []*tx.Transaction
	shouldFail  bool
}

func (m *MockTransactionExecutor) ExecuteTransaction(tx *tx.Transaction) error {
	if m.shouldFail {
		return nil // 模拟失败
	}
	m.executedTxs = append(m.executedTxs, tx)
	return nil
}

// RealTransactionExecutor 真实的交易执行器，使用VM
type RealTransactionExecutor struct {
	vm *vm.VM
}

func NewRealTransactionExecutor(vm *vm.VM) *RealTransactionExecutor {
	return &RealTransactionExecutor{vm: vm}
}

func (r *RealTransactionExecutor) ExecuteTransaction(tx *tx.Transaction) error {
	return r.vm.ExecuteTransaction(tx)
}

// 创建测试用的私钥和地址
var (
	testPrivateKey = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	testAddress    = common.Address{}.NewAddress([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20})
)

// 创建测试交易
func createTestTransaction(nonce uint64, gasPrice *big.Int, value *big.Int, to *common.Address) *tx.Transaction {
	chainID := big.NewInt(1)
	tx := tx.NewTransaction(nonce, *to, value, 21000, gasPrice, []byte{}, chainID)

	// 签名交易
	err := tx.Sign(testPrivateKey)
	if err != nil {
		panic(err)
	}

	return tx
}

// 测试完整的交易流程：从私钥生成地址 -> mint代币 -> 转账
func TestCompleteTransactionFlow(t *testing.T) {
	// 创建临时数据库目录
	dbDir := "test_db_complete_flow"
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("Failed to create DB directory: %v", err)
	}
	defer os.RemoveAll(dbDir) // 测试完成后清理

	// 1. 从私钥生成公钥和地址
	privateKeyHex := "0000000000000000000000000000000000000000000000000000000000000001"
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		t.Fatalf("Failed to decode private key: %v", err)
	}

	// 获取发送者地址
	publicKey, err := common.PrivateKeyToPublicKey(privateKeyHex)
	if err != nil {
		t.Fatalf("Failed to get public key: %v", err)
	}
	hash := common.Hash{}.NewHash(publicKey)
	senderAddr := common.Address{}.NewAddress(hash[:20])
	fmt.Printf("Debug - Generated sender address: %v\n", senderAddr)

	// 2. 创建多个接收者地址
	receiverAddrs := make([]common.Address, 5)
	for i := 0; i < 5; i++ {
		receiverBytes := make([]byte, 20)
		copy(receiverBytes, []byte(fmt.Sprintf("receiver_%d_address", i)))
		receiverAddrs[i] = common.Address{}.NewAddress(receiverBytes)
		fmt.Printf("Debug - Generated receiver %d address: %v\n", i, receiverAddrs[i])
	}

	// 3. 创建共享的数据库、VM和交易池
	db, err := mpt.NewDB(filepath.Join(dbDir, "MPT_shared"))
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	stateDB := mpt.NewMPT(db)
	virtualMachine := vm.NewVM(stateDB)
	txPool := tx.NewTxPool(stateDB)

	// 4. 给发送者mint代币（这会创建账户）100 0000个代币
	fmt.Printf("Debug - Minting tokens to sender...\n")
	err = virtualMachine.Mint(senderAddr)
	if err != nil {
		t.Fatalf("Failed to mint tokens: %v", err)
	}

	// 验证mint后的余额
	senderAccount, err := virtualMachine.GetAccount(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get sender account: %v", err)
	}
	fmt.Printf("Debug - Sender balance after mint: %v\n", senderAccount.Balance)
	if senderAccount.Balance != 1000000 {
		t.Errorf("Expected sender balance 1000000, got %v", senderAccount.Balance)
	}

	// 5. 验证账户nonce（现在应该能正确获取）
	nonce, err := txPool.GetAccountNonce(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get account nonce: %v", err)
	}
	fmt.Printf("Debug - Sender nonce: %d\n", nonce)
	if nonce != 0 {
		t.Errorf("Expected nonce 0, got %d", nonce)
	}

	// 6. 创建多笔转账交易
	fmt.Printf("Debug - Creating multiple transfer transactions...\n")
	transactions := make([]*tx.Transaction, 5)
	transferValues := []*big.Int{
		big.NewInt(100000), // 第1笔：10万代币  89 12.1
		big.NewInt(150000), // 第2笔：15万代币  78 12.1
		big.NewInt(200000), // 第3笔：20万代币  62 17.1
		big.NewInt(250000), // 第4笔：25万代币	51 12.1
		big.NewInt(200000), // 第5笔：20万代币  35
	}
	gasPrice := big.NewInt(1) // 1 wei
	gasLimit := uint64(10000) // 标准转账gas

	for i := 0; i < 5; i++ {
		// 创建nonce为i+1的转账交易
		transferTx := tx.NewTransaction(
			uint64(i+1),       // nonce从1开始递增
			receiverAddrs[i],  // to
			transferValues[i], // value
			gasLimit,          // gasLimit
			gasPrice,          // gasPrice
			[]byte{},          // data
			big.NewInt(1),     // chainID
		)

		// 签名交易
		err = transferTx.Sign(privateKeyBytes)
		if err != nil {
			t.Fatalf("Failed to sign transfer transaction %d: %v", i+1, err)
		}

		transactions[i] = transferTx
		fmt.Printf("Debug - Created transaction %d with nonce %d, value %v\n", i+1, i+1, transferValues[i])
	}

	// 7. 将所有交易添加到交易池
	fmt.Printf("Debug - Adding all transactions to pool...\n")
	for i, transferTx := range transactions {
		err = txPool.AddTx(transferTx)
		if err != nil {
			t.Fatalf("Failed to add transaction %d to pool: %v", i+1, err)
		}
		fmt.Printf("Debug - Added transaction %d to pool\n", i+1)
	}

	// 验证交易是否被添加到queue pool（不是pending pool）
	txBoxes := txPool.GetQueueTxBoxes()
	if len(txBoxes) == 0 {
		t.Fatal("No transactions added to queue pool")
	}
	fmt.Printf("Debug - All transactions added to pool, queue boxes: %d\n", len(txBoxes))

	// 8. 创建真实的交易执行器
	executor := NewRealTransactionExecutor(virtualMachine)

	// 9. 通过交易池执行所有交易
	fmt.Printf("Debug - Executing all transactions through pool...\n")
	err = txPool.Execute(executor, 5) // 执行5笔交易
	if err != nil {
		t.Fatalf("Failed to execute transactions: %v", err)
	}

	// 10. 验证执行结果
	fmt.Printf("Debug - Verifying execution results...\n")

	// 验证发送者余额变化
	updatedSenderAccount, err := virtualMachine.GetAccount(senderAddr)
	if err != nil {
		t.Fatalf("Failed to get updated sender account: %v", err)
	}

	// 计算总转账金额和gas费用
	totalTransferValue := uint64(0)
	for _, value := range transferValues {
		totalTransferValue += value.Uint64()
	}
	totalGasCost := uint64(5) * gasPrice.Uint64() // 5笔交易的gas费用
	expectedSenderBalance := uint64(1000000) - totalTransferValue - totalGasCost

	fmt.Printf("Debug - Total transfer value: %v\n", totalTransferValue)
	fmt.Printf("Debug - Total gas cost: %v\n", totalGasCost)
	fmt.Printf("Debug - Expected sender balance: %v, Actual: %v\n", expectedSenderBalance, updatedSenderAccount.Balance)

	if updatedSenderAccount.Balance != expectedSenderBalance {
		t.Errorf("Sender balance incorrect. Expected %v, got %v", expectedSenderBalance, updatedSenderAccount.Balance)
	}

	// 验证所有接收者余额
	for i, receiverAddr := range receiverAddrs {
		receiverAccount, err := virtualMachine.GetAccount(receiverAddr)
		if err != nil {
			t.Fatalf("Failed to get receiver %d account: %v", i+1, err)
		}
		fmt.Printf("Debug - Receiver %d balance: %v (expected: %v)\n", i+1, receiverAccount.Balance, transferValues[i].Uint64())

		if receiverAccount.Balance != transferValues[i].Uint64() {
			t.Errorf("Receiver %d balance incorrect. Expected %v, got %v", i+1, transferValues[i].Uint64(), receiverAccount.Balance)
		}
	}

	// 验证发送者nonce增加
	expectedNonce := uint64(5) // 执行了5笔交易，nonce应该是5
	if updatedSenderAccount.Nonce != expectedNonce {
		t.Errorf("Sender nonce should be %v, got %v", expectedNonce, updatedSenderAccount.Nonce)
	}

	// 11. 验证交易池状态 - 执行完成后，queue和pending都应该为空
	remainingQueueBoxes := txPool.GetQueueTxBoxes()
	remainingPendingBoxes := txPool.GetPendingTxBoxes()
	if len(remainingQueueBoxes) != 0 {
		t.Errorf("Expected no remaining transactions in queue, got %d", len(remainingQueueBoxes))
	}
	if len(remainingPendingBoxes) != 0 {
		t.Errorf("Expected no remaining transactions in pending, got %d", len(remainingPendingBoxes))
	}

	fmt.Printf("Debug - Complete transaction flow test with multiple transactions passed!\n")
}
