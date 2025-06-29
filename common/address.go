package common

import (
	"encoding/hex"
	"fmt"
	"strings"

	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
)

const (
	AddressLength = 20
)

type Address [AddressLength]byte

func (a Address) NewAddress(data []byte) Address {
	if len(data) != AddressLength {
		panic("data length must be 20")
	}
	var address Address
	copy(address[:], data)
	return address
}

func (a Address) Bytes() []byte {
	return a[:]
}

func (a Address) String() string {
	return hex.EncodeToString(a[:])
}

// IsZero returns true if the address is the zero address
func (a Address) IsZero() bool {
	for _, b := range a {
		if b != 0 {
			return false
		}
	}
	return true
}

func PrivateKeyToPublicKey(hexKey string) ([]byte, error) {
	// 移除可能的 "0x" 前缀
	hexKey = strings.TrimPrefix(hexKey, "0x")

	// 将十六进制字符串转换为 ECDSA 私钥
	privateKey, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %v", err)
	}

	// 从私钥获取公钥
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("无法断言类型: publicKey 不是 *ecdsa.PublicKey")
	}

	// 将公钥转换为字节数组（未压缩格式，65 字节，首字节为 0x04）
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)

	return publicKeyBytes, nil
}

func (a Address) PublicKeyToAddress(publicKey []byte) Address {
	// 使用 MIMC 哈希公钥
	hash := Hash{}.NewHash(publicKey)
	return Address{}.NewAddress(hash[:AddressLength])
}

func PirvateKeyToAddress(privateKey []byte) Address {
	//hash the public key
	hash := Hash{}.NewHash(privateKey)
	return Address{}.NewAddress(hash[:AddressLength])
}
