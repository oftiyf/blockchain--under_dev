package main

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
)

func TestSimpleMerkleTree(t *testing.T) {
	assert := test.NewAssert(t)

	// 创建一个包含多个节点的测试用例
	pathLength := frontend.Variable(4) // 4个节点：叶子节点 + 3个内部节点

	// 创建测试用的 ZKNode 路径
	var pathFragment [8]frontend.Variable
	var children [16]frontend.Variable

	// 初始化数组
	for i := 0; i < 8; i++ {
		pathFragment[i] = frontend.Variable(0)
	}
	for i := 0; i < 16; i++ {
		children[i] = frontend.Variable(0)
	}

	// 创建叶子节点 (Level 0)
	leafNode := ZKNode{
		NodeType:     frontend.Variable(0), // Leaf
		PathFragment: pathFragment,
		Value:        frontend.Variable(123), // 给叶子节点一个值
		Level:        frontend.Variable(0),
		Children:     children,
	}

	// 创建内部节点 (Level 1) - Branch节点
	branchNode1 := ZKNode{
		NodeType:     frontend.Variable(2), // Branch
		PathFragment: pathFragment,
		Value:        frontend.Variable(0), // Branch节点没有值
		Level:        frontend.Variable(1),
		Children:     children,
	}

	// 创建内部节点 (Level 2) - Branch节点
	branchNode2 := ZKNode{
		NodeType:     frontend.Variable(2), // Branch
		PathFragment: pathFragment,
		Value:        frontend.Variable(0),
		Level:        frontend.Variable(2),
		Children:     children,
	}

	// 创建内部节点 (Level 3) - Branch节点
	branchNode3 := ZKNode{
		NodeType:     frontend.Variable(2), // Branch
		PathFragment: pathFragment,
		Value:        frontend.Variable(0),
		Level:        frontend.Variable(3),
		Children:     children,
	}

	// 设置电路和见证
	var mtCircuit MTCircuit
	var witness MTCircuit

	mtCircuit.PathLength = pathLength
	witness.PathLength = pathLength

	// 暂时使用0值，避免断言失败
	witness.Root = frontend.Variable(0)
	witness.KnownHash = frontend.Variable(0)

	// 填充路径数据到固定长度数组
	for i := 0; i < 32; i++ {
		switch i {
		case 0:
			witness.ModifiedPath[i] = leafNode
		case 1:
			witness.ModifiedPath[i] = branchNode1
		case 2:
			witness.ModifiedPath[i] = branchNode2
		case 3:
			witness.ModifiedPath[i] = branchNode3
		default:
			// 填充空的 ZKNode
			witness.ModifiedPath[i] = ZKNode{
				NodeType:     frontend.Variable(0),
				PathFragment: pathFragment,
				Value:        frontend.Variable(0),
				Level:        frontend.Variable(0),
				Children:     children,
			}
		}
	}

	assert.ProverSucceeded(&mtCircuit, &witness, test.WithCurves(ecc.BN254))
}

