package main

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// MerkleCircuit 定义电路结构
type MerkleCircuit struct {
	LeafHash   frontend.Variable   // 私有输入：叶子节点的哈希值
	PathHashes []frontend.Variable // 私有输入：路径上的兄弟节点哈希值
	PathBits   []frontend.Variable // 私有输入：路径方向（0表示左，1表示右）
	NewRoot    frontend.Variable   // 公共输入：新的根哈希
}

// Define 定义电路约束
func (circuit *MerkleCircuit) Define(api frontend.API) error {
	// 初始化 MiMC 哈希函数
	hash, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}

	// 从叶子节点开始计算哈希值
	currentHash := circuit.LeafHash

	// 遍历路径上的兄弟节点哈希值和方向
	for i := 0; i < len(circuit.PathHashes); i++ {
		siblingHash := circuit.PathHashes[i]
		direction := circuit.PathBits[i]

		// 根据方向决定哈希顺序
		left := api.Select(direction, siblingHash, currentHash)
		right := api.Select(direction, currentHash, siblingHash)

		// 计算父节点的哈希值
		hash.Write(left)
		hash.Write(right)
		currentHash = hash.Sum()
	}

	// 验证计算得到的根哈希是否等于提供的 newRoot
	api.AssertIsEqual(currentHash, circuit.NewRoot)

	return nil
}
