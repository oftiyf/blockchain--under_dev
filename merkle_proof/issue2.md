# 🚀 Proposal: Bring Hash Diversity to the MPT – Add zk-friendly Hash Option

> _"现在性别都那么多了，MPT 的哈希怎么还只能选 Keccak？"_

### Background

Ethereum’s Merkle Patricia Trie (MPT) has long relied on a single, noble hash function: **Keccak256**. It’s secure, robust, and battle-tested.

But in the zk world? It’s a **constraint-heavy nightmare**.

As a zk circuit developer working with [gnark](https://github.com/consensys/gnark), I’m building a zero-knowledge proof that validates MPT path modifications. But when every node hash requires thousands of constraints just to implement Keccak, my prover cries, my CPU screams, and my deadlines burn.

### Proposal

**Let’s add an *optional*, zk-friendly hash function to the MPT implementation.**

Specifically:

- Keep Keccak256 as the default (obviously)
- Allow opt-in usage of something like **MiMC** or **Poseidon** (both are zk-circuit friendly)
- The hash function could be selected via:
  - A compile-time build tag (e.g., `--tags zk_mpt`)
  - Or a `HashFunc` strategy interface injected into trie construction

This doesn’t affect consensus. It’s purely for clients or tooling that work with zk circuits, e.g., L2s, light clients, bridges, etc.

### Real World Use Case

I’m implementing a zk circuit that verifies whether an MPT update was valid **without requiring all node hashes to be public**. Instead, the verifier can provide just one known intermediate hash, and the prover walks up the modified path using zk-friendly hashing.

Example (simplified):

```go
for each node in ModifiedPath {
    if node.Level changes {
        flush hash stack;
        hash += currentNode;
    }
    check hash == KnownHash;
}
assert final hash == Root;
```

This works beautifully with MiMC, but is practically **impossible** with Keccak in-circuit.

### Why This Matters

- **Lowers proving cost** for zkRollups and zkBridges
- Makes **on-chain zk state verification** practical
- Adds **hash diversity** to the protocol (inclusive computing ftw ✊)
- Could help projects like mine get shipped on time 😉

### In Summary

Keccak will always be the king 👑. But even kings need sidekicks.

Let’s give the MPT a little more love from the zk world. One hash doesn’t fit all – and it’s time we hash a bit more kindly.

---

Happy to PR something minimal if the idea has legs. Let me know what y’all think!


---

### 🛠 Design Example (From My Current zk Circuit)

Here's a real snippet from the zk circuit I'm building using [gnark](https://github.com/consensys/gnark) + MiMC.
The idea is to recompute the modified path from the leaf node upward, only using MiMC hashes. The prover doesn't need to expose every node hash — just one known point on the path.

```go
package main

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

// My idea is to encode like this: continuously hashsum until encountering a ZKNode with a level different from the current hash,
// implement the previous hash output, and hashsum the result with the current ZKNode.
// Then, compare this ZKNode's level with the current level. If the levels are the same, continue hashsumming
// until encountering a ZKNode with a level different from the current hash, implement the previous hash output,
// and hashsum the result with the current ZKNode.
type ZKNode struct {
	NodeType     frontend.Variable    // 0=Leaf, 1=Extension, 2=Branch
	PathFragment [8]frontend.Variable // path used by extension or leaf, fixed length 8
	Value        frontend.Variable    // only used by leaf nodes

	Level frontend.Variable
	// For Branch nodes: Children[0..15]
	// For Leaf/Extension nodes: empty
	Children [16]frontend.Variable // 16 children for branch nodes (fill with empty variables if not present)
}

type MTCircuit struct {
	KnownHash    frontend.Variable `gnark:",public"`
	Root         frontend.Variable `gnark:",public"`
	PathLength   frontend.Variable `gnark:",public"`
	ModifiedPath [32]ZKNode        // changed to fixed length array
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

	// write path fragment
	for i := 0; i < 8; i++ {
		h.Write(node.PathFragment[i])
	}

	// write value (if it's a leaf node)
	h.Write(node.Value)

	// write child node hash
	h.Write(childHash)

	// write child node array
	for i := 0; i
...
```

This circuit structure enables validating state modifications *without* having to expose the entire MPT path, and it works because MiMC keeps the constraint count small.

With Keccak? This design would go up in constraint flames 🔥.

