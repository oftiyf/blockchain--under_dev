package mpt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Node 表示 MPT 树中的节点
type Node struct {
	Type     string           `json:"type"`     // 节点类型：branch, extension, leaf
	Value    []byte           `json:"value"`    // 节点值
	Children map[string]*Node `json:"children"` // 分支节点的子节点
	Path     []byte           `json:"path"`     // 扩展节点或叶子节点的路径
	IsLeaf   bool             `json:"isLeaf"`   // 是否是叶子节点
}

// MPT 表示 Merkle Patricia Trie
type MPT struct {
	Root *Node
	DB   *DB
}

// NewMPT 创建一个新的 MPT
func NewMPT(db *DB) *MPT {
	mpt := &MPT{
		Root: &Node{
			Type:     "branch",
			Children: make(map[string]*Node),
		},
		DB: db,
	}

	// 尝试从数据库加载已存在的树结构
	data, err := db.Get([]byte("mpt_root"))
	if err == nil && len(data) > 0 {
		var root Node
		if err := json.Unmarshal(data, &root); err == nil {
			mpt.Root = &root
			fmt.Printf("从数据库加载 MPT 成功\n")
		} else {
			fmt.Printf("从数据库加载 MPT 失败: %v\n", err)
		}
	} else {
		fmt.Printf("数据库中没有找到 MPT 数据\n")
	}
	return mpt
}

// Put 向 MPT 中插入键值对
func (mpt *MPT) Put(key, value []byte) error {
	fmt.Printf("[PUT] 插入键: %s, 值: %s\n", string(key), string(value))
	// 将 key 转换为十六进制字符串
	keyHex := fmt.Sprintf("%x", key)
	mpt.Root = mpt.put(mpt.Root, keyHex, value)
	return mpt.saveToDB()
}

// put 递归插入节点
func (mpt *MPT) put(node *Node, key string, value []byte) *Node {
	if node == nil {
		fmt.Printf("[PUT] 新建 leaf 节点, key: %s\n", key)
		// 创建新的叶子节点
		return &Node{
			Type:   "leaf",
			Value:  value,
			Path:   []byte(key),
			IsLeaf: true,
		}
	}

	switch node.Type {
	case "branch":
		if key == "" {
			// 如果 key 为空，更新当前分支节点的值
			node.Value = value
			return node
		}
		// 获取第一个字符作为子节点索引
		index := key[:1]
		if node.Children == nil {
			node.Children = make(map[string]*Node)
		}
		if node.Children[index] == nil {
			fmt.Printf("[PUT] 新建 branch 子节点, index: %s\n", index)
			node.Children[index] = &Node{Type: "branch", Children: make(map[string]*Node)}
		}
		node.Children[index] = mpt.put(node.Children[index], key[1:], value)
		return node

	case "leaf", "extension":
		// 找到共同前缀
		commonPrefix := findCommonPrefix(string(node.Path), key)
		if commonPrefix == "" {
			// 没有共同前缀，创建新的分支节点
			branch := &Node{
				Type:     "branch",
				Children: make(map[string]*Node),
			}
			// 将当前节点和新的叶子节点添加到分支节点
			if len(node.Path) > 0 {
				branch.Children[string(node.Path[0])] = &Node{
					Type:     node.Type,
					Value:    node.Value,
					Path:     node.Path[1:],
					IsLeaf:   node.IsLeaf,
					Children: node.Children,
				}
			}
			if len(key) > 0 {
				branch.Children[key[:1]] = &Node{
					Type:   "leaf",
					Value:  value,
					Path:   []byte(key[1:]),
					IsLeaf: true,
				}
			}
			return branch
		}

		// 有共同前缀，创建扩展节点
		extension := &Node{
			Type:     "extension",
			Path:     []byte(commonPrefix),
			Children: make(map[string]*Node),
		}
		// 创建新的分支节点
		branch := &Node{
			Type:     "branch",
			Children: make(map[string]*Node),
		}
		// 将剩余部分添加到分支节点
		if len(node.Path) > len(commonPrefix) {
			branch.Children[string(node.Path[len(commonPrefix)])] = &Node{
				Type:     node.Type,
				Value:    node.Value,
				Path:     node.Path[len(commonPrefix)+1:],
				IsLeaf:   node.IsLeaf,
				Children: node.Children,
			}
		}
		if len(key) > len(commonPrefix) {
			branch.Children[key[len(commonPrefix):len(commonPrefix)+1]] = &Node{
				Type:   "leaf",
				Value:  value,
				Path:   []byte(key[len(commonPrefix)+1:]),
				IsLeaf: true,
			}
		}
		// 将分支节点作为扩展节点的子节点
		extension.Children[""] = branch
		return extension
	}

	fmt.Printf("[PUT] 未知类型节点: %+v\n", node)
	return node
}

// Get 从 MPT 中获取值
func (mpt *MPT) Get(key []byte) ([]byte, error) {
	// 将 key 转换为十六进制字符串
	keyHex := fmt.Sprintf("%x", key)
	fmt.Printf("[GET] 查询键: %s (hex: %s)\n", string(key), keyHex)
	return mpt.get(mpt.Root, keyHex)
}

