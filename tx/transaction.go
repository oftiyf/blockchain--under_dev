package tx

import (
	"blockchain/common"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// TxData represents the core data of an Ethereum transaction
type TxData struct {
	Nonce    uint64          // 发送者账户已发送的交易数量
	GasPrice *big.Int        // gas价格
	GasLimit uint64          // 交易允许消耗的最大Gas数量
	To       *common.Address // 接收方地址，nil表示合约创建
	Value    *big.Int        // 转账的以太币数量（以Wei计）
	Data     []byte          // 调用合约函数的编码数据，或部署合约时的字节码
	ChainID  *big.Int        // 链的标识符
}

// SignatureData represents the signature of an Ethereum transaction
type SignatureData struct {
	V *big.Int // 签名值V
	R *big.Int // 签名值R
	S *big.Int // 签名值S
}

// Transaction represents an Ethereum transaction
type Transaction struct {
	TxData
	SignatureData
}

// NewTransaction creates a new transaction
func NewTransaction(nonce uint64, to common.Address, value *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, chainID *big.Int) *Transaction {
	return &Transaction{
		TxData: TxData{
			Nonce:    nonce,
			GasPrice: gasPrice,
			GasLimit: gasLimit,
			To:       &to,
			Value:    value,
			Data:     data,
			ChainID:  chainID,
		},
		SignatureData: SignatureData{
			V: new(big.Int), // 默认值为0
			R: new(big.Int), // 默认值为0
			S: new(big.Int), // 默认值为0
		},
	}
}

// Sign signs the transaction with the given private key
func (tx *Transaction) Sign(privateKey []byte) error {
	// Create a new transaction with only TxData for hashing
	txToSign := &Transaction{
		TxData: tx.TxData, // Use existing TxData
		SignatureData: SignatureData{ // Empty signature for hashing
			V: new(big.Int),
			R: new(big.Int),
			S: new(big.Int),
		},
	}
	hash, err := txToSign.GetHash()
	if err != nil {
		return err
	}

	// 3. 使用私钥签名
	key, err := crypto.ToECDSA(privateKey)
	if err != nil {
		return err
	}

	signature, err := crypto.Sign(hash.Bytes(), key)
	if err != nil {
		return err
	}

	// 4. 解析签名值
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:64])
	v := new(big.Int).SetBytes([]byte{signature[64]})

	// 只需要加上基础值27即可
	v.Add(v, big.NewInt(27))

	tx.SignatureData.R = r
	tx.SignatureData.S = s
	tx.SignatureData.V = v

	return nil
}

// Serialize 序列化交易数据
func (tx *Transaction) Serialize() ([]byte, error) {
	// 准备交易数据
	var data []interface{}
	// 根据EIP-155，V值需要包含链ID
	v := tx.SignatureData.V.Uint64()
	if tx.TxData.ChainID != nil && tx.TxData.ChainID.Sign() > 0 {
		// 如果V值大于等于35，说明已经包含了链ID
		if v < 35 {
			v = v + 35 + 2*tx.TxData.ChainID.Uint64()
		}
	}
	data = []interface{}{
		tx.TxData.Nonce,
		tx.TxData.GasPrice,
		tx.TxData.GasLimit,
		tx.TxData.To,
		tx.TxData.Value,
		tx.TxData.Data,
		tx.TxData.ChainID,
		v,
		tx.SignatureData.R,
		tx.SignatureData.S,
	}

	// 编码数据
	return rlp.EncodeToBytes(data)
}

// GetHash 获取交易哈希
func (tx *Transaction) GetHash() (common.Hash, error) {
	data, err := tx.Serialize()
	if err != nil {
		return common.Hash{}, err
	}
	hash := common.Hash{}.NewHash(data)
	return hash, nil
}

// GetSender 从签名中恢复发送者地址
func (tx *Transaction) GetSender() (common.Address, error) {
	if tx.SignatureData.R == nil || tx.SignatureData.S == nil || tx.SignatureData.V == nil {
		return common.Address{}, errors.New("transaction is not signed")
	}

	// 准备未签名的交易数据 (only TxData part)
	txToHash := &Transaction{
		TxData: tx.TxData,
		SignatureData: SignatureData{ // Empty signature for hashing
			V: new(big.Int),
			R: new(big.Int),
			S: new(big.Int),
		},
	}

	// 计算未签名交易的哈希
	hash, err := txToHash.GetHash()
	if err != nil {
		return common.Address{}, err
	}

	// 创建签名
	sig := make([]byte, 65)
	rBytes := tx.SignatureData.R.Bytes()
	sBytes := tx.SignatureData.S.Bytes()

	// 确保r和s是32字节
	if len(rBytes) > 32 || len(sBytes) > 32 {
		return common.Address{}, errors.New("invalid signature length")
	}

	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)

	// 处理V值
	v := tx.SignatureData.V.Uint64()
	// 提取recovery id (0 或 1)
	recoveryId := v - 27
	if recoveryId > 1 {
		return common.Address{}, errors.New("invalid signature recovery id")
	}
	sig[64] = byte(recoveryId)

	// 恢复公钥
	pubKey, err := crypto.SigToPub(hash.Bytes(), sig)
	if err != nil {
		return common.Address{}, err
	}
	publicKeyBytes := crypto.FromECDSAPub(pubKey)
	// 从公钥获取地址
	addr := common.Address{}.PublicKeyToAddress(publicKeyBytes)
	return addr, nil
}

// Deserialize 从RLP编码的数据反序列化交易
func Deserialize(data []byte) (*Transaction, error) {
	type txRLP struct {
		Nonce    uint64
		GasPrice *big.Int
		GasLimit uint64
		To       *common.Address
		Value    *big.Int
		Data     []byte
		ChainID  *big.Int
		V        uint64
		R        *big.Int
		S        *big.Int
	}

	var dec txRLP
	if err := rlp.DecodeBytes(data, &dec); err != nil {
		return nil, err
	}

	tx := &Transaction{
		TxData: TxData{
			Nonce:    dec.Nonce,
			GasPrice: dec.GasPrice,
			GasLimit: dec.GasLimit,
			To:       dec.To,
			Value:    dec.Value,
			Data:     dec.Data,
			ChainID:  dec.ChainID,
		},
		SignatureData: SignatureData{},
	}
	// 只有当签名存在时才赋值
	if dec.R != nil && dec.S != nil && dec.V != 0 {
		tx.SignatureData.R = dec.R
		tx.SignatureData.S = dec.S
		tx.SignatureData.V = new(big.Int).SetUint64(dec.V)
	} else {
		tx.SignatureData.R = new(big.Int) // Initialize if not present
		tx.SignatureData.S = new(big.Int) // Initialize if not present
		tx.SignatureData.V = new(big.Int) // Initialize if not present
	}
	return tx, nil
}
