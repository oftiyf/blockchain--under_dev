package main

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// NonInclusionCircuit 定义非包含证明的电路
// 结构体定义建议也放在这里，便于管理
// 如果有 tag 需求可自行补充
//
//	type merkleCircuit struct {
//	    RootHash     frontend.Variable `gnark:",public"`
//	    Path, Helper []frontend.Variable
//	}
type NonInclusionCircuit struct {
	RootHash    frontend.Variable `gnark:",public"`
	Path        []frontend.Variable
	Helper      []frontend.Variable
	KeyPath     frontend.Variable
	BranchIndex []frontend.Variable // 存储分支节点的子节点索引
}

func (circuit *NonInclusionCircuit) Define(api frontend.API) error {
	// 初始化 MiMC 哈希函数
	mimc, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}

	// 手动验证路径
	current := circuit.RootHash
	for i := 0; i < len(circuit.Path); i++ {
		// 根据辅助信息和分支索引选择哈希顺序
		left := api.Select(circuit.Helper[i], circuit.Path[i], current)
		right := api.Select(circuit.Helper[i], current, circuit.Path[i])
		mimc.Write(left)
		mimc.Write(right)
		current = mimc.Sum()
	}

	// 验证最终结果不等于目标路径
	api.AssertIsDifferent(current, circuit.KeyPath)

	return nil
}

// type merkleCircuit struct {
//     RootHash     frontend.Variable `gnark:",public"`
//     Path, Helper []frontend.Variable
// }

// func (circuit *merkleCircuit) Define(api frontend.API) error {
//     hFunc, _ := mimc.NewMiMC(api.Curve())
//     merkle.VerifyProof(cs, hFunc, circuit.RootHash, circuit.Path, circuit.Helper)
//     return nil
// }
