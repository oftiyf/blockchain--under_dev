package main

import (
	"blockchain/common"
	"blockchain/mpt"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) (*mpt.DB, func()) {
	// 创建数据库目录
	dbDir := "test_db_account"
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("创建数据库目录失败: %v", err)
	}

	// 创建数据库连接
	db, err := mpt.NewDB(filepath.Join(dbDir, "MPT"))
	if err != nil {
		t.Fatalf("创建数据库连接失败: %v", err)
	}

	// 返回清理函数
	cleanup := func() {
		db.Close()
		os.RemoveAll(dbDir)
	}

	return db, cleanup
}

func TestMPTBasicOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	trie := mpt.NewMPT(db)

	// 测试简单的键值对
	t.Run("Simple Put and Get", func(t *testing.T) {
		key := []byte("hello")
		value := []byte("world")

		// 插入
		if err := trie.Put(key, value); err != nil {
			t.Fatalf("Put failed: %v", err)
		}

		// 获取并验证
		retrievedValue, err := trie.Get(key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if string(retrievedValue) != string(value) {
			t.Errorf("Expected value %s, got %s", value, retrievedValue)
		}
	})

	// 测试多个键值对
	t.Run("Multiple Key-Value Pairs", func(t *testing.T) {
		pairs := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}

		// 插入所有键值对
		for k, v := range pairs {
			if err := trie.Put([]byte(k), []byte(v)); err != nil {
				t.Errorf("Put failed for key %s: %v", k, err)
			}
		}

		// 验证所有键值对
		for k, v := range pairs {
			retrievedValue, err := trie.Get([]byte(k))
			if err != nil {
				t.Errorf("Get failed for key %s: %v", k, err)
				continue
			}
			if string(retrievedValue) != v {
				t.Errorf("For key %s, expected value %s, got %s", k, v, retrievedValue)
			}
		}
	})

	// 测试更新操作
	t.Run("Update Value", func(t *testing.T) {
		key := []byte("test_key")
		initialValue := []byte("initial_value")
		updatedValue := []byte("updated_value")

		// 插入初始值
		if err := trie.Put(key, initialValue); err != nil {
			t.Fatalf("Initial Put failed: %v", err)
		}

		// 验证初始值
		value, err := trie.Get(key)
		if err != nil {
			t.Fatalf("Get initial value failed: %v", err)
		}
		if string(value) != string(initialValue) {
			t.Errorf("Expected initial value %s, got %s", initialValue, value)
		}

		// 更新值
		if err := trie.Put(key, updatedValue); err != nil {
			t.Fatalf("Update Put failed: %v", err)
		}

		// 验证更新后的值
		value, err = trie.Get(key)
		if err != nil {
			t.Fatalf("Get updated value failed: %v", err)
		}
		if string(value) != string(updatedValue) {
			t.Errorf("Expected updated value %s, got %s", updatedValue, value)
		}
	})
}

func TestMPTAccountOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	trie := mpt.NewMPT(db)

	// 创建测试账户
	accounts := []struct {
		key   []byte
		value *common.Account
	}{
		{
			key:   []byte("0x1234567890abcdef1234567890abcdef12345678"),
			value: common.NewEOA([]byte("0x1234567890abcdef1234567890abcdef12345678")),
		},
		{
			key: []byte("0xabcdef1234567890abcdef1234567890abcdef12"),
			value: common.NewContract(
				[]byte("codehash2"),
				[]byte("contract2"),
			),
		},
	}

	// 设置账户数据
	accounts[0].value.Nonce = 1
	accounts[0].value.Balance = 1000
	accounts[0].value.Storage = map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	accounts[1].value.Nonce = 2
	accounts[1].value.Balance = 2000
	accounts[1].value.Storage = map[string]string{
		"key3": "value3",
		"key4": "value4",
	}

	t.Run("Insert Accounts", func(t *testing.T) {
		for _, acc := range accounts {
			valueBytes := acc.value.Serialize()
			if err := trie.Put(acc.key, valueBytes); err != nil {
				t.Errorf("插入账户失败 %x: %v", acc.key, err)
			}
		}
	})

	t.Run("Query Accounts", func(t *testing.T) {
		for _, acc := range accounts {
			valueBytes, err := trie.Get(acc.key)
			if err != nil {
				t.Errorf("获取账户失败 %x: %v", acc.key, err)
				continue
			}

			var retrievedAcc common.Account
			if err := json.Unmarshal(valueBytes, &retrievedAcc); err != nil {
				t.Errorf("反序列化账户失败 %x: %v", acc.key, err)
				continue
			}

			if retrievedAcc.Nonce != acc.value.Nonce {
				t.Errorf("账户 %x Nonce 不匹配: 期望 %d, 得到 %d", acc.key, acc.value.Nonce, retrievedAcc.Nonce)
			}
			if retrievedAcc.Balance != acc.value.Balance {
				t.Errorf("账户 %x Balance 不匹配: 期望 %d, 得到 %d", acc.key, acc.value.Balance, retrievedAcc.Balance)
			}
		}
	})

	t.Run("Update Account", func(t *testing.T) {
		updateKey := accounts[0].key
		updatedAccount := common.NewEOA([]byte("0x1234567890abcdef1234567890abcdef12345678"))
		updatedAccount.Nonce = 10
		updatedAccount.Balance = 5000
		updatedAccount.Storage = map[string]string{
			"updated_key": "updated_value",
		}

		valueBytes := updatedAccount.Serialize()
		if err := trie.Put(updateKey, valueBytes); err != nil {
			t.Errorf("更新账户失败 %x: %v", updateKey, err)
		}

		// 验证更新
		valueBytes, err := trie.Get(updateKey)
		if err != nil {
			t.Errorf("获取更新后的账户失败 %x: %v", updateKey, err)
			return
		}

		var retrievedAcc common.Account
		if err := json.Unmarshal(valueBytes, &retrievedAcc); err != nil {
			t.Errorf("反序列化更新后的账户失败 %x: %v", updateKey, err)
			return
		}

		if retrievedAcc.Nonce != 10 {
			t.Errorf("更新后的 Nonce 不匹配: 期望 10, 得到 %d", retrievedAcc.Nonce)
		}
		if retrievedAcc.Balance != 5000 {
			t.Errorf("更新后的 Balance 不匹配: 期望 5000, 得到 %d", retrievedAcc.Balance)
		}
	})
}

func TestMPTTreeStructure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	trie := mpt.NewMPT(db)

	// 插入一些测试数据
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for k, v := range testData {
		if err := trie.Put([]byte(k), []byte(v)); err != nil {
			t.Errorf("插入数据失败 %s: %v", k, err)
		}
	}

	t.Run("Verify Tree Structure", func(t *testing.T) {
		allData, err := db.GetAll()
		if err != nil {
			t.Errorf("获取所有数据失败: %v", err)
			return
		}

		if len(allData) == 0 {
			t.Error("数据库为空")
			return
		}

		for k, v := range allData {
			var nodeType struct {
				NodeType mpt.NodeType `json:"nodeType"`
			}
			if err := json.Unmarshal(v, &nodeType); err != nil {
				t.Errorf("解码节点类型失败 (key: %x): %v", k, err)
				continue
			}

			// 验证节点类型
			switch nodeType.NodeType {
			case mpt.LeafNodeType:
				var node mpt.LeafNode
				if err := json.Unmarshal(v, &node); err != nil {
					t.Errorf("解码叶子节点失败: %v", err)
				}
			case mpt.ExtensionNodeType:
				var node mpt.ExtensionNode
				if err := json.Unmarshal(v, &node); err != nil {
					t.Errorf("解码扩展节点失败: %v", err)
				}
			case mpt.BranchNodeType:
				var node mpt.FullNode
				if err := json.Unmarshal(v, &node); err != nil {
					t.Errorf("解码分支节点失败: %v", err)
				}
			default:
				t.Errorf("未知的节点类型: %d", nodeType.NodeType)
			}
		}
	})
}
