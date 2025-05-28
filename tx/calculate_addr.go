package tx

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// VerifyAddress 验证公钥哈希与签名中的地址是否一致
// pubKey: 发送者的公钥
// address: 签名中的地址
// 返回值: 如果一致返回true，否则返回false和错误信息
func VerifyAddress(pubKey []byte, address string) (bool, error) {
	// 计算公钥的SHA256哈希
	hash := sha256.Sum256(pubKey)

	// 将哈希转换为十六进制字符串
	hashHex := hex.EncodeToString(hash[:])

	// 比较哈希值与地址是否一致
	if hashHex == address {
		return true, nil
	}

	return false, errors.New("public key hash does not match the address in signature")
}
