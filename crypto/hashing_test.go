package crypto

import "testing"

func TestHashBytesDeterministic(t *testing.T) {
	a := HashBytes([]byte("hello"))
	b := HashBytes([]byte("hello"))
	if a != b {
		t.Fatalf("expected identical hashes, got %q and %q", a, b)
	}
	if len(a) != HashSize*2 { // hex-encoded
		t.Fatalf("expected %d hex chars, got %d", HashSize*2, len(a))
	}
}

func TestComputeRootHashOrderIndependent(t *testing.T) {
	hashes := []ResourceHash{
		{ResourceID: "ec2:i-1", Hash: HashBytes([]byte("config-1"))},
		{ResourceID: "vpc:v-1", Hash: HashBytes([]byte("config-2"))},
		{ResourceID: "sg:sg-1", Hash: HashBytes([]byte("config-3"))},
	}
	reordered := []ResourceHash{hashes[2], hashes[0], hashes[1]}

	root1, err := ComputeRootHash(hashes)
	if err != nil {
		t.Fatalf("ComputeRootHash: %v", err)
	}
	root2, err := ComputeRootHash(reordered)
	if err != nil {
		t.Fatalf("ComputeRootHash: %v", err)
	}
	if root1 != root2 {
		t.Fatal("expected root hash to be independent of input order")
	}
}

func TestComputeRootHashChangesOnTamper(t *testing.T) {
	original := []ResourceHash{
		{ResourceID: "ec2:i-1", Hash: HashBytes([]byte("config-1"))},
		{ResourceID: "vpc:v-1", Hash: HashBytes([]byte("config-2"))},
	}
	tampered := []ResourceHash{
		{ResourceID: "ec2:i-1", Hash: HashBytes([]byte("config-1-modified"))},
		{ResourceID: "vpc:v-1", Hash: HashBytes([]byte("config-2"))},
	}

	rootOrig, err := ComputeRootHash(original)
	if err != nil {
		t.Fatalf("ComputeRootHash: %v", err)
	}
	rootTampered, err := ComputeRootHash(tampered)
	if err != nil {
		t.Fatalf("ComputeRootHash: %v", err)
	}
	if rootOrig == rootTampered {
		t.Fatal("expected root hash to change when a resource's content hash changes")
	}
}

func TestComputeRootHashDetectsAddedOrRemovedResource(t *testing.T) {
	base := []ResourceHash{
		{ResourceID: "ec2:i-1", Hash: HashBytes([]byte("config-1"))},
	}
	withExtra := []ResourceHash{
		{ResourceID: "ec2:i-1", Hash: HashBytes([]byte("config-1"))},
		{ResourceID: "ec2:i-2", Hash: HashBytes([]byte("config-2"))},
	}

	rootBase, _ := ComputeRootHash(base)
	rootExtra, _ := ComputeRootHash(withExtra)
	if rootBase == rootExtra {
		t.Fatal("expected root hash to change when a resource is added")
	}
}

func TestComputeRootHashSingleResource(t *testing.T) {
	// Odd-length levels (including a single leaf) must not panic and must
	// still produce a hash-of-hash, not the raw leaf.
	hashes := []ResourceHash{{ResourceID: "s3:bucket-1", Hash: HashBytes([]byte("config"))}}
	root, err := ComputeRootHash(hashes)
	if err != nil {
		t.Fatalf("ComputeRootHash: %v", err)
	}
	if root == hashes[0].Hash {
		t.Fatal("root hash of a single leaf should not equal the raw leaf hash")
	}
}

func TestComputeRootHashChangesOnResourceIdentityChange(t *testing.T) {
	// Same content hash, different resource ID (e.g. AWS reused the same
	// config for a different subnet). The Merkle root must still change,
	// because InfraKey's leaves commit to identity, not just content —
	// otherwise swapping which resource a hash belongs to would go
	// undetected by the root hash alone.
	contentHash := HashBytes([]byte("identical config"))
	before := []ResourceHash{{ResourceID: "subnet-001", Hash: contentHash}}
	after := []ResourceHash{{ResourceID: "subnet-999", Hash: contentHash}}

	rootBefore, err := ComputeRootHash(before)
	if err != nil {
		t.Fatalf("ComputeRootHash: %v", err)
	}
	rootAfter, err := ComputeRootHash(after)
	if err != nil {
		t.Fatalf("ComputeRootHash: %v", err)
	}
	if rootBefore == rootAfter {
		t.Fatal("expected root hash to change when a resource ID changes, even with identical content hash")
	}
}

func TestComputeRootHashRejectsEmpty(t *testing.T) {
	if _, err := ComputeRootHash(nil); err == nil {
		t.Fatal("expected error for zero resources")
	}
}

func TestComputeRootHashRejectsDuplicateResourceID(t *testing.T) {
	hashes := []ResourceHash{
		{ResourceID: "ec2:i-1", Hash: HashBytes([]byte("config-a"))},
		{ResourceID: "ec2:i-1", Hash: HashBytes([]byte("config-b"))}, // same ID, different content
	}
	if _, err := ComputeRootHash(hashes); err == nil {
		t.Fatal("expected error for duplicate ResourceID, got nil")
	}
}

func TestVerifyRootHash(t *testing.T) {
	hashes := []ResourceHash{
		{ResourceID: "ec2:i-1", Hash: HashBytes([]byte("config-1"))},
		{ResourceID: "vpc:v-1", Hash: HashBytes([]byte("config-2"))},
	}
	root, err := ComputeRootHash(hashes)
	if err != nil {
		t.Fatalf("ComputeRootHash: %v", err)
	}

	ok, err := VerifyRootHash(hashes, root)
	if err != nil {
		t.Fatalf("VerifyRootHash: %v", err)
	}
	if !ok {
		t.Fatal("expected VerifyRootHash to succeed for matching root hash")
	}

	tampered := append([]ResourceHash{}, hashes...)
	tampered[0].Hash = HashBytes([]byte("modified"))
	ok, err = VerifyRootHash(tampered, root)
	if err != nil {
		t.Fatalf("VerifyRootHash: %v", err)
	}
	if ok {
		t.Fatal("expected VerifyRootHash to fail after tampering with a resource hash")
	}
}

// TestVerifyRootHashRejectsMalformedExpectedHash covers inputs that are
// not valid resource hashes at all, distinct from a well-formed-but-wrong
// hash (which the mismatch case above already covers).
func TestVerifyRootHashRejectsMalformedExpectedHash(t *testing.T) {
	hashes := []ResourceHash{
		{ResourceID: "ec2:i-1", Hash: HashBytes([]byte("config-1"))},
	}

	cases := []struct {
		name     string
		expected string
	}{
		{"not hex", "not-hex"},
		{"too short", "abcd"},
		{"empty string", ""},
		{"odd-length hex", "abc"},
		{"valid hex, wrong length", "aabbcc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := VerifyRootHash(hashes, tc.expected)
			if err == nil && ok {
				t.Fatalf("expected malformed expectedRootHash %q to fail verification", tc.expected)
			}
		})
	}
}
