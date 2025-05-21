package main

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/gnark/std/merkle"
)

type NonInclusionCircuit struct {
	RootHash frontend.Variable   `gnark:",public"` // MPT的根哈希值
	Path     []frontend.Variable // 从根到终止节点的路径
	Helper   []frontend.Variable // 用于验证的辅助信息
	KeyPath  frontend.Variable   // 被查找的键的路径
}

func (c *NonInclusionCircuit) Define(api frontend.API) error {
	// 初始化哈希函数
	hFunc, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}

	// 验证Merkle路径
	merkle.VerifyProof(api, hFunc, c.RootHash, c.Path, c.Helper)

	// 验证终止节点不包含目标键
	// 这里假设Path的最后一个元素是终止节点的哈希值
	api.AssertIsDifferent(c.Path[len(c.Path)-1], c.KeyPath)

	return nil
}
