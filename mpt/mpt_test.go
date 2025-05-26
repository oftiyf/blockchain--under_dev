package mpt

import (
	"encoding/json"
	"os"
	"testing"
)

func cleanupDB(path string) {
	os.RemoveAll(path)
}

func TestMPT_SimplePutGet(t *testing.T) {
	dbPath := "test_db_simple"
	cleanupDB(dbPath)
	defer cleanupDB(dbPath)

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

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
	dbPath := "test_db_get"
	cleanupDB(dbPath)
	defer cleanupDB(dbPath)

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

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

func TestMPT_UpdateValue(t *testing.T) {
	dbPath := "test_db_update"
	cleanupDB(dbPath)
	defer cleanupDB(dbPath)

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	mpt := NewMPT(db)

	key := []byte("test_key")
	initialValue := []byte("initial_value")
	updatedValue := []byte("updated_value")

	// 插入初始值
	if err := mpt.Put(key, initialValue); err != nil {
		t.Fatalf("Initial Put failed: %v", err)
	}

	// 验证初始值
	value, err := mpt.Get(key)
	if err != nil {
		t.Fatalf("Get initial value failed: %v", err)
	}
	if string(value) != string(initialValue) {
		t.Errorf("Expected initial value %s, got %s", initialValue, value)
	}

	// 更新值
	if err := mpt.Put(key, updatedValue); err != nil {
		t.Fatalf("Update Put failed: %v", err)
	}

	// 验证更新后的值
	value, err = mpt.Get(key)
	if err != nil {
		t.Fatalf("Get updated value failed: %v", err)
	}
	if string(value) != string(updatedValue) {
		t.Errorf("Expected updated value %s, got %s", updatedValue, value)
	}
}

func TestMPT_DeleteValue(t *testing.T) {
	dbPath := "test_db_delete"
	cleanupDB(dbPath)
	defer cleanupDB(dbPath)

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	mpt := NewMPT(db)

	key := []byte("test_key")
	value := []byte("test_value")

	// 插入值
	if err := mpt.Put(key, value); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 验证值存在
	_, err = mpt.Get(key)
	if err != nil {
		t.Fatalf("Get before delete failed: %v", err)
	}

	// 删除值
	if err := mpt.Delete(key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证值已被删除
	_, err = mpt.Get(key)
	if err == nil {
		t.Error("Expected error when getting deleted key, got nil")
	}
}

func TestMPT_EmptyKey(t *testing.T) {
	dbPath := "test_db_empty_key"
	cleanupDB(dbPath)
	defer cleanupDB(dbPath)

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	mpt := NewMPT(db)

	emptyKey := []byte("")
	value := []byte("empty_key_value")

	// 测试空键
	if err := mpt.Put(emptyKey, value); err != nil {
		t.Fatalf("Put with empty key failed: %v", err)
	}

	retrievedValue, err := mpt.Get(emptyKey)
	if err != nil {
		t.Fatalf("Get with empty key failed: %v", err)
	}
	if string(retrievedValue) != string(value) {
		t.Errorf("Expected value %s for empty key, got %s", value, retrievedValue)
	}
}

func TestMPT_LongKey(t *testing.T) {
	dbPath := "test_db_long_key"
	cleanupDB(dbPath)
	defer cleanupDB(dbPath)

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	mpt := NewMPT(db)

	// 创建一个很长的键
	longKey := make([]byte, 1000)
	for i := range longKey {
		longKey[i] = byte(i % 256)
	}
	value := []byte("long_key_value")

	// 测试长键
	if err := mpt.Put(longKey, value); err != nil {
		t.Fatalf("Put with long key failed: %v", err)
	}

	retrievedValue, err := mpt.Get(longKey)
	if err != nil {
		t.Fatalf("Get with long key failed: %v", err)
	}
	if string(retrievedValue) != string(value) {
		t.Errorf("Expected value %s for long key, got %s", value, retrievedValue)
	}
}
