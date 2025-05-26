package mpt

import (
	"blockchain/common"
	"bytes"
	"fmt"
)

// MPT represents a Merkle Patricia Trie
type MPT struct {
	Root Node
	DB   *DB
}

// NewMPT creates a new MPT instance
func NewMPT(db *DB) *MPT {
	return &MPT{
		Root: nil,
		DB:   db,
	}
}

// LoadNode loads a node from the database by its hash
func (m *MPT) LoadNode(hash common.Hash) (Node, error) {
	// Get the node data from database
	data, err := m.DB.Get(hash.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to load node: %v", err)
	}
	if data == nil {
		return nil, fmt.Errorf("node not found: %x", hash)
	}

	// Deserialize the node
	return deserializeNode(data)
}

// Put inserts or updates a key-value pair in the trie
func (m *MPT) Put(key, value []byte) error {
	// Convert key to nibbles (hex)
	nibbles := keyToNibbles(key)
	fmt.Printf("Put: key=%x, value=%x, nibbles=%x\n", key, value, nibbles)

	// If root is nil, create a new leaf node
	if m.Root == nil {
		m.Root = &LeafNode{
			NodeType: LeafNodeType,
			Key:      nibbles,
			Value:    value,
			flags:    nodeFlag{},
		}
		fmt.Printf("Create root LeafNode: key=%x, value=%x\n", nibbles, value)
		return m.saveNode(m.Root)
	}

	// Insert the key-value pair
	newRoot, err := m.insert(m.Root, nibbles, value)
	if err != nil {
		return err
	}
	m.Root = newRoot
	return m.saveNode(m.Root)
}

