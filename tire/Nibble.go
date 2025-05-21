package tire

// Nibble represents a single nibble (4 bits)
type Nibble byte

// Nibbles represents a slice of nibbles
type Nibbles []Nibble

// ToNibbles converts a byte slice to nibbles
func ToNibbles(key []byte) Nibbles {
	nibbles := make(Nibbles, len(key)*2)
	for i, b := range key {
		nibbles[i*2] = Nibble(b >> 4)
		nibbles[i*2+1] = Nibble(b & 0x0f)
	}
	return nibbles
}

// ToBytes converts nibbles back to bytes
func (n Nibbles) ToBytes() []byte {
	if len(n)%2 != 0 {
		panic("nibbles length must be even")
	}
	bytes := make([]byte, len(n)/2)
	for i := 0; i < len(n); i += 2 {
		bytes[i/2] = byte(n[i]<<4 | n[i+1])
	}
	return bytes
}

// FindCommonPrefix finds the common prefix of two nibble slices
func FindCommonPrefix(a, b Nibbles) Nibbles {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return a[:i]
}

// HasPrefix checks if a nibble slice has a given prefix
func HasPrefix(s, prefix Nibbles) bool {
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

// CompactEncode encodes nibbles with a flag
func CompactEncode(nibbles Nibbles) []byte {
	// Add flag byte
	flag := byte(0)
	if len(nibbles)%2 == 1 {
		flag = 1
	}

	// Create result slice
	result := make([]byte, len(nibbles)/2+1)
	result[0] = flag

	// Encode nibbles
	for i := 0; i < len(nibbles); i += 2 {
		if i+1 < len(nibbles) {
			result[i/2+1] = byte(nibbles[i]<<4 | nibbles[i+1])
		} else {
			result[i/2+1] = byte(nibbles[i] << 4)
		}
	}

	return result
}

// CompactDecode decodes bytes back to nibbles
func CompactDecode(data []byte) Nibbles {
	if len(data) == 0 {
		return nil
	}

	// Get flag
	flag := data[0]
	isOdd := flag == 1

	// Create result slice
	nibbles := make(Nibbles, len(data)*2-1)
	if isOdd {
		nibbles = make(Nibbles, len(data)*2-2)
	}

	// Decode bytes
	for i := 1; i < len(data); i++ {
		nibbles[(i-1)*2] = Nibble(data[i] >> 4)
		if !isOdd || i < len(data)-1 {
			nibbles[(i-1)*2+1] = Nibble(data[i] & 0x0f)
		}
	}

	return nibbles
}

// IsTerminal checks if the nibbles represent a terminal node
func IsTerminal(nibbles Nibbles) bool {
	return len(nibbles) > 0 && nibbles[0] == 0x10
}

// AddTerminalFlag adds the terminal flag to nibbles
func AddTerminalFlag(nibbles Nibbles) Nibbles {
	result := make(Nibbles, len(nibbles)+1)
	result[0] = 0x10
	copy(result[1:], nibbles)
	return result
}

// RemoveTerminalFlag removes the terminal flag from nibbles
func RemoveTerminalFlag(nibbles Nibbles) Nibbles {
	if len(nibbles) == 0 || nibbles[0] != 0x10 {
		return nibbles
	}
	return nibbles[1:]
}