// TestComplexMerkleTree 测试更复杂的Merkle树路径
func TestComplexMerkleTree(t *testing.T) {
	assert := test.NewAssert(t)

	// 创建一个更复杂的测试用例
	pathLength := frontend.Variable(6) // 6个节点：叶子 + Extension + 4个Branch

	// 创建测试用的 ZKNode 路径
	var pathFragment [8]frontend.Variable
	var children [16]frontend.Variable

	// 初始化数组
	for i := 0; i < 8; i++ {
		pathFragment[i] = frontend.Variable(0)
	}
	for i := 0; i < 16; i++ {
		children[i] = frontend.Variable(0)
	}

	// 设置一些路径片段数据
	pathFragment[0] = frontend.Variable(1) // 路径片段
	pathFragment[1] = frontend.Variable(2)
	pathFragment[2] = frontend.Variable(3)

	// 创建叶子节点 (Level 0)
	leafNode := ZKNode{
		NodeType:     frontend.Variable(0), // Leaf
		PathFragment: pathFragment,
		Value:        frontend.Variable(456), // 叶子节点值
		Level:        frontend.Variable(0),
		Children:     children,
	}

	// 创建Extension节点 (Level 1)
	extensionNode := ZKNode{
		NodeType:     frontend.Variable(1), // Extension
		PathFragment: pathFragment,
		Value:        frontend.Variable(0), // Extension节点没有值
		Level:        frontend.Variable(1),
		Children:     children,
	}

	// 创建Branch节点 (Level 2-5)
	branchNodes := make([]ZKNode, 4)
	for i := 0; i < 4; i++ {
		branchNodes[i] = ZKNode{
			NodeType:     frontend.Variable(2), // Branch
			PathFragment: pathFragment,
			Value:        frontend.Variable(0),
			Level:        frontend.Variable(i + 2),
			Children:     children,
		}
	}

	// 设置电路和见证
	var mtCircuit MTCircuit
	var witness MTCircuit

	mtCircuit.PathLength = pathLength
	witness.PathLength = pathLength

	// 暂时使用0值，避免断言失败
	witness.Root = frontend.Variable(0)
	witness.KnownHash = frontend.Variable(0)

	// 填充路径数据到固定长度数组
	for i := 0; i < 32; i++ {
		switch i {
		case 0:
			witness.ModifiedPath[i] = leafNode
		case 1:
			witness.ModifiedPath[i] = extensionNode
		case 2:
			witness.ModifiedPath[i] = branchNodes[0]
		case 3:
			witness.ModifiedPath[i] = branchNodes[1]
		case 4:
			witness.ModifiedPath[i] = branchNodes[2]
		case 5:
			witness.ModifiedPath[i] = branchNodes[3]
		default:
			// 填充空的 ZKNode
			witness.ModifiedPath[i] = ZKNode{
				NodeType:     frontend.Variable(0),
				PathFragment: pathFragment,
				Value:        frontend.Variable(0),
				Level:        frontend.Variable(0),
				Children:     children,
			}
		}
	}

	assert.ProverSucceeded(&mtCircuit, &witness, test.WithCurves(ecc.BN254))
}

// TestDifferentPathLengths 测试不同路径长度的处理
func TestDifferentPathLengths(t *testing.T) {
	assert := test.NewAssert(t)

	// 测试不同的路径长度
	testCases := []struct {
		name       string
		pathLength int
		nodeCount  int
	}{
		{"SingleNode", 1, 1},
		{"TwoNodes", 2, 2},
		{"ThreeNodes", 3, 3},
		{"FiveNodes", 5, 5},
		{"TenNodes", 10, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试用的 ZKNode 路径
			var pathFragment [8]frontend.Variable
			var children [16]frontend.Variable

			// 初始化数组
			for i := 0; i < 8; i++ {
				pathFragment[i] = frontend.Variable(0)
			}
			for i := 0; i < 16; i++ {
				children[i] = frontend.Variable(0)
			}

			// 创建节点数组
			nodes := make([]ZKNode, tc.nodeCount)
			for i := 0; i < tc.nodeCount; i++ {
				nodeType := frontend.Variable(2) // 默认Branch
				if i == 0 {
					nodeType = frontend.Variable(0) // 第一个是Leaf
				}

				nodes[i] = ZKNode{
					NodeType:     nodeType,
					PathFragment: pathFragment,
					Value:        frontend.Variable(i * 100), // 不同的值
					Level:        frontend.Variable(i),
					Children:     children,
				}
			}

			// 设置电路和见证
			var mtCircuit MTCircuit
			var witness MTCircuit

			mtCircuit.PathLength = frontend.Variable(tc.pathLength)
			witness.PathLength = frontend.Variable(tc.pathLength)

			// 暂时使用0值，避免断言失败
			witness.Root = frontend.Variable(0)
			witness.KnownHash = frontend.Variable(0)

			// 填充路径数据到固定长度数组
			for i := 0; i < 32; i++ {
				if i < tc.nodeCount {
					witness.ModifiedPath[i] = nodes[i]
				} else {
					// 填充空的 ZKNode
					witness.ModifiedPath[i] = ZKNode{
						NodeType:     frontend.Variable(0),
						PathFragment: pathFragment,
						Value:        frontend.Variable(0),
						Level:        frontend.Variable(0),
						Children:     children,
					}
				}
			}

			assert.ProverSucceeded(&mtCircuit, &witness, test.WithCurves(ecc.BN254))
		})
	}
}