// Get retrieves the value for a given key
func (m *MPT) Get(key []byte) ([]byte, error) {
	if m.Root == nil {
		return nil, fmt.Errorf("empty trie")
	}

	nibbles := keyToNibbles(key)
	value, err := m.get(m.Root, nibbles)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Delete removes a key-value pair from the trie
func (m *MPT) Delete(key []byte) error {
	if m.Root == nil {
		return fmt.Errorf("empty trie")
	}

	nibbles := keyToNibbles(key)
	newRoot, err := m.delete(m.Root, nibbles)
	if err != nil {
		return err
	}
	m.Root = newRoot
	if m.Root != nil {
		return m.saveNode(m.Root)
	}
	return nil
}

// insert recursively inserts a key-value pair into the trie
func (m *MPT) insert(node Node, nibbles []byte, value []byte) (Node, error) {
	switch n := node.(type) {
	case *LeafNode:
		fmt.Printf("LeafNode insert: existing key=%x, new nibbles=%x, commonPrefix=%x\n", n.Key, nibbles, findCommonPrefix(n.Key, nibbles))
		commonPrefix := findCommonPrefix(n.Key, nibbles)

		// 如果键完全匹配，更新值
		if bytes.Equal(n.Key, nibbles) {
			n.Value = value
			fmt.Printf("Update existing leaf: key=%x, new value=%x\n", n.Key, value)
			if err := m.saveNode(n); err != nil {
				return nil, err
			}
			return n, nil
		}

		// 创建分支节点
		branch := &FullNode{
			NodeType: BranchNodeType,
			Children: [17]Node{},
			flags:    nodeFlag{},
		}

		// 将现有节点插入到分支节点
		if len(n.Key) > len(commonPrefix) {
			existingIdx := n.Key[len(commonPrefix)]
			existingLeafKey := n.Key[len(commonPrefix)+1:]
			existingLeaf := &LeafNode{NodeType: LeafNodeType, Key: existingLeafKey, Value: n.Value}
			fmt.Printf("Insert existing leaf: idx=%d, key=%x, value=%x\n", existingIdx, existingLeaf.Key, existingLeaf.Value)
			if err := m.saveNode(existingLeaf); err != nil {
				return nil, err
			}
			branch.Children[existingIdx] = existingLeaf
		}

		// 将新节点插入到分支节点
		if len(nibbles) > len(commonPrefix) {
			newIdx := nibbles[len(commonPrefix)]
			newLeafKey := nibbles[len(commonPrefix)+1:]
			newLeaf := &LeafNode{NodeType: LeafNodeType, Key: newLeafKey, Value: value}
			fmt.Printf("Insert new leaf: idx=%d, key=%x, value=%x\n", newIdx, newLeaf.Key, newLeaf.Value)
			if err := m.saveNode(newLeaf); err != nil {
				return nil, err
			}
			branch.Children[newIdx] = newLeaf
		}

		// 如果有公共前缀，创建扩展节点
		if len(commonPrefix) > 0 {
			if err := m.saveNode(branch); err != nil {
				return nil, err
			}
			ext := &ExtensionNode{
				NodeType: ExtensionNodeType,
				Path:     commonPrefix,
				Value:    branch.GetHash(),
				flags:    nodeFlag{},
			}
			fmt.Printf("Create extension node: path=%x, value=%x\n", ext.Path, ext.Value)
			if err := m.saveNode(ext); err != nil {
				return nil, err
			}
			return ext, nil
		}

		if err := m.saveNode(branch); err != nil {
			return nil, err
		}
		return branch, nil

	case *ExtensionNode:
		commonPrefix := findCommonPrefix(nibbles, n.Path)
		if len(commonPrefix) != len(n.Path) {
			// Create a branch node to split at the diverging point
			branch := &FullNode{
				NodeType: BranchNodeType,
				flags:    nodeFlag{},
			}

			// Add the existing extension node's path
			if len(n.Path[len(commonPrefix):]) > 0 {
				idx := n.Path[len(commonPrefix)]
				ext := &ExtensionNode{
					NodeType: ExtensionNodeType,
					Path:     n.Path[len(commonPrefix)+1:],
					Value:    n.Value,
					flags:    nodeFlag{},
				}
				if err := m.saveNode(ext); err != nil {
					return nil, err
				}
				branch.Children[idx] = ext
			}

			// Add the new path
			if len(nibbles[len(commonPrefix):]) > 0 {
				idx := nibbles[len(commonPrefix)]
				leaf := &LeafNode{
					NodeType: LeafNodeType,
					Key:      nibbles[len(commonPrefix)+1:],
					Value:    value,
					flags:    nodeFlag{},
				}
				if err := m.saveNode(leaf); err != nil {
					return nil, err
				}
				branch.Children[idx] = leaf
			}

			// Create extension node if needed
			if len(commonPrefix) > 0 {
				if err := m.saveNode(branch); err != nil {
					return nil, err
				}
				ext := &ExtensionNode{
					NodeType: ExtensionNodeType,
					Path:     commonPrefix,
					Value:    branch.GetHash(),
					flags:    nodeFlag{},
				}
				if err := m.saveNode(ext); err != nil {
					return nil, err
				}
				return ext, nil
			}

			if err := m.saveNode(branch); err != nil {
				return nil, err
			}
			return branch, nil
		}
		mn, err := m.LoadNode(n.Value)
		if err != nil {
			return nil, err
		}
		newChild, err := m.insert(mn, nibbles[len(commonPrefix):], value)
		if err != nil {
			return nil, err
		}
		n.Value = newChild.GetHash()
		if err := m.saveNode(n); err != nil {
			return nil, err
		}
		return n, nil

	case *FullNode:
		if len(nibbles) == 0 {
			var hash common.Hash
			hash.NewHash(value)
			n.Value = hash
			if err := m.saveNode(n); err != nil {
				return nil, err
			}
			return n, nil
		}

		idx := nibbles[0]
		child := n.Children[idx]
		if child == nil {
			// Create a new leaf node
			leaf := &LeafNode{
				NodeType: LeafNodeType,
				Key:      nibbles[1:],
				Value:    value,
				flags:    nodeFlag{},
			}
			if err := m.saveNode(leaf); err != nil {
				return nil, err
			}
			n.Children[idx] = leaf
		} else {
			// Insert into existing child
			newChild, err := m.insert(child, nibbles[1:], value)
			if err != nil {
				return nil, err
			}
			n.Children[idx] = newChild
		}
		if err := m.saveNode(n); err != nil {
			return nil, err
		}
		return n, nil

	default:
		return nil, fmt.Errorf("unknown node type")
	}
}

// get recursively retrieves a value from the trie
func (m *MPT) get(node Node, nibbles []byte) ([]byte, error) {
	switch n := node.(type) {
	case *LeafNode:
		fmt.Printf("LeafNode: key=%x, value=%x\n", n.Key, n.Value)
		// 如果叶子节点的 key 为空，说明这是一个分支节点的直接子节点，直接返回其 value
		if len(n.Key) == 0 {
			return n.Value, nil
		}
		// 如果查找的key长度小于叶子节点的key长度，或者key不匹配，说明key不存在
		if len(nibbles) < len(n.Key) || !bytes.Equal(n.Key, nibbles[:len(n.Key)]) {
			fmt.Printf("LeafNode mismatch: nibbles=%x, key=%x\n", nibbles, n.Key)
			return nil, fmt.Errorf("key not found")
		}
		// 如果查找的key长度与叶子节点的key长度不相等，说明不是完全匹配，key不存在
		if len(nibbles) != len(n.Key) {
			fmt.Printf("LeafNode length mismatch: nibbles=%x, key=%x\n", nibbles, n.Key)
			return nil, fmt.Errorf("key not found")
		}
		return n.Value, nil

	case *ExtensionNode:
		fmt.Printf("ExtensionNode: path=%x, value=%x\n", n.Path, n.Value)
		// 如果查找的key长度小于扩展节点的路径长度，或者路径不匹配，说明key不存在
		if len(nibbles) < len(n.Path) || !bytes.Equal(n.Path, nibbles[:len(n.Path)]) {
			fmt.Printf("ExtensionNode mismatch: nibbles=%x, path=%x\n", nibbles, n.Path)
			return nil, fmt.Errorf("key not found")
		}
		child, err := m.LoadNode(n.Value)
		if err != nil {
			return nil, err
		}
		return m.get(child, nibbles[len(n.Path):])

	case *FullNode:
		fmt.Printf("FullNode: nibbles=%x\n", nibbles)
		if len(nibbles) == 0 {
			return n.Value.Bytes(), nil
		}
		idx := nibbles[0]
		if idx >= 16 {
			return nil, fmt.Errorf("invalid nibble value: %d", idx)
		}
		child := n.Children[idx]
		if child == nil {
			fmt.Printf("FullNode: no child at index %d\n", idx)
			return nil, fmt.Errorf("key not found")
		}
		return m.get(child, nibbles[1:])

	default:
		return nil, fmt.Errorf("unknown node type")
	}
}

// delete recursively removes a key-value pair from the trie
func (m *MPT) delete(node Node, nibbles []byte) (Node, error) {
	switch n := node.(type) {
	case *LeafNode:
		// 如果键完全匹配，返回 nil 表示删除成功
		if bytes.Equal(n.Key, nibbles) {
			return nil, nil
		}
		// 如果键不匹配，返回原节点
		return n, nil

	case *ExtensionNode:
		if !hasPrefix(nibbles, n.Path) {
			return n, nil
		}
		child, err := m.LoadNode(n.Value)
		if err != nil {
			return nil, err
		}
		newChild, err := m.delete(child, nibbles[len(n.Path):])
		if err != nil {
			return nil, err
		}
		if newChild == nil {
			return nil, nil
		}
		n.Value = newChild.GetHash()
		if err := m.saveNode(n); err != nil {
			return nil, err
		}
		return n, nil

	case *FullNode:
		if len(nibbles) == 0 {
			n.Value = common.Hash{}
			if err := m.saveNode(n); err != nil {
				return nil, err
			}
			return n, nil
		}
		idx := nibbles[0]
		child := n.Children[idx]
		if child == nil {
			return n, nil
		}
		newChild, err := m.delete(child, nibbles[1:])
		if err != nil {
			return nil, err
		}
		if newChild == nil {
			n.Children[idx] = nil
			// 检查是否可以合并这个分支节点
			nonNilChildren := 0
			var lastChild Node
			for _, child := range n.Children {
				if child != nil {
					nonNilChildren++
					lastChild = child
				}
			}
			if nonNilChildren == 0 {
				return nil, nil
			}
			if nonNilChildren == 1 && n.Value == (common.Hash{}) {
				// 可以将这个分支节点合并为一个扩展节点
				return lastChild, nil
			}
		} else {
			n.Children[idx] = newChild
		}
		if err := m.saveNode(n); err != nil {
			return nil, err
		}
		return n, nil

	default:
		return nil, fmt.Errorf("unknown node type")
	}
}

// saveNode saves a node to the database
func (m *MPT) saveNode(node Node) error {
	// 先保存所有子节点
	if fullNode, ok := node.(*FullNode); ok {
		for _, child := range fullNode.Children {
			if child != nil {
				if err := m.saveNode(child); err != nil {
					return err
				}
			}
		}
	} else if extNode, ok := node.(*ExtensionNode); ok {
		// 对于 ExtensionNode，我们需要先加载子节点
		if extNode.Value != (common.Hash{}) {
			child, err := m.LoadNode(extNode.Value)
			if err != nil {
				return err
			}
			if err := m.saveNode(child); err != nil {
				return err
			}
		}
	}

	// 序列化节点
	data, err := node.Serialize()
	if err != nil {
		return err
	}

	// 保存到数据库
	return m.DB.Put(node.GetHash().Bytes(), data)
}

// keyToNibbles converts a byte slice to nibbles (hex)
func keyToNibbles(key []byte) []byte {
	nibbles := make([]byte, len(key)*2)
	for i, b := range key {
		nibbles[i*2] = b >> 4
		nibbles[i*2+1] = b & 0x0f
	}
	return nibbles
}

// findCommonPrefix finds the common prefix of two byte slices
func findCommonPrefix(a, b []byte) []byte {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return a[:i]
}

// hasPrefix checks if a byte slice has a given prefix
func hasPrefix(s, prefix []byte) bool {
	if len(s) < len(prefix) || len(prefix) == 0 {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}
