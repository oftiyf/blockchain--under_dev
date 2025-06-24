package vm

import (
	"blockchain/common"
	"blockchain/mpt"
	"blockchain/tx"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
)

// VM represents the Ethereum Virtual Machine
type VM struct {
	stateDB   *mpt.MPT
	mintCount map[string]int // 每个地址独立的mint计数
}

// NewVM creates a new VM instance
func NewVM(stateDB *mpt.MPT) *VM {
	return &VM{
		stateDB:   stateDB,
		mintCount: make(map[string]int),
	}
}

// ExecuteTransaction executes a transaction and updates the state
func (vm *VM) ExecuteTransaction(tx *tx.Transaction) error {
	// 1. 验证交易
	if err := vm.validateTransaction(tx); err != nil {
		return err
	}

	// 2. 获取发送者地址
	sender, err := tx.GetSender()
	if err != nil {
		return err
	}

	// 3. 检查发送者余额
	senderAccount, err := vm.GetAccount(sender)
	if err != nil {
		return err
	}

	// 计算交易总成本
	totalCost := new(big.Int).Mul(tx.GasPrice, new(big.Int).SetUint64(tx.GasLimit))
	totalCost.Add(totalCost, tx.Value)

	// 将总成本转换为uint64进行比较
	if totalCost.Cmp(new(big.Int).SetUint64(senderAccount.Balance)) > 0 {
		fmt.Printf("Debug - Insufficient balance: have %v, need %v\n", senderAccount.Balance, totalCost)
		return errors.New("insufficient balance")
	}

	// 4. 执行交易
	if tx.To == nil || (tx.To.IsZero() && len(tx.Data) != 0) {
		// 执行VM层操作
		fmt.Printf("oftiyfdebug\n")
		return vm.executeVMOperation(tx, sender, totalCost)
	} else {
		// 普通转账或合约调用
		return vm.transfer(tx, sender, totalCost)
	}
}

// validateTransaction validates a transaction
func (vm *VM) validateTransaction(tx *tx.Transaction) error {
	if tx == nil {
		return errors.New("transaction is nil")
	}
	if tx.GasPrice == nil || tx.GasPrice.Sign() <= 0 {
		return errors.New("invalid gas price")
	}
	if tx.GasLimit == 0 {
		return errors.New("invalid gas limit")
	}
	if tx.Value == nil || tx.Value.Sign() < 0 {
		return errors.New("invalid value")
	}
	return nil
}

// GetAccount retrieves the account of an address
func (vm *VM) GetAccount(addr common.Address) (*common.Account, error) {
	// 使用地址的原始字节作为键
	key := addr.Bytes()
	fmt.Printf("Debug - Getting account with key: %x\n", key)
	accountData, err := vm.stateDB.Get(key)
	if err != nil {
		// 如果账户不存在，返回新账户
		fmt.Printf("Debug - Account not found: %v\n", addr)
		return &common.Account{
			Balance: 0,
			Nonce:   0,
		}, nil
	}

	var account common.Account
	if err := json.Unmarshal(accountData, &account); err != nil {
		fmt.Printf("Debug - Failed to unmarshal account data: %v\n", err)
		return nil, err
	}
	fmt.Printf("Debug - Retrieved account for %v: balance=%v, nonce=%v\n", addr, account.Balance, account.Nonce)
	return &account, nil
}

// SetAccount sets the account of an address
func (vm *VM) SetAccount(addr common.Address, account *common.Account) error {
	// 使用地址的原始字节作为键
	key := addr.Bytes()
	fmt.Printf("Debug - Setting account with key: %x\n", key)
	accountData, err := json.Marshal(account)
	if err != nil {
		fmt.Printf("Debug - Failed to marshal account data: %v\n", err)
		return err
	}
	fmt.Printf("Debug - Setting account for %v: balance=%v, nonce=%v\n", addr, account.Balance, account.Nonce)
	return vm.stateDB.Put(key, accountData)
}

