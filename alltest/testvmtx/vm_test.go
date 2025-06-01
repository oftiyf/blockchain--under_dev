package testvmtx

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

// 使用固定的测试私钥
var testPrivateKeyHex = "0000000000000000000000000000000000000000000000000000000000000001"

func TestVM_ExecuteTransaction(t *testing.T) {
	// 创建临时数据库目录
	dbDir := "test_db_vm"
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("Failed to create DB directory: %v", err)
	}
	defer os.RemoveAll(dbDir) // 测试完成后清理

	// 生成测试用的私钥
	privateKeyBytes, err := hex.DecodeString(testPrivateKeyHex)
	if err != nil {
		t.Fatalf("Failed to decode private key: %v", err)
	}

	// 获取发送者地址
	publicKey, err := common.PrivateKeyToPublicKey(testPrivateKeyHex)
	if err != nil {
		t.Fatalf("Failed to get public key: %v", err)
	}
	hash := common.Hash{}.NewHash(publicKey)
	senderAddr := common.Address{}.NewAddress(hash[:20])
	fmt.Printf("Debug - Sender address: %v\n", senderAddr)

	// 创建共享的数据库和VM实例
	db, err := mpt.NewDB(filepath.Join(dbDir, "MPT_shared"))
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()
	stateDB := mpt.NewMPT(db)
	virtualMachine := vm.NewVM(stateDB)

	// 给发送者mint一次代币
	err = virtualMachine.Mint(senderAddr)
	if err != nil {
		t.Fatalf("Failed to mint tokens: %v", err)
	}

	// 测试用例1: 普通转账交易
	t.Run("Normal Transfer", func(t *testing.T) {
		// 创建接收者地址
		receiverBytes := make([]byte, 20)
		copy(receiverBytes, []byte("receiver"))
		receiver := common.Address{}.NewAddress(receiverBytes)

		// 创建普通转账交易
		transferTx := tx.NewTransaction(
			1,              // nonce
			receiver,       // to
			big.NewInt(50), // value
			1000,           // gasLimit
			big.NewInt(1),  // gasPrice
			[]byte{},       // data
			big.NewInt(1),  // chainID
		)

		// 签名交易
		err = transferTx.Sign(privateKeyBytes)
		if err != nil {
			t.Fatalf("Failed to sign transaction: %v", err)
		}

		// 执行交易
		aerr := virtualMachine.ExecuteTransaction(transferTx)
		if aerr != nil {
			t.Errorf("ExecuteTransaction failed: %v", aerr)
		}

		// 验证余额变化
		senderAccount, err := virtualMachine.GetAccount(senderAddr)
		if err != nil {
			t.Errorf("Failed to get sender account: %v", err)
		}
		receiverAccount, err := virtualMachine.GetAccount(receiver)
		if err != nil {
			t.Errorf("Failed to get receiver account: %v", err)
		}

		// 计算gas费用
		gasCost := uint64(transferTx.GasLimit) * transferTx.GasPrice.Uint64()
		expectedSenderBalance := uint64(1000000) - (uint64(50) + gasCost)
		expectedReceiverBalance := uint64(50)

		if senderAccount.Balance != expectedSenderBalance {
			t.Errorf("Sender balance incorrect. Got %v, want %v", senderAccount.Balance, expectedSenderBalance)
		}
		if receiverAccount.Balance != expectedReceiverBalance {
			t.Errorf("Receiver balance incorrect. Got %v, want %v", receiverAccount.Balance, expectedReceiverBalance)
		}
	})

	// 测试用例2: 合约创建
	t.Run("Contract Creation", func(t *testing.T) {
		// 给发送者mint一次代币
		err = virtualMachine.Mint(senderAddr)
		if err != nil {
			t.Fatalf("Failed to mint tokens: %v", err)
		}

		// 创建合约创建交易
		contractTx := tx.NewTransaction(
			0,                       // nonce
			common.Address{},        // to address is empty for contract creation
			big.NewInt(0),           // value
			1000,                    // 降低gasLimit
			big.NewInt(1),           // gasPrice
			[]byte("contract code"), // data
			big.NewInt(1),           // chainID
		)

		// 签名交易
		err = contractTx.Sign(privateKeyBytes)
		if err != nil {
			t.Fatalf("Failed to sign transaction: %v", err)
		}
		
		nonceBytes := []byte{byte(contractTx.Nonce)}
		// 执行交易
		err = virtualMachine.ExecuteTransaction(contractTx)
		if err != nil {
			t.Errorf("Contract creation failed: %v", err)
		}

		
		hash := common.Hash{}.NewHash(append(senderAddr.Bytes(), nonceBytes...))
		contractAddr := common.Address{}.NewAddress(hash[:20])
		contractAccount, err := virtualMachine.GetAccount(contractAddr)
		if err != nil {
			t.Errorf("Failed to get contract account: %v", err)
		}
		if len(contractAccount.Code) == 0 {
			t.Error("Contract code not found")
		}

		// 验证发送者余额变化
		senderAccount, err := virtualMachine.GetAccount(senderAddr)
		if err != nil {
			t.Errorf("Failed to get sender account: %v", err)
		}

		// 计算gas费用
		gasCost := uint64(contractTx.GasLimit) * contractTx.GasPrice.Uint64()
		// 2000000 (两次mint) - 50 (第一次转账) - 1000 (第一次gas) - gasCost (本次合约创建gas)
		expectedSenderBalance := uint64(2000000) - 50 - 1000 - gasCost

		if senderAccount.Balance != expectedSenderBalance {
			t.Errorf("Sender balance incorrect after contract creation. Got %v, want %v", senderAccount.Balance, expectedSenderBalance)
		}
	})

	// 测试用例3: 余额不足
	t.Run("Insufficient Balance", func(t *testing.T) {
		// 创建接收者地址
		receiverBytes := make([]byte, 20)
		copy(receiverBytes, []byte("rich_receiver"))
		receiver := common.Address{}.NewAddress(receiverBytes)

		// 创建转账交易
		transferTx := tx.NewTransaction(
			0, // nonce
			receiver,
			big.NewInt(1000000000000000000), // value
			21000,                           // gasLimit
			big.NewInt(1),                   // gasPrice
			nil,                             // data
			big.NewInt(1),                   // chainID
		)

		// 签名交易
		err = transferTx.Sign(privateKeyBytes)
		if err != nil {
			t.Fatalf("Failed to sign transaction: %v", err)
		}

		// 执行交易
		err = virtualMachine.ExecuteTransaction(transferTx)
		if err == nil {
			t.Error("Expected error for insufficient balance, got nil")
		}
	})

	// 测试用例4: 无效交易
	t.Run("Invalid Transaction", func(t *testing.T) {
		// 创建无效交易
		invalidTx := tx.NewTransaction(
			0,                // nonce
			common.Address{}, // to address
			big.NewInt(0),    // value
			0,                // invalid gasLimit
			big.NewInt(0),    // invalid gasPrice
			nil,              // data
			big.NewInt(1),    // chainID
		)

		// 签名交易
		err = invalidTx.Sign(privateKeyBytes)
		if err != nil {
			t.Fatalf("Failed to sign transaction: %v", err)
		}

		// 执行交易
		err = virtualMachine.ExecuteTransaction(invalidTx)
		if err == nil {
			t.Error("Expected error for invalid transaction, got nil")
		}
	})

	// 测试用例5: Mint功能测试
	t.Run("Mint Functionality", func(t *testing.T) {
		// 创建测试地址
		testAddrBytes := make([]byte, 20)
		copy(testAddrBytes, []byte("test_mint_addr"))
		testAddr := common.Address{}.NewAddress(testAddrBytes)

		// 测试正常mint
		for i := 0; i < 5; i++ {
			err := virtualMachine.Mint(testAddr)
			if err != nil {
				t.Errorf("Mint failed on attempt %d: %v", i+1, err)
			}

			// 验证余额
			account, err := virtualMachine.GetAccount(testAddr)
			if err != nil {
				t.Errorf("Failed to get account: %v", err)
			}
			expectedBalance := uint64(1000000 * (i + 1))
			if account.Balance != expectedBalance {
				t.Errorf("Balance incorrect after mint %d. Got %v, want %v", i+1, account.Balance, expectedBalance)
			}

			// 验证mint计数
			if virtualMachine.GetMintCount(testAddr) != i+1 {
				t.Errorf("Mint count incorrect. Got %d, want %d", virtualMachine.GetMintCount(testAddr), i+1)
			}
		}

		// 测试超过mint限制
		err = virtualMachine.Mint(testAddr)
		if err == nil {
			t.Error("Expected error for exceeding mint limit, got nil")
		}
		if err != nil && err.Error() != "mint limit exceeded" {
			t.Errorf("Unexpected error message. Got %v, want 'mint limit exceeded'", err)
		}

		// 验证最终余额
		account, err := virtualMachine.GetAccount(testAddr)
		if err != nil {
			t.Errorf("Failed to get final account: %v", err)
		}
		expectedBalance := uint64(5000000) // 5次mint，每次1000000
		if account.Balance != expectedBalance {
			t.Errorf("Final balance incorrect. Got %v, want %v", account.Balance, expectedBalance)
		}
	})
}
