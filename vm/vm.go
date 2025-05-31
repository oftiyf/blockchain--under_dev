package vm

import (
	"blockchain/common"
	"blockchain/mpt"
	"blockchain/tx"
	"errors"
	"math/big"
)

// VM represents the Ethereum Virtual Machine
type VM struct {
	stateDB *mpt.MPT
}

// NewVM creates a new VM instance
func NewVM(stateDB *mpt.MPT) *VM {
	return &VM{
		stateDB: stateDB,
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
	senderBalance, err := vm.getBalance(sender)
	if err != nil {
		return err
	}

	// 计算交易总成本
	totalCost := new(big.Int).Mul(tx.GasPrice, new(big.Int).SetUint64(tx.GasLimit))
	totalCost.Add(totalCost, tx.Value)

	if senderBalance.Cmp(totalCost) < 0 {
		return errors.New("insufficient balance")
	}

	// 4. 执行交易
	if tx.To == nil {
		// 合约创建
		return vm.createContract(tx, sender)
	} else {
		// 普通转账或合约调用
		return vm.transfer(tx, sender)
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

// getBalance retrieves the balance of an account
func (vm *VM) getBalance(addr common.Address) (*big.Int, error) {
	balanceKey := append([]byte("balance:"), addr.Bytes()...)
	balanceData, err := vm.stateDB.Get(balanceKey)
	if err != nil {
		// 如果账户不存在，返回0余额
		return big.NewInt(0), nil
	}
	return new(big.Int).SetBytes(balanceData), nil
}

// setBalance sets the balance of an account
func (vm *VM) setBalance(addr common.Address, balance *big.Int) error {
	balanceKey := append([]byte("balance:"), addr.Bytes()...)
	return vm.stateDB.Put(balanceKey, balance.Bytes())
}

// createContract handles contract creation
func (vm *VM) createContract(tx *tx.Transaction, sender common.Address) error {
	// 1. 计算合约地址
	nonceBytes := []byte{byte(tx.Nonce)}
	hash := common.Hash{}.NewHash(append(sender.Bytes(), nonceBytes...))
	contractAddr := common.Address{}.NewAddress(hash[:20])

	// 2. 检查合约地址是否已存在
	balanceKey := append([]byte("balance:"), contractAddr.Bytes()...)
	exists, err := vm.stateDB.Get(balanceKey)
	if err == nil && exists != nil {
		return errors.New("contract address already exists")
	}

	// 3. 扣除发送者余额
	senderBalance, err := vm.getBalance(sender)
	if err != nil {
		return err
	}

	totalCost := new(big.Int).Mul(tx.GasPrice, new(big.Int).SetUint64(tx.GasLimit))
	totalCost.Add(totalCost, tx.Value)

	newSenderBalance := new(big.Int).Sub(senderBalance, totalCost)
	if err := vm.setBalance(sender, newSenderBalance); err != nil {
		return err
	}

	// 4. 设置合约初始余额
	if err := vm.setBalance(contractAddr, tx.Value); err != nil {
		return err
	}

	// 5. 存储合约代码
	codeKey := append([]byte("code:"), contractAddr.Bytes()...)
	if err := vm.stateDB.Put(codeKey, tx.Data); err != nil {
		return err
	}

	return nil
}

// transfer handles value transfer and contract calls
func (vm *VM) transfer(tx *tx.Transaction, sender common.Address) error {
	// 1. 扣除发送者余额
	senderBalance, err := vm.getBalance(sender)
	if err != nil {
		return err
	}

	totalCost := new(big.Int).Mul(tx.GasPrice, new(big.Int).SetUint64(tx.GasLimit))
	totalCost.Add(totalCost, tx.Value)

	newSenderBalance := new(big.Int).Sub(senderBalance, totalCost)
	if err := vm.setBalance(sender, newSenderBalance); err != nil {
		return err
	}

	// 2. 增加接收者余额
	receiverBalance, err := vm.getBalance(*tx.To)
	if err != nil {
		return err
	}

	newReceiverBalance := new(big.Int).Add(receiverBalance, tx.Value)
	if err := vm.setBalance(*tx.To, newReceiverBalance); err != nil {
		return err
	}

	// 3. 如果是合约调用，执行合约代码
	codeKey := append([]byte("code:"), tx.To.Bytes()...)
	code, err := vm.stateDB.Get(codeKey)
	if err == nil && code != nil {
		// TODO: 实现合约代码执行逻辑
		// 这里需要实现EVM的指令集和合约执行逻辑
	}

	return nil
}
