package mpt

import (
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// DB 封装 levelDB 操作的结构体123
type DB struct {
	db *leveldb.DB
}

// NewDB 创建一个新的数据库连接
func NewDB(path string) (*DB, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, fmt.Errorf("无法打开数据库: %v", err)
	}
	return &DB{db: db}, nil
}

// Put 存储键值对
func (d *DB) Put(key, value []byte) error {
	return d.db.Put(key, value, nil)
}

// Get 获取指定键的值
func (d *DB) Get(key []byte) ([]byte, error) {
	return d.db.Get(key, nil)
}

// Delete 删除指定键的值
func (d *DB) Delete(key []byte) error {
	return d.db.Delete(key, nil)
}

// Has 检查键是否存在
func (d *DB) Has(key []byte) (bool, error) {
	return d.db.Has(key, nil)
}

// Close 关闭数据库连接
func (d *DB) Close() error {
	return d.db.Close()
}

// GetAll 获取所有键值对
func (d *DB) GetAll() (map[string][]byte, error) {
	iter := d.db.NewIterator(nil, nil)
	defer iter.Release()

	result := make(map[string][]byte)
	for iter.Next() {
		key := string(iter.Key())
		value := make([]byte, len(iter.Value()))
		copy(value, iter.Value())
		result[key] = value
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetRange 获取指定范围的键值对
func (d *DB) GetRange(start, limit []byte) (map[string][]byte, error) {
	iter := d.db.NewIterator(&util.Range{Start: start, Limit: limit}, nil)
	defer iter.Release()

	result := make(map[string][]byte)
	for iter.Next() {
		key := string(iter.Key())
		value := make([]byte, len(iter.Value()))
		copy(value, iter.Value())
		result[key] = value
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}
	return result, nil
}

// BatchPut 批量存储键值对
func (d *DB) BatchPut(kvs map[string][]byte) error {
	batch := new(leveldb.Batch)
	for k, v := range kvs {
		batch.Put([]byte(k), v)
	}
	return d.db.Write(batch, nil)
}

// BatchDelete 批量删除键
func (d *DB) BatchDelete(keys []string) error {
	batch := new(leveldb.Batch)
	for _, k := range keys {
		batch.Delete([]byte(k))
	}
	return d.db.Write(batch, nil)
}