// get 递归获取值
func (mpt *MPT) get(node *Node, key string) ([]byte, error) {
	if node == nil {
		fmt.Printf("[GET] node == nil\n")
		return nil, fmt.Errorf("key not found")
	}
	fmt.Printf("[GET] 当前节点类型: %s, key: %s, node: %+v\n", node.Type, key, node)
	switch node.Type {
	case "branch":
		if key == "" {
			return node.Value, nil
		}
		if len(key) == 0 {
			return nil, fmt.Errorf("key not found")
		}
		child := node.Children[key[:1]]
		if child == nil {
			fmt.Printf("[GET] branch 子节点不存在, index: %s\n", key[:1])
			return nil, fmt.Errorf("key not found")
		}
		return mpt.get(child, key[1:])

	case "leaf":
		if string(node.Path) == key {
			return node.Value, nil
		}
		fmt.Printf("[GET] leaf 路径不匹配, node.Path: %s, key: %s\n", string(node.Path), key)
		return nil, fmt.Errorf("key not found")

	case "extension":
		if !hasPrefix(key, string(node.Path)) {
			fmt.Printf("[GET] extension 路径不匹配, node.Path: %s, key: %s\n", string(node.Path), key)
			return nil, fmt.Errorf("key not found")
		}
		// 修改：递归查询子节点，而不是当前节点
		remainingKey := key[len(node.Path):]
		// 获取子节点（分支节点）
		child := node.Children[""]
		if child == nil {
			fmt.Printf("[GET] extension 子节点不存在\n")
			return nil, fmt.Errorf("key not found")
		}
		return mpt.get(child, remainingKey)
	}

	fmt.Printf("[GET] invalid node type: %+v\n", node)
	return nil, fmt.Errorf("invalid node type")
}

// Delete 从 MPT 中删除键值对
func (mpt *MPT) Delete(key []byte) error {
	keyHex := fmt.Sprintf("%x", key)
	mpt.Root = mpt.delete(mpt.Root, keyHex)
	return mpt.saveToDB()
}

// delete 递归删除节点
func (mpt *MPT) delete(node *Node, key string) *Node {
	if node == nil {
		return nil
	}

	switch node.Type {
	case "branch":
		if key == "" {
			node.Value = nil
			// 如果分支节点没有子节点，返回 nil
			if len(node.Children) == 0 {
				return nil
			}
			return node
		}
		index := key[:1]
		if child, ok := node.Children[index]; ok {
			node.Children[index] = mpt.delete(child, key[1:])
			// 如果子节点被删除，从 map 中移除
			if node.Children[index] == nil {
				delete(node.Children, index)
			}
		}
		// 如果分支节点没有子节点且没有值，返回 nil
		if len(node.Children) == 0 && node.Value == nil {
			return nil
		}
		return node

	case "leaf":
		if string(node.Path) == key {
			return nil
		}
		return node

	case "extension":
		if !hasPrefix(key, string(node.Path)) {
			return node
		}
		remainingKey := key[len(node.Path):]
		child := node.Children[""]
		if child != nil {
			node.Children[""] = mpt.delete(child, remainingKey)
			// 如果子节点被删除，返回 nil
			if node.Children[""] == nil {
				return nil
			}
		}
		return node
	}

	return node
}

// Has 检查键是否存在
func (mpt *MPT) Has(key []byte) bool {
	_, err := mpt.Get(key)
	return err == nil
}

// GetAll 获取所有键值对
func (mpt *MPT) GetAll() map[string][]byte {
	result := make(map[string][]byte)
	mpt.getAll(mpt.Root, "", result)
	return result
}

// getAll 递归获取所有键值对
func (mpt *MPT) getAll(node *Node, prefix string, result map[string][]byte) {
	if node == nil {
		return
	}

	switch node.Type {
	case "branch":
		if node.Value != nil {
			result[prefix] = node.Value
		}
		for k, child := range node.Children {
			mpt.getAll(child, prefix+k, result)
		}

	case "leaf":
		if node.IsLeaf {
			result[prefix+string(node.Path)] = node.Value
		}

	case "extension":
		for k, child := range node.Children {
			mpt.getAll(child, prefix+string(node.Path)+k, result)
		}
	}
}

// PrintTree 打印树结构
func (mpt *MPT) PrintTree() {
	fmt.Println("MPT Tree Structure:")
	mpt.printNode(mpt.Root, 0)
}

// printNode 递归打印节点
func (mpt *MPT) printNode(node *Node, depth int) {
	if node == nil {
		return
	}

	indent := strings.Repeat("  ", depth)
	fmt.Printf("%sType: %s\n", indent, node.Type)
	if node.Value != nil {
		fmt.Printf("%sValue: %s\n", indent, string(node.Value))
	}
	if node.Path != nil {
		fmt.Printf("%sPath: %s\n", indent, string(node.Path))
	}
	fmt.Printf("%sIsLeaf: %v\n", indent, node.IsLeaf)

	if node.Children != nil {
		fmt.Printf("%sChildren:\n", indent)
		for k, child := range node.Children {
			fmt.Printf("%s  Key: %s\n", indent, k)
			mpt.printNode(child, depth+2)
		}
	}
}

// saveToDB 将 MPT 保存到数据库
func (mpt *MPT) saveToDB() error {
	data, err := json.Marshal(mpt.Root)
	if err != nil {
		return fmt.Errorf("序列化 MPT 失败: %v", err)
	}
	fmt.Printf("保存 MPT 到数据库，数据大小: %d 字节\n", len(data))
	if err := mpt.DB.Put([]byte("mpt_root"), data); err != nil {
		return fmt.Errorf("保存 MPT 到数据库失败: %v", err)
	}
	return nil
}

// findCommonPrefix 找到两个字符串的共同前缀
func findCommonPrefix(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return a[:i]
}

// hasPrefix 检查字符串是否以指定前缀开头
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
