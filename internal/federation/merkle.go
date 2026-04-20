package federation

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
)

const (
	// LeafDomain is the domain separator for Merkle leaf hashes.
	LeafDomain = "VK:MERKLE:LEAF:v1"
	// NodeDomain is the domain separator for internal Merkle node hashes.
	NodeDomain = "VK:MERKLE:NODE:v1"
)

// ProofStep is one step in a Merkle inclusion proof.
type ProofStep struct {
	SiblingHash []byte `json:"sibling_hash"`
	// Position indicates where the sibling sits relative to the
	// hash being verified: "left" means sibling is on the left.
	Position string `json:"position"`
}

// MerkleTree holds a binary Merkle tree built from a set of leaf hashes.
type MerkleTree struct {
	Root   []byte
	Leaves [][]byte
	// layers stores every layer bottom-up: layers[0] = leaves,
	// layers[len-1] = [root].
	layers [][]byte
	// layerSizes records the number of nodes at each layer so we
	// can navigate the flat layers slice.
	layerSizes []int
}

// BuildMerkleTree constructs a binary Merkle tree from pre-computed
// leaf hashes. Returns an error if leaves is empty.
//
// Odd leaves are duplicated as right siblings at each layer.
// Internal node hash: SHA-256("VK:MERKLE:NODE:v1" || left || right).
func BuildMerkleTree(leaves [][]byte) (*MerkleTree, error) {
	if len(leaves) == 0 {
		return nil, errors.New("cannot build merkle tree from zero leaves")
	}

	// Copy leaves so the caller's slice is not mutated.
	current := make([][]byte, len(leaves))
	copy(current, leaves)

	allLayers := [][]byte{}
	layerSizes := []int{}

	// Record leaf layer.
	allLayers = append(allLayers, current...)
	layerSizes = append(layerSizes, len(current))

	for len(current) > 1 {
		next := make([][]byte, 0, (len(current)+1)/2)
		for i := 0; i < len(current); i += 2 {
			left := current[i]
			right := left // duplicate if odd
			if i+1 < len(current) {
				right = current[i+1]
			}
			next = append(next, nodeHash(left, right))
		}
		allLayers = append(allLayers, next...)
		layerSizes = append(layerSizes, len(next))
		current = next
	}

	return &MerkleTree{
		Root:       current[0],
		Leaves:     leaves,
		layers:     allLayers,
		layerSizes: layerSizes,
	}, nil
}

// Proof generates a Merkle inclusion proof for the leaf at the given
// index. The proof is a path from the leaf to the root.
func (t *MerkleTree) Proof(index int) ([]ProofStep, error) {
	if index < 0 || index >= len(t.Leaves) {
		return nil, fmt.Errorf("leaf index %d out of range [0, %d)", index, len(t.Leaves))
	}

	var proof []ProofStep
	idx := index

	layerOffset := 0
	for li := 0; li < len(t.layerSizes)-1; li++ {
		layerLen := t.layerSizes[li]

		var siblingIdx int
		var pos string
		if idx%2 == 0 {
			siblingIdx = idx + 1
			if siblingIdx >= layerLen {
				siblingIdx = idx // odd leaf duplicated
			}
			pos = "right"
		} else {
			siblingIdx = idx - 1
			pos = "left"
		}

		proof = append(proof, ProofStep{
			SiblingHash: t.layers[layerOffset+siblingIdx],
			Position:    pos,
		})

		layerOffset += layerLen
		idx /= 2
	}

	return proof, nil
}

// VerifyProof checks that a leaf hash, combined with the inclusion
// proof, produces the expected root. Uses constant-time comparison.
func VerifyProof(leaf []byte, proof []ProofStep, expectedRoot []byte) bool {
	current := leaf
	for _, step := range proof {
		if step.Position == "left" {
			current = nodeHash(step.SiblingHash, current)
		} else {
			current = nodeHash(current, step.SiblingHash)
		}
	}
	return subtle.ConstantTimeCompare(current, expectedRoot) == 1
}

// nodeHash computes SHA-256("VK:MERKLE:NODE:v1" || left || right).
func nodeHash(left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte(NodeDomain))
	h.Write(left)
	h.Write(right)
	sum := h.Sum(nil)
	return sum
}
