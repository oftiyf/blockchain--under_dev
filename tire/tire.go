package tire

type Trie interface {
	Insert(key, value []byte) error

	Search(key []byte) ([]byte, error)

	Root() ([]byte, error)
	
	
}

