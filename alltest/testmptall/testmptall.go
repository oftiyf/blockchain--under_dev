package main

import (
	"blockchain/common"
	"blockchain/mpt"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	// 创建数据库目录
	dbDir := "test_db_account"
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("创建数据库目录失败: %v", err)
	}
	defer os.RemoveAll(dbDir) // 测试完成后清理

	// 创建数据库连接
	db, err := mpt.NewDB(filepath.Join(dbDir, "MPT"))
	if err != nil {
		log.Fatalf("创建数据库连接失败: %v", err)
	}
	defer db.Close()

	// 创建 MPT
	trie := mpt.NewMPT(db)

	// 创建一些测试账户
	accounts := []struct {
		key   []byte
		value *common.Account
	}{
		{
			key:   []byte("0x1234567890abcdef1234567890abcdef12345678"), // 模拟公钥哈希
			value: common.NewEOA(),                                      // 创建外部账户
		},
		{
			key: []byte("0xabcdef1234567890abcdef1234567890abcdef12"),
			value: common.NewContract(
				[]byte("codehash2"),
				[]byte("contract2"),
			),
		},
		{
			key: []byte("0x7890abcdef1234567890abcdef1234567890abcd"),
			value: common.NewContract(
				[]byte("codehash3"),
				[]byte("contract3"),
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

	accounts[2].value.Nonce = 3
	accounts[2].value.Balance = 3000
	accounts[2].value.Storage = map[string]string{
		"key5": "value5",
		"key6": "value6",
	}

	fmt.Println("=== 开始测试 MPT 账户存储 ===")

	// 1. 插入测试
	fmt.Println("\n1. 插入账户数据:")
	for _, acc := range accounts {
		// 序列化账户数据
		valueBytes := acc.value.Serialize()
		if err := trie.Put(acc.key, valueBytes); err != nil {
			log.Printf("插入账户失败 %x: %v", acc.key, err)
		} else {
			fmt.Printf("成功插入账户: %x\n", acc.key)
		}
	}

	// 2. 查询测试
	fmt.Println("\n2. 查询账户数据:")
	for _, acc := range accounts {
		valueBytes, err := trie.Get(acc.key)
		if err != nil {
			log.Printf("获取账户失败 %x: %v", acc.key, err)
			continue
		}

		// 反序列化账户数据
		var retrievedAcc common.Account
		if err := json.Unmarshal(valueBytes, &retrievedAcc); err != nil {
			log.Printf("反序列化账户失败 %x: %v", acc.key, err)
			continue
		}

		fmt.Printf("账户 %x:\n", acc.key)
		fmt.Printf("  Nonce: %d\n", retrievedAcc.Nonce)
		fmt.Printf("  Balance: %d\n", retrievedAcc.Balance)
		fmt.Printf("  CodeHash: %x\n", retrievedAcc.CodeHash)
		fmt.Printf("  Code: %s\n", retrievedAcc.Code)
		fmt.Printf("  Storage: %v\n", retrievedAcc.Storage)
		fmt.Printf("  IsEoa: %v\n", retrievedAcc.IsEoa)
	}

	// 3. 更新测试
	fmt.Println("\n3. 更新账户数据:")
	updateKey := accounts[0].key
	updatedAccount := common.NewEOA()
	updatedAccount.Nonce = 10
	updatedAccount.Balance = 5000
	updatedAccount.Storage = map[string]string{
		"updated_key": "updated_value",
	}

	valueBytes := updatedAccount.Serialize()
	if err := trie.Put(updateKey, valueBytes); err != nil {
		log.Printf("更新账户失败 %x: %v", updateKey, err)
	} else {
		fmt.Printf("成功更新账户: %x\n", updateKey)
	}

	// 验证更新
	valueBytes, err = trie.Get(updateKey)
	if err != nil {
		log.Printf("获取更新后的账户失败 %x: %v", updateKey, err)
	} else {
		var retrievedAcc common.Account
		if err := json.Unmarshal(valueBytes, &retrievedAcc); err != nil {
			log.Printf("反序列化更新后的账户失败 %x: %v", updateKey, err)
		} else {
			fmt.Printf("更新后的账户 %x:\n", updateKey)
			fmt.Printf("  Nonce: %d\n", retrievedAcc.Nonce)
			fmt.Printf("  Balance: %d\n", retrievedAcc.Balance)
			fmt.Printf("  CodeHash: %x\n", retrievedAcc.CodeHash)
			fmt.Printf("  Code: %s\n", retrievedAcc.Code)
			fmt.Printf("  Storage: %v\n", retrievedAcc.Storage)
			fmt.Printf("  IsEoa: %v\n", retrievedAcc.IsEoa)
		}
	}

	// 4. 删除测试
	fmt.Println("\n4. 删除账户数据:")
	deleteKey := accounts[1].key
	if err := trie.Delete(deleteKey); err != nil {
		log.Printf("删除账户失败 %x: %v", deleteKey, err)
	} else {
		fmt.Printf("成功删除账户: %x\n", deleteKey)
	}

	// 验证删除
	_, err = trie.Get(deleteKey)
	if err == nil {
		log.Printf("删除后的账户仍然存在 %x", deleteKey)
	} else {
		fmt.Printf("账户已成功删除: %x\n", deleteKey)
	}

	// 5. 打印树结构
	fmt.Println("\n5. 打印树结构:")
	allData, err := db.GetAll()
	if err != nil {
		log.Printf("获取所有数据失败: %v", err)
	} else {
		fmt.Println("数据库内容:")
		for k, v := range allData {
			fmt.Printf("Key: %x\n", k)
			fmt.Printf("Value (hex): %x\n", v)

			// 尝试解码节点
			var nodeType struct {
				NodeType mpt.NodeType `json:"nodeType"`
			}
			if err := json.Unmarshal(v, &nodeType); err != nil {
				fmt.Printf("Failed to decode node type: %v\n", err)
				continue
			}

			switch nodeType.NodeType {
			case mpt.LeafNodeType:
				var node mpt.LeafNode
				if err := json.Unmarshal(v, &node); err != nil {
					fmt.Printf("Failed to decode leaf node: %v\n", err)
				} else {
					fmt.Printf("Node: %s\n", node.String())
				}
			case mpt.ExtensionNodeType:
				var node mpt.ExtensionNode
				if err := json.Unmarshal(v, &node); err != nil {
					fmt.Printf("Failed to decode extension node: %v\n", err)
				} else {
					fmt.Printf("Node: %s\n", node.String())
				}
			case mpt.BranchNodeType:
				var node mpt.FullNode
				if err := json.Unmarshal(v, &node); err != nil {
					fmt.Printf("Failed to decode branch node: %v\n", err)
				} else {
					fmt.Printf("Node: %s\n", node.String())
				}
			default:
				fmt.Printf("Unknown node type: %d\n", nodeType.NodeType)
			}
		}
	}
}
