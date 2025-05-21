package mpt

import (
	"blockchain/common"
	"encoding/json"
	"fmt"
)

const (
	LeafNodeType      NodeType = iota
	ExtensionNodeType          = 1
	BranchNodeType             = 2
)

type Node interface {
	GetHash() common.Hash
	GetType() NodeType
	Serialize() ([]byte, error)
}

type NodeType int
type LeafNode struct {
	Value    common.Hash `json:"value"`
	Key      []byte      `json:"key"`
	NodeType NodeType    `json:"nodeType"`
}

func (n *LeafNode) GetType() NodeType {
	return LeafNodeType
}

func (n *LeafNode) Serialize() ([]byte, error) {
	data, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (n *LeafNode) GetHash() common.Hash {
	data, _ := n.Serialize()
	return sha3_256(data)
}

type ExtensionNode struct {
	Path     []byte      `json:"path"`
	Value    common.Hash `json:"value"`
	NodeType NodeType    `json:"nodeType"`
}

func (n *ExtensionNode) GetType() NodeType {
	return ExtensionNodeType
}

func (n *ExtensionNode) Serialize() ([]byte, error) {
	data, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (n *ExtensionNode) GetHash() common.Hash {
	data, _ := n.Serialize()
	return sha3_256(data)
}

type BranchNode struct {
	Children [16]*MPTNode `json:"children"`
	Value    common.Hash  `json:"value"`
	NodeType NodeType     `json:"nodeType"`
}

func (n *BranchNode) GetType() NodeType {
	return BranchNodeType
}

func (n *BranchNode) Serialize() ([]byte, error) {
	data, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (n *BranchNode) GetHash() common.Hash {
	data, _ := n.Serialize()
	return sha3_256(data)
}

func newBranchNode() *BranchNode {
	return &BranchNode{
		Children: [16]*MPTNode{},
		Value:    common.Hash{},
		NodeType: BranchNodeType,
	}
}

func deserializeNode(data []byte) (Node, error) {
	// 首先解析出节点类型
	var nodeType struct {
		NodeType NodeType `json:"nodeType"`
	}
	if err := json.Unmarshal(data, &nodeType); err != nil {
		return nil, err
	}

	// 根据节点类型反序列化对应的节点
	switch nodeType.NodeType {
	case LeafNodeType:
		var node LeafNode
		if err := json.Unmarshal(data, &node); err != nil {
			return nil, err
		}
		return &node, nil
	case ExtensionNodeType:
		var node ExtensionNode
		if err := json.Unmarshal(data, &node); err != nil {
			return nil, err
		}
		return &node, nil
	case BranchNodeType:
		var node BranchNode
		if err := json.Unmarshal(data, &node); err != nil {
			return nil, err
		}
		return &node, nil
	default:
		return nil, fmt.Errorf("unknown node type: %d", nodeType.NodeType)
	}
}
