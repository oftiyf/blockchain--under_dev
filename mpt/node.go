package mpt

import (
	"blockchain/common"
	"encoding/hex"
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
	NodeType NodeType    `json:"nodeType"`
	Children [17]Node    `json:"-"`
	Value    common.Hash `json:"value"`//短节点才会考虑这个
	flags    nodeFlag    `json:"-"`
}

type LeafNode struct {
	NodeType NodeType `json:"nodeType"`
	Key      []byte   `json:"key"`
	Value    []byte   `json:"value"`
	flags    nodeFlag `json:"-"`
}

type ExtensionNode struct {
	NodeType NodeType    `json:"nodeType"`
	Path     []byte      `json:"path"`
	Value    common.Hash `json:"value"`
	flags    nodeFlag    `json:"-"`
}

// FullNode JSON序列化结构
type fullNodeJSON struct {
	NodeType NodeType `json:"nodeType"`
	Children []string `json:"children"` // hash hex字符串
	Value    string   `json:"value"`
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

func (n *FullNode) MarshalJSON() ([]byte, error) {
	childrenHashes := make([]string, 17)
	for i, child := range n.Children {
		if child != nil {
			// 先保存子节点
			if _, err := child.Serialize(); err != nil {
				return nil, err
			}
			childrenHashes[i] = hex.EncodeToString(child.GetHash().Bytes())
		} else {
			childrenHashes[i] = ""
		}
	}
	return json.Marshal(&fullNodeJSON{
		NodeType: n.NodeType,
		Children: childrenHashes,
		Value:    hex.EncodeToString(n.Value.Bytes()),
	})
}

func (n *FullNode) UnmarshalJSON(data []byte) error {
	var temp fullNodeJSON
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	n.NodeType = temp.NodeType
	for i, h := range temp.Children {
		if h != "" {
			bytes, err := hex.DecodeString(h)
			if err != nil {
				return err
			}
			var hash common.Hash
			copy(hash[:], bytes)
			// 这里只能存hash，实际访问时需通过DB加载
			n.Children[i] = &ExtensionNode{NodeType: ExtensionNodeType, Path: nil, Value: hash}
		} else {
			n.Children[i] = nil
		}
	}
	if temp.Value != "" {
		bytes, err := hex.DecodeString(temp.Value)
		if err != nil {
			return err
		}
		copy(n.Value[:], bytes)
	}
	return nil
}

// nibblesToBytes 将 nibbles 转换回字节
func nibblesToBytes(nibbles []byte) []byte {
	if len(nibbles) == 0 {
		return nil
	}
	// 确保 nibbles 长度是偶数
	if len(nibbles)%2 != 0 {
		return nil
	}
	bytes := make([]byte, len(nibbles)/2)
	for i := 0; i < len(nibbles); i += 2 {
		if i+1 < len(nibbles) {
			bytes[i/2] = nibbles[i]<<4 | nibbles[i+1]
		}
	}
	return bytes
}

// GetReadableKey 返回可读的键
func (n *LeafNode) GetReadableKey() string {
	bytes := nibblesToBytes(n.Key)
	if bytes == nil {
		return fmt.Sprintf("%x", n.Key) // 如果转换失败，返回十六进制表示
	}
	return string(bytes)
}

// GetReadableValue 返回可读的值
func (n *LeafNode) GetReadableValue() string {
	return string(n.Value)
}

// GetReadablePath 返回可读的路径
func (n *ExtensionNode) GetReadablePath() string {
	bytes := nibblesToBytes(n.Path)
	if bytes == nil {
		return fmt.Sprintf("%x", n.Path) // 如果转换失败，返回十六进制表示
	}
	return string(bytes)
}

// String 返回节点的字符串表示
func (n *LeafNode) String() string {
	return fmt.Sprintf("LeafNode{Type=%d, Key=%s, Value=%s}", n.NodeType, n.GetReadableKey(), n.GetReadableValue())
}

func (n *ExtensionNode) String() string {
	return fmt.Sprintf("ExtensionNode{Type=%d, Path=%s, Value=%x}", n.NodeType, n.GetReadablePath(), n.Value)
}

func (n *FullNode) String() string {
	return fmt.Sprintf("FullNode{Type=%d, Value=%x}", n.NodeType, n.Value)
}
