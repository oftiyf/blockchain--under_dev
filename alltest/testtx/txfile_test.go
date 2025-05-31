package testtx

import (
	"blockchain/common"
	"blockchain/tx"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

// 使用固定的测试私钥
var testPrivateKeyHex = "0000000000000000000000000000000000000000000000000000000000000001"

// setupTestKey 创建测试用的私钥和地址
func setupTestKey(t *testing.T) (*common.Address, []byte) {
	privateKeyBytes, err := hex.DecodeString(testPrivateKeyHex)
	assert.NoError(t, err)

	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	assert.NoError(t, err)

	fromAddress := common.Address{}.NewAddress(crypto.PubkeyToAddress(privateKey.PublicKey).Bytes())
	return &fromAddress, privateKeyBytes
}

// setupTestAddress 创建测试用的目标地址
func setupTestAddress(t *testing.T) *common.Address {
	toAddressBytes, err := hex.DecodeString("742d35cc6634c0532925a3b844bc454e4438f44e")
	assert.NoError(t, err)
	assert.Equal(t, 20, len(toAddressBytes), "address bytes length must be 20")

	toAddress := common.Address{}.NewAddress(toAddressBytes)
	return &toAddress
}

func TestTransactionCreation(t *testing.T) {
	// 测试创建基本交易
	toAddress := setupTestAddress(t)

	testCases := []struct {
		name     string
		nonce    uint64
		value    *big.Int
		gasPrice *big.Int
		gasLimit uint64
		data     []byte
		chainID  *big.Int
	}{
		{
			name:     "Basic Transfer",
			nonce:    0,
			value:    big.NewInt(1000000000000000000), // 1 ETH
			gasPrice: big.NewInt(20000000000),         // 20 Gwei
			gasLimit: 21000,
			data:     nil,
			chainID:  big.NewInt(1),
		},
		{
			name:     "Contract Interaction",
			nonce:    1,
			value:    big.NewInt(0),
			gasPrice: big.NewInt(20000000000),
			gasLimit: 100000,
			data:     []byte("Hello, Blockchain!"),
			chainID:  big.NewInt(1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := tx.NewTransaction(
				tc.nonce,
				*toAddress,
				tc.value,
				tc.gasLimit,
				tc.gasPrice,
				tc.data,
				tc.chainID,
			)

			assert.NotNil(t, tx)
			assert.Equal(t, tc.nonce, tx.Nonce)
			assert.Equal(t, tc.value.String(), tx.Value.String())
			assert.Equal(t, tc.gasPrice.String(), tx.GasPrice.String())
			assert.Equal(t, tc.gasLimit, tx.GasLimit)
			assert.Equal(t, tc.data, tx.Data)
			assert.Equal(t, tc.chainID.String(), tx.ChainID.String())
		})
	}
}

func TestTransactionSerialization(t *testing.T) {
	toAddress := setupTestAddress(t)

	// 创建测试交易
	testTx := tx.NewTransaction(
		0,
		*toAddress,
		big.NewInt(1000000000000000000),
		21000,
		big.NewInt(20000000000),
		nil,
		big.NewInt(1),
	)

	// 测试序列化
	serializedData, err := testTx.Serialize()
	assert.NoError(t, err)
	assert.NotNil(t, serializedData)
	t.Logf("序列化结果: %s", hex.EncodeToString(serializedData))

	// 测试反序列化
	decodedTx, err := tx.Deserialize(serializedData)
	assert.NoError(t, err)
	assert.NotNil(t, decodedTx)

	// 验证反序列化后的数据
	assert.Equal(t, testTx.Nonce, decodedTx.Nonce)
	assert.Equal(t, testTx.GasPrice.String(), decodedTx.GasPrice.String())
	assert.Equal(t, testTx.GasLimit, decodedTx.GasLimit)
	assert.Equal(t, testTx.To.String(), decodedTx.To.String())
	assert.Equal(t, testTx.Value.String(), decodedTx.Value.String())
	assert.Equal(t, testTx.ChainID.String(), decodedTx.ChainID.String())
}

func TestTransactionSigning(t *testing.T) {
	fromAddress, privateKeyBytes := setupTestKey(t)
	toAddress := setupTestAddress(t)

	// 创建测试交易
	testTx := tx.NewTransaction(
		0,
		*toAddress,
		big.NewInt(1000000000000000000),
		21000,
		big.NewInt(20000000000),
		nil,
		big.NewInt(1),
	)

	// 测试签名
	err := testTx.Sign(privateKeyBytes)
	assert.NoError(t, err)
	assert.NotNil(t, testTx.R)
	assert.NotNil(t, testTx.S)
	assert.NotNil(t, testTx.V)
	t.Logf("签名: r=%s\ns=%s\nv=%s", testTx.R.String(), testTx.S.String(), testTx.V.String())

	// 测试从签名恢复地址
	recoveredAddress, err := testTx.GetSender()
	assert.NoError(t, err)
	t.Logf("恢复地址: %s", recoveredAddress.String())
	assert.Equal(t, fromAddress.String(), recoveredAddress.String())

	// 测试交易哈希
	hash, err := testTx.GetHash()
	assert.NoError(t, err)
	t.Logf("交易哈希: %s", hash.String())
	assert.NotEqual(t, common.Hash{}, hash)
}

func TestTransactionWithData(t *testing.T) {
	fromAddress, privateKeyBytes := setupTestKey(t)
	toAddress := setupTestAddress(t)

	// 创建带有数据的交易
	data := []byte("Hello, Blockchain!")
	testTx := tx.NewTransaction(
		1,
		*toAddress,
		big.NewInt(0),
		100000,
		big.NewInt(20000000000),
		data,
		big.NewInt(1),
	)

	// 测试序列化和反序列化
	serializedData, err := testTx.Serialize()
	assert.NoError(t, err)
	t.Logf("序列化结果: %s", hex.EncodeToString(serializedData))

	decodedTx, err := tx.Deserialize(serializedData)
	assert.NoError(t, err)
	t.Logf("反序列化: Nonce=%d, Data=%s", decodedTx.Nonce, string(decodedTx.Data))
	assert.Equal(t, data, decodedTx.Data)

	// 测试签名和验证
	err = testTx.Sign(privateKeyBytes)
	assert.NoError(t, err)
	t.Logf("签名: r=%s\ns=%s\nv=%s", testTx.R.String(), testTx.S.String(), testTx.V.String())

	recoveredAddress, err := testTx.GetSender()
	assert.NoError(t, err)
	t.Logf("恢复地址: %s", recoveredAddress.String())
	assert.Equal(t, fromAddress.String(), recoveredAddress.String())
}
