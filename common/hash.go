package common

import (
	"encoding/hex"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
)

const (
	HashLength = 32
)

type Hash [HashLength]byte

func (h Hash) NewHash(data []byte) Hash {
	// 将字节数据转换为域元素
	var input fr.Element
	input.SetBytes(data)

	// 创建 MiMC 哈希实例
	mimcHash := mimc.NewMiMC()

	// 计算哈希
	inputBytes := input.Bytes()
	mimcHash.Write(inputBytes[:])
	hash := mimcHash.Sum(nil)

	// 转换为 Hash
	var result Hash
	copy(result[:], hash)
	return result
}

func (h Hash) Bytes() []byte {
	return h[:]
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}
