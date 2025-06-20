package tx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// txJournal 用于管理本地交易的磁盘日志
// 支持交易持久化和重启后恢复
type txJournal struct {
	path    string        // 日志文件路径
	writer  *os.File      // 文件写入器
	decoder *json.Decoder // JSON解码器
}

// newTxJournal 创建新的txJournal
func newTxJournal(path string) *txJournal {
	return &txJournal{
		path: path,
	}
}

// load 从磁盘加载交易日志
func (j *txJournal) load(add func(*Transaction) error) error {
	// 检查文件是否存在
	if _, err := os.Stat(j.path); os.IsNotExist(err) {
		return nil // 文件不存在，不是错误
	}

	// 打开文件
	file, err := os.Open(j.path)
	if err != nil {
		return fmt.Errorf("failed to open journal file: %v", err)
	}
	defer file.Close()

	// 创建JSON解码器
	j.decoder = json.NewDecoder(file)

	// 读取文件头
	var header struct {
		Version string `json:"version"`
		Date    string `json:"date"`
	}
	if err := j.decoder.Decode(&header); err != nil {
		return fmt.Errorf("failed to decode journal header: %v", err)
	}

	// 逐行读取交易
	for j.decoder.More() {
		var tx Transaction
		if err := j.decoder.Decode(&tx); err != nil {
			return fmt.Errorf("failed to decode transaction: %v", err)
		}

		// 添加到交易池
		if err := add(&tx); err != nil {
			return fmt.Errorf("failed to add transaction: %v", err)
		}
	}

	return nil
}

// insert 插入交易到日志
func (j *txJournal) insert(tx *Transaction) error {
	if j.writer == nil {
		return fmt.Errorf("journal not opened for writing")
	}

	// 写入交易JSON
	if err := json.NewEncoder(j.writer).Encode(tx); err != nil {
		return fmt.Errorf("failed to encode transaction: %v", err)
	}

	// 刷新到磁盘
	return j.writer.Sync()
}

// rotate 轮转日志文件
func (j *txJournal) rotate(txs []*Transaction) error {
	// 关闭当前文件
	if j.writer != nil {
		j.writer.Close()
		j.writer = nil
	}

	// 创建目录
	if err := os.MkdirAll(filepath.Dir(j.path), 0755); err != nil {
		return fmt.Errorf("failed to create journal directory: %v", err)
	}

	// 打开新文件
	file, err := os.OpenFile(j.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open journal file: %v", err)
	}
	j.writer = file

	// 写入文件头
	header := struct {
		Version string `json:"version"`
		Date    string `json:"date"`
	}{
		Version: "1.0",
		Date:    time.Now().Format(time.RFC3339),
	}
	if err := json.NewEncoder(j.writer).Encode(header); err != nil {
		return fmt.Errorf("failed to encode journal header: %v", err)
	}

	// 写入所有交易
	for _, tx := range txs {
		if err := j.insert(tx); err != nil {
			return err
		}
	}

	return nil
}

// close 关闭日志文件
func (j *txJournal) close() error {
	if j.writer != nil {
		return j.writer.Close()
	}
	return nil
}
