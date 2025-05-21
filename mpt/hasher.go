package mpt

import (
	"blockchain/common"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
)

// MiMC 哈希函数实现
func mimcHash(data []byte) common.Hash {
	// 将字节数据转换为域元素
	var input fr.Element
	input.SetBytes(data)

	// 创建 MiMC 哈希实例
	h := mimc.NewMiMC()

	// 计算哈希
	// 将域元素转换为字节切片
	inputBytes := input.Bytes()
	h.Write(inputBytes[:])
	hash := h.Sum(nil)

	// 转换为 common.Hash
	var result common.Hash
	copy(result[:], hash)
	return result
}

// 为了保持兼容性，保留原来的函数名但使用 MiMC
func sha3_256(data []byte) common.Hash {
	return mimcHash(data)
}
