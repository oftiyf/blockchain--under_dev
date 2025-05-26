package mpt

import (
	"blockchain/common"
	"encoding/json"
	"fmt"
)

type Node interface {
	GetType() NodeType
	GetHash() common.Hash
	Serialize() ([]byte, error)
}

type NodeType int

const (
	LeafNodeType      NodeType = iota
	ExtensionNodeType          = 1
	BranchNodeType             = 2
)

type nodeFlag struct {
	hash  common.Hash
	dirty bool
}

type FullNode struct {
	Children [17]Node
	Value    common.Hash
	flags    nodeFlag
}

type LeafNode struct {
	Key   []byte
	Value []byte
	flags nodeFlag
}

type ExtensionNode struct {
	Path  []byte
	Value common.Hash
	flags nodeFlag
}

func (n *FullNode) GetType() NodeType {
	return BranchNodeType
}

func (n *LeafNode) GetType() NodeType {
	return LeafNodeType
}

func (n *ExtensionNode) GetType() NodeType {
	return ExtensionNodeType
}

func (n *FullNode) Serialize() ([]byte, error) {
	return json.Marshal(n)
}

func (n *LeafNode) Serialize() ([]byte, error) {
	return json.Marshal(n)
}

func (n *ExtensionNode) Serialize() ([]byte, error) {
	return json.Marshal(n)
}

func (n *FullNode) GetHash() common.Hash {
	if n.flags.hash != (common.Hash{}) {
		return n.flags.hash
	}
	data, _ := n.Serialize()
	hash := sha3_256(data)
	n.flags.hash = hash
	return hash
}

func (n *LeafNode) GetHash() common.Hash {
	if n.flags.hash != (common.Hash{}) {
		return n.flags.hash
	}
	data, _ := n.Serialize()
	hash := sha3_256(data)
	n.flags.hash = hash
	return hash
}

func (n *ExtensionNode) GetHash() common.Hash {
	if n.flags.hash != (common.Hash{}) {
		return n.flags.hash
	}
	data, _ := n.Serialize()
	hash := sha3_256(data)
	n.flags.hash = hash
	return hash
}

func deserializeNode(data []byte) (Node, error) {
	var nodeType struct {
		NodeType NodeType `json:"nodeType"`
	}
	if err := json.Unmarshal(data, &nodeType); err != nil {
		return nil, err
	}

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
		var node FullNode
		if err := json.Unmarshal(data, &node); err != nil {
			return nil, err
		}
		return &node, nil
	default:
		return nil, fmt.Errorf("unknown node type: %d", nodeType.NodeType)
	}
}
