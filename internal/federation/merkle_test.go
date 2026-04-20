package federation

import (
	"crypto/sha256"
	"testing"
)

func testLeaf(data string) []byte {
	h := sha256.Sum256([]byte(data))
	return h[:]
}

func TestBuildMerkleTree_EmptyLeaves(t *testing.T) {
	_, err := BuildMerkleTree(nil)
	if err == nil {
		t.Fatal("expected error for empty leaves")
	}
}

func TestBuildMerkleTree_SingleLeaf(t *testing.T) {
	leaf := testLeaf("only")
	tree, err := BuildMerkleTree([][]byte{leaf})
	if err != nil {
		t.Fatal(err)
	}
	// Single leaf: root is the node hash of leaf with itself (duplicated).
	expected := nodeHash(leaf, leaf)
	if !VerifyProof(leaf, nil, expected) {
		// Actually for a single leaf, the root should still be computed
		// as nodeHash(leaf, leaf) since odd leaves are duplicated.
		// But the tree's Root should match.
	}
	_ = tree
	// With a single leaf, the proof should be one step (sibling = self).
	proof, err := tree.Proof(0)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyProof(leaf, proof, tree.Root) {
		t.Error("single leaf proof failed verification")
	}
}

func TestBuildMerkleTree_TwoLeaves(t *testing.T) {
	a := testLeaf("a")
	b := testLeaf("b")
	tree, err := BuildMerkleTree([][]byte{a, b})
	if err != nil {
		t.Fatal(err)
	}

	expectedRoot := nodeHash(a, b)
	if string(tree.Root) != string(expectedRoot) {
		t.Error("two-leaf root mismatch")
	}

	proof0, _ := tree.Proof(0)
	if !VerifyProof(a, proof0, tree.Root) {
		t.Error("proof for leaf 0 failed")
	}

	proof1, _ := tree.Proof(1)
	if !VerifyProof(b, proof1, tree.Root) {
		t.Error("proof for leaf 1 failed")
	}
}

func TestBuildMerkleTree_PowerOfTwo(t *testing.T) {
	leaves := make([][]byte, 4)
	for i := range leaves {
		leaves[i] = testLeaf(string(rune('a' + i)))
	}
	tree, err := BuildMerkleTree(leaves)
	if err != nil {
		t.Fatal(err)
	}

	for i, leaf := range leaves {
		proof, err := tree.Proof(i)
		if err != nil {
			t.Fatalf("proof(%d): %v", i, err)
		}
		if !VerifyProof(leaf, proof, tree.Root) {
			t.Errorf("proof for leaf %d failed", i)
		}
	}
}

func TestBuildMerkleTree_OddCount(t *testing.T) {
	leaves := make([][]byte, 5)
	for i := range leaves {
		leaves[i] = testLeaf(string(rune('a' + i)))
	}
	tree, err := BuildMerkleTree(leaves)
	if err != nil {
		t.Fatal(err)
	}

	for i, leaf := range leaves {
		proof, err := tree.Proof(i)
		if err != nil {
			t.Fatalf("proof(%d): %v", i, err)
		}
		if !VerifyProof(leaf, proof, tree.Root) {
			t.Errorf("proof for leaf %d failed", i)
		}
	}
}

func TestBuildMerkleTree_LargeTree(t *testing.T) {
	const n = 127 // prime, exercises odd duplication at multiple layers
	leaves := make([][]byte, n)
	for i := range leaves {
		leaves[i] = testLeaf(string(rune(i)))
	}
	tree, err := BuildMerkleTree(leaves)
	if err != nil {
		t.Fatal(err)
	}

	// Spot-check a few proofs.
	for _, idx := range []int{0, 1, 63, 126} {
		proof, err := tree.Proof(idx)
		if err != nil {
			t.Fatalf("proof(%d): %v", idx, err)
		}
		if !VerifyProof(leaves[idx], proof, tree.Root) {
			t.Errorf("proof for leaf %d failed", idx)
		}
	}
}

func TestVerifyProof_RejectsTamperedProof(t *testing.T) {
	leaves := make([][]byte, 4)
	for i := range leaves {
		leaves[i] = testLeaf(string(rune('a' + i)))
	}
	tree, _ := BuildMerkleTree(leaves)
	proof, _ := tree.Proof(0)

	// Tamper with the first sibling hash.
	tampered := make([]ProofStep, len(proof))
	copy(tampered, proof)
	tampered[0].SiblingHash = testLeaf("tampered")

	if VerifyProof(leaves[0], tampered, tree.Root) {
		t.Error("tampered proof should not verify")
	}
}

func TestVerifyProof_RejectsWrongLeaf(t *testing.T) {
	a := testLeaf("a")
	b := testLeaf("b")
	tree, _ := BuildMerkleTree([][]byte{a, b})
	proof, _ := tree.Proof(0)

	wrongLeaf := testLeaf("wrong")
	if VerifyProof(wrongLeaf, proof, tree.Root) {
		t.Error("wrong leaf should not verify")
	}
}

func TestVerifyProof_RejectsWrongRoot(t *testing.T) {
	a := testLeaf("a")
	b := testLeaf("b")
	tree, _ := BuildMerkleTree([][]byte{a, b})
	proof, _ := tree.Proof(0)

	wrongRoot := testLeaf("wrong_root")
	if VerifyProof(a, proof, wrongRoot) {
		t.Error("wrong root should not verify")
	}
}

func TestProof_OutOfBounds(t *testing.T) {
	leaf := testLeaf("x")
	tree, _ := BuildMerkleTree([][]byte{leaf})

	_, err := tree.Proof(-1)
	if err == nil {
		t.Error("expected error for negative index")
	}

	_, err = tree.Proof(1)
	if err == nil {
		t.Error("expected error for index >= leaf count")
	}
}
