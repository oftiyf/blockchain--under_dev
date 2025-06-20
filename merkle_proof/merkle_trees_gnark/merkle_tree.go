package main

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// 我的想法是这样编码，不断hashsum，直到遇到一个level与当前hash不同的一个ZKNode，实现之前的哈希输出，并且结果与当前的ZKNode进行hashsum。
// 然后，将这个ZKNode的level与当前的level进行比较，如果level相同，则继续hashsum，直到遇到一个level与当前hash不同的一个ZKNode，实现之前的哈希输出，并且结果与当前的ZKNode进行hashsum。
type ZKNode struct {
	NodeType     frontend.Variable    // 0=Leaf, 1=Extension, 2=Branch
	PathFragment [8]frontend.Variable // extension 或 leaf 用到的 path，固定长度8
	Value        frontend.Variable    // 仅 leaf 节点使用

	Level frontend.Variable
	// 对于 Branch 节点：Children[0..15]
	// 对于 Leaf/Extension 节点：为空
	Children [16]frontend.Variable // 分支节点16个孩子（没有的用空变量填）
}

type MTCircuit struct {
	KnownHash    frontend.Variable `gnark:",public"`
	Root         frontend.Variable `gnark:",public"`
	PathLength   frontend.Variable `gnark:",public"`
	ModifiedPath [32]ZKNode        // 改为固定长度数组
}

// ComputeNodeHash computes hash for a ZKNode using MiMC
func ComputeNodeHash(api frontend.API, node ZKNode, childHash frontend.Variable) frontend.Variable {
	h, err := mimc.NewMiMC(api)
	if err != nil {
		panic(err)
	}

	h.Reset()
	h.Write(node.NodeType)
	h.Write(node.Level)

	// 写入路径片段
	for i := 0; i < 8; i++ {
		h.Write(node.PathFragment[i])
	}

	// 写入值（如果是叶子节点）
	h.Write(node.Value)

	// 写入子节点哈希
	h.Write(childHash)

	// 写入子节点数组
	for i := 0; i < 16; i++ {
		h.Write(node.Children[i])
	}

	return h.Sum()
}

// InjectChildHash injects child hash into a node
func InjectChildHash(api frontend.API, node ZKNode, childHash frontend.Variable) ZKNode {
	// TODO: Implement child hash injection logic
	// For now, return the node as-is
	return node
}

func (circuit *MTCircuit) Define(api frontend.API) error {
	var previousHash frontend.Variable
	var previousLevel frontend.Variable
	var knownHashMatched frontend.Variable = 0

	// 初始 previousHash 为 leaf 节点 hash
	node := circuit.ModifiedPath[0]
	previousHash = ComputeNodeHash(api, node, api.Sub(0, 0)) // 无子节点
	previousLevel = node.Level

	// 检查 leaf 节点是否等于 KnƒiownHash
	isEqual := api.IsZero(api.Sub(previousHash, circuit.KnownHash))
	knownHashMatched = api.Add(isEqual, knownHashMatched)

	// 从路径第2个节点开始，逐层向上处理
	// 使用固定最大长度，通过条件判断是否处理
	maxPathLength := 32 // 设置一个合理的最大路径长度
	for i := 1; i < maxPathLength; i++ {
		// 检查是否应该处理这个索引：PathLength > i
		// api.Cmp(PathLength, i) 返回: 1(>), 0(=), -1(<)
		// 我们需要 PathLength > i，即 api.Cmp(PathLength, i) == 1
		cmpResult := api.Cmp(circuit.PathLength, frontend.Variable(i))
		shouldProcess := api.Select(api.IsZero(api.Sub(cmpResult, 1)), 1, 0) // 如果cmpResult==1则shouldProcess=1，否则shouldProcess=0

		current := circuit.ModifiedPath[i]

		// 判断是否 level 不同（需要切换组合）
		_ = api.IsZero(api.Sub(current.Level, previousLevel))

		// 如果 level 不同，则用 previousHash 插入到当前节点中进行 hashSum
		// 替换掉 current.Children 中正确的子位置
		adjusted := InjectChildHash(api, current, previousHash)

		currentHash := ComputeNodeHash(api, adjusted, api.Sub(0, 0)) // 重建节点 hash

		// 检查是否匹配 KnownHash
		isEqual := api.IsZero(api.Sub(currentHash, circuit.KnownHash))
		knownHashMatched = api.Add(isEqual, knownHashMatched)

		// 更新上下文 hash、level（仅在应该处理时）
		previousHash = api.Select(shouldProcess, currentHash, previousHash)
		previousLevel = api.Select(shouldProcess, current.Level, previousLevel)
	}

	// 最终得到的 hash 应与 root 相等
	// api.AssertIsEqual(previousHash, circuit.Root)

	// 路径中至少有一个节点 hash == KnownHash
	// hasMatch := api.Select(api.IsZero(knownHashMatched), 0, 1)
	// api.AssertIsEqual(hasMatch, 1)

	return nil
}
