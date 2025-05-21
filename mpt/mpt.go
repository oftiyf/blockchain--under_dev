package mpt

import (
	"blockchain/common"
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

	// If root is nil, create a new leaf node
	if m.Root == nil {
		var hash common.Hash
		hash.NewHash(value)
		m.Root = &LeafNode{
			Key:      nibbles,
			Value:    hash,
			NodeType: LeafNodeType,
		}
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
	return m.saveNode(m.Root)
}

// insert recursively inserts a key-value pair into the trie
func (m *MPT) insert(node Node, nibbles []byte, value []byte) (Node, error) {
	switch n := node.(type) {
	case *LeafNode:
		// If this is a leaf node, we need to handle the collision
		commonPrefix := findCommonPrefix(n.Key, nibbles)
		if len(commonPrefix) == len(n.Key) && len(commonPrefix) == len(nibbles) {
			// Same key, update value
			var hash common.Hash
			hash.NewHash(value)
			n.Value = hash
			return n, nil
		}

		// Create a new branch node
		branch := &BranchNode{
			Children: [16]Node{},
			NodeType: BranchNodeType,
		}

		// Insert the existing leaf node
		if len(n.Key) > len(commonPrefix) {
			idx := n.Key[len(commonPrefix)]
			branch.Children[idx] = &LeafNode{
				Key:      n.Key[len(commonPrefix)+1:],
				Value:    n.Value,
				NodeType: LeafNodeType,
			}
		}

		// Insert the new value
		if len(nibbles) > len(commonPrefix) {
			idx := nibbles[len(commonPrefix)]
			var hash common.Hash
			hash.NewHash(value)
			branch.Children[idx] = &LeafNode{
				Key:      nibbles[len(commonPrefix)+1:],
				Value:    hash,
				NodeType: LeafNodeType,
			}
		}

		// If there's a common prefix, wrap the branch in an extension node
		if len(commonPrefix) > 0 {
			return &ExtensionNode{
				Path:     commonPrefix,
				Value:    branch.GetHash(),
				NodeType: ExtensionNodeType,
			}, nil
		}

		return branch, nil

	case *ExtensionNode:
		// If this is an extension node, we need to handle the path
		commonPrefix := findCommonPrefix(n.Path, nibbles)
		if len(commonPrefix) == len(n.Path) {
			// The path matches, continue down the trie
			child, err := m.LoadNode(n.Value)
			if err != nil {
				return nil, err
			}
			newChild, err := m.insert(child, nibbles[len(commonPrefix):], value)
			if err != nil {
				return nil, err
			}
			n.Value = newChild.GetHash()
			return n, nil
		}

		// Create a new branch node
		branch := &BranchNode{
			Children: [16]Node{},
			NodeType: BranchNodeType,
		}

		// Insert the existing extension node
		if len(n.Path) > len(commonPrefix) {
			idx := n.Path[len(commonPrefix)]
			branch.Children[idx] = &ExtensionNode{
				Path:     n.Path[len(commonPrefix)+1:],
				Value:    n.Value,
				NodeType: ExtensionNodeType,
			}
		}

		// Insert the new value
		if len(nibbles) > len(commonPrefix) {
			idx := nibbles[len(commonPrefix)]
			var hash common.Hash
			hash.NewHash(value)
			branch.Children[idx] = &LeafNode{
				Key:      nibbles[len(commonPrefix)+1:],
				Value:    hash,
				NodeType: LeafNodeType,
			}
		}

		// If there's a common prefix, wrap the branch in an extension node
		if len(commonPrefix) > 0 {
			return &ExtensionNode{
				Path:     commonPrefix,
				Value:    branch.GetHash(),
				NodeType: ExtensionNodeType,
			}, nil
		}

		return branch, nil

	case *BranchNode:
		// If this is a branch node, we need to handle the children
		if len(nibbles) == 0 {
			var hash common.Hash
			hash.NewHash(value)
			n.Value = hash
			return n, nil
		}

		idx := nibbles[0]
		child := n.Children[idx]
		if child == nil {
			// Create a new leaf node
			var hash common.Hash
			hash.NewHash(value)
			n.Children[idx] = &LeafNode{
				Key:      nibbles[1:],
				Value:    hash,
				NodeType: LeafNodeType,
			}
		} else {
			// Insert into existing child
			newChild, err := m.insert(child, nibbles[1:], value)
			if err != nil {
				return nil, err
			}
			n.Children[idx] = newChild
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
		if len(nibbles) == 0 {
			return n.Value.Bytes(), nil
		}
		return nil, fmt.Errorf("key not found")

	case *ExtensionNode:
		if !hasPrefix(nibbles, n.Path) {
			return nil, fmt.Errorf("key not found")
		}
		child, err := m.LoadNode(n.Value)
		if err != nil {
			return nil, err
		}
		return m.get(child, nibbles[len(n.Path):])

	case *BranchNode:
		if len(nibbles) == 0 {
			return n.Value.Bytes(), nil
		}
		idx := nibbles[0]
		child := n.Children[idx]
		if child == nil {
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
		if len(nibbles) == 0 {
			return nil, nil
		}
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
		return n, nil

	case *BranchNode:
		if len(nibbles) == 0 {
			n.Value = common.Hash{}
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
			// Check if we can collapse this branch node
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
				// We can collapse this branch node into an extension node
				return lastChild, nil
			}
		} else {
			n.Children[idx] = newChild
		}
		return n, nil

	default:
		return nil, fmt.Errorf("unknown node type")
	}
}

// saveNode saves a node to the database
func (m *MPT) saveNode(node Node) error {
	data, err := node.Serialize()
	if err != nil {
		return err
	}
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
