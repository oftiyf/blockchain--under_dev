package mpt

import (
	"encoding/json"
	"testing"
)

func TestMPT_SimplePutGet(t *testing.T) {
	db, err := NewDB("test_db_simple")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	mpt := NewMPT(db)

	key := []byte("hello")
	value := []byte("world")

	// 插入
	if err := mpt.Put(key, value); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 直接检查数据库内容
	allData, err := db.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all data from DB: %v", err)
	}

	t.Logf("Database contents:")
	for k, v := range allData {
		t.Logf("Key: %x", k)
		t.Logf("Value (hex): %x", v)

		// 尝试解码 JSON
		var nodeData map[string]interface{}
		if err := json.Unmarshal(v, &nodeData); err != nil {
			t.Logf("Failed to decode JSON: %v", err)
		} else {
			t.Logf("Value (decoded): %+v", nodeData)
		}
	}

	if len(allData) == 0 {
		t.Fatal("Database is empty after Put")
	}
}

func TestMPT_GetValue(t *testing.T) {
	db, err := NewDB("test_db_get")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	mpt := NewMPT(db)

	// 保证所有 value 长度一致（6 字节）
	pairs := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	for k, v := range pairs {
		err := mpt.Put([]byte(k), []byte(v))
		if err != nil {
			t.Fatalf("Put failed for key %s: %v", k, err)
		}
		t.Logf("Put key %s with value %s", k, v)
	}

	// 插入后，打印数据库内容
	allData, err := db.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all data from DB: %v", err)
	}
	t.Logf("Database contents after all puts:")
	for dbKey, dbValue := range allData {
		t.Logf("Key: %x", dbKey)
		t.Logf("Value (hex): %x", dbValue)
	}

	// 测试获取已存在的值
	for k := range pairs {
		value, err := mpt.Get([]byte(k))
		if err != nil {
			t.Fatalf("Get failed for key %s: %v", k, err)
		}
		t.Logf("Got value for key %s: %s", k, value)
	}

	// 测试获取不存在的值
	nonExistentKey := []byte("nonexistent")
	_, err = mpt.Get(nonExistentKey)
	if err == nil {
		t.Error("Get should fail for non-existent key")
	}
}