// executeVMOperation handles VM-level operations
func (vm *VM) executeVMOperation(tx *tx.Transaction, sender common.Address, totalCost *big.Int) error {
	switch {
	case tx.To == nil || tx.To.IsZero():
		// 创建新合约
		// 计算操作地址
		nonceBytes := []byte{byte(tx.Nonce)}
		hash := common.Hash{}.NewHash(append(sender.Bytes(), nonceBytes...))
		operationAddr := common.Address{}.NewAddress(hash[:20])

		// 检查操作地址是否已存在
		exists, err := vm.stateDB.Get(operationAddr.Bytes())
		if err == nil && exists != nil {
			return errors.New("operation address already exists")
		}

		// 1. 扣除发送者余额
		senderAccount, err := vm.GetAccount(sender)
		if err != nil {
			fmt.Printf("Debug - Failed to get sender account: %v\n", err)
			return err
		}

		senderAccount.Balance -= totalCost.Uint64()
		senderAccount.Nonce++
		if err := vm.SetAccount(sender, senderAccount); err != nil {
			fmt.Printf("Debug - Failed to set sender account: %v\n", err)
			return err
		}

		// 设置操作初始状态
		operationAccount := &common.Account{
			Nonce:    0,
			Balance:  tx.Value.Uint64(),
			CodeHash: common.Hash{}.NewHash(tx.Data).Bytes(),
			Code:     tx.Data,
			Storage:  make(map[string]string),
			IsEoa:    false,
		}
		if err := vm.SetAccount(operationAddr, operationAccount); err != nil {
			fmt.Printf("Debug - Failed to set operation account: %v\n", err)
			return err
		}
		fmt.Printf("Debug - Operation account: %v\n", operationAccount)

	default:
		// 调用已存在的合约
		// 1. 扣除发送者余额
		senderAccount, err := vm.GetAccount(sender)
		if err != nil {
			return err
		}

		senderAccount.Balance -= totalCost.Uint64()
		senderAccount.Nonce++
		if err := vm.SetAccount(sender, senderAccount); err != nil {
			return err
		}

		contractAccount, err := vm.GetAccount(*tx.To)
		if err != nil {
			return err
		}

		if len(contractAccount.Code) == 0 {
			return errors.New("not a contract address")
		}
		fmt.Printf("12312313212 %v\n", contractAccount)
		// TODO: 实现合约调用逻辑
		// 这里需要实现EVM的指令集和合约执行逻辑
	}

	return nil
}

// transfer handles value transfer and contract calls
func (vm *VM) transfer(tx *tx.Transaction, sender common.Address, totalCost *big.Int) error {
	// 1. 扣除发送者余额
	senderAccount, err := vm.GetAccount(sender)
	if err != nil {
		return err
	}

	senderAccount.Balance -= totalCost.Uint64()
	fmt.Printf("Debug - totalCostaawdsaw: %v\n", totalCost)
	fmt.Printf("Debug - tx.Value: %v\n", tx.Value)
	fmt.Printf("Debug - Sender balance before transfer123321: %v\n", senderAccount.Balance)
	senderAccount.Nonce++
	fmt.Printf("Debug - Sender nonce after deducting cost: %v, new nonce: %v\n", senderAccount.Balance, senderAccount.Nonce)
	if err := vm.SetAccount(sender, senderAccount); err != nil {
		return err
	}

	// 2. 增加接收者余额
	receiverAccount, err := vm.GetAccount(*tx.To)
	if err != nil {
		return err
	}

	receiverAccount.Balance += tx.Value.Uint64()
	if err := vm.SetAccount(*tx.To, receiverAccount); err != nil {
		return err
	}

	// 3. 如果是合约调用，执行合约代码
	if len(receiverAccount.Code) > 0 {
		// TODO: 实现合约代码执行逻辑
		// 这里需要实现EVM的指令集和合约执行逻辑
	}

	return nil
}

// Mint adds tokens to an address
func (vm *VM) Mint(addr common.Address) error {
	// 检查mint次数是否超过限制
	addrStr := addr.String()
	if vm.mintCount[addrStr] >= 5 {
		return errors.New("mint limit exceeded")
	}

	// 获取当前账户
	account, err := vm.GetAccount(addr)
	if err != nil {
		return err
	}

	// 增加100个代币
	mintAmount := uint64(1000000)
	account.Balance += mintAmount
	fmt.Printf("Debug - Mint address: %v\n", addr)
	fmt.Printf("Debug - New balance: %v\n", account.Balance)

	// 更新账户
	if err := vm.SetAccount(addr, account); err != nil {
		return err
	}
	afterAccount, err := vm.GetAccount(addr)
	fmt.Printf("Debug - After mint address: %v\n", addr)
	fmt.Printf("Debug - After mint balance: %v\n", afterAccount.Balance)

	// 增加mint计数
	vm.mintCount[addrStr]++

	return nil
}

// GetMintCount returns the current mint count for an address
func (vm *VM) GetMintCount(addr common.Address) int {
	return vm.mintCount[addr.String()]
}
