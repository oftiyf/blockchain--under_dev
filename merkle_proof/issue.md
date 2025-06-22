# 🚀 Proposal: Bring Hash Diversity to the MPT – Add zk-friendly Hash Option

> _"There are so many genders nowadays, but the MPT still only accepts one hash function — Keccak?"_

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
