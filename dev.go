package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"blockchain/mpt"
)
type Account struct {
	Nonce uint64	
	Balance int
	CodeHash []byte
	Code []byte
	Storage map[string]string
}


func main() {
	// 创建数据库目录
	dbDir := "DB"
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("创建数据库目录失败: %v", err)
	}

	// 创建数据库连接
	db, err := mpt.NewDB(filepath.Join(dbDir, "MPT"))
	if err != nil {
		log.Fatalf("创建数据库连接失败: %v", err)
	}
	defer db.Close()

	// 创建 MPT
	trie := mpt.NewMPT(db)

	// 测试数据
	testData := map[string]string{
		"test1": "value1",
		"test2": "value2",
		"test3": "value3",
	}

	fmt.Println("=== 开始测试 MPT 基础功能 ===")

	// 1. 插入测试
	fmt.Println("\n1. 插入测试数据:")
	for k, v := range testData {
		if err := trie.Put([]byte(k), []byte(v)); err != nil {
			log.Printf("插入数据失败 %s: %v", k, err)
		} else {
			fmt.Printf("成功插入: %s -> %s\n", k, v)
		}
	}

	// 2. 查询测试
	fmt.Println("\n2. 查询测试数据:")
	for k := range testData {
		value, err := trie.Get([]byte(k))
		if err != nil {
			log.Printf("获取数据失败 %s: %v", k, err)
		} else {
			fmt.Printf("成功获取: %s -> %s\n", k, string(value))
		}
	}

	// 3. 检查键是否存在
	fmt.Println("\n3. 检查键是否存在:")
	for k := range testData {
		exists := trie.Has([]byte(k))
		fmt.Printf("键 %s 存在: %v\n", k, exists)
	}

	// 4. 获取所有数据
	fmt.Println("\n4. 获取所有数据:")
	allData := trie.GetAll()
	for k, v := range allData {
		fmt.Printf("%s -> %s\n", k, string(v))
	}

	// 5. 打印树结构
	fmt.Println("\n5. 打印树结构:")
	trie.PrintTree()

	// 6. 删除测试
	fmt.Println("\n6. 删除测试:")
	keyToDelete := "test2"
	if err := trie.Delete([]byte(keyToDelete)); err != nil {
		log.Printf("删除数据失败 %s: %v", keyToDelete, err)
	} else {
		fmt.Printf("成功删除: %s\n", keyToDelete)
	}

	// 7. 验证删除结果
	fmt.Println("\n7. 验证删除结果:")
	value, err := trie.Get([]byte(keyToDelete))
	if err != nil {
		fmt.Printf("键 %s 已不存在\n", keyToDelete)
	} else {
		fmt.Printf("键 %s 仍然存在，值为: %s\n", keyToDelete, string(value))
	}

	// 8. 打印最终树结构
	fmt.Println("\n8. 最终树结构:")
	trie.PrintTree()

	fmt.Println("\n=== MPT 基础功能测试完成 ===")
}
