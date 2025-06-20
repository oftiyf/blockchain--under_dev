package tx

import (
	"blockchain/common"
	"sync"
)

// accountSet 用于管理本地账户集合
// 本地账户的交易享有特殊待遇，不会被价格限制等规则驱逐
type accountSet struct {
	accounts map[common.Address]struct{} // 账户集合
	signer   Signer                      // 签名器，用于验证交易
	mu       sync.RWMutex                // 读写锁
}

// newAccountSet 创建新的accountSet
func newAccountSet(signer Signer) *accountSet {
	return &accountSet{
		accounts: make(map[common.Address]struct{}),
		signer:   signer,
	}
}

// add 添加账户到本地集合
func (as *accountSet) add(addr common.Address) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.accounts[addr] = struct{}{}
}

// remove 从本地集合移除账户
func (as *accountSet) remove(addr common.Address) {
	as.mu.Lock()
	defer as.mu.Unlock()
	delete(as.accounts, addr)
}

// contains 检查账户是否在本地集合中
func (as *accountSet) contains(addr common.Address) bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	_, exists := as.accounts[addr]
	return exists
}

// len 返回本地账户数量
func (as *accountSet) len() int {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return len(as.accounts)
}

// flatten 返回所有本地账户的副本
func (as *accountSet) flatten() []common.Address {
	as.mu.RLock()
	defer as.mu.RUnlock()
	accounts := make([]common.Address, 0, len(as.accounts))
	for addr := range as.accounts {
		accounts = append(accounts, addr)
	}
	return accounts
}

// Signer 接口定义签名器
type Signer interface {
	// Sender 从交易中恢复发送者地址
	Sender(tx *Transaction) (common.Address, error)
}
