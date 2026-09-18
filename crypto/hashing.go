package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
)

// HashSize is the length in bytes of a SHA-256 digest.
const HashSize = sha256.Size

// HashBytes returns the lowercase-hex SHA-256 digest of data.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HashReader streams r and returns the lowercase-hex SHA-256 digest of its
// contents, without holding the whole input in memory. Useful for hashing
// individual resource files as they're written to the vault.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ResourceHash pairs a stable resource identifier (e.g. "ec2:i-0123abcd")
// with the hex-encoded SHA-256 hash of that resource's normalized,
// serialized configuration. This is the unit that CloudVault/InfraKey
// hashes and later verifies with `verify`.
type ResourceHash struct {
	ResourceID string
	Hash       string // lowercase hex SHA-256
}

// HashResource computes the ResourceHash for a resource's serialized
// configuration bytes. Callers are responsible for serializing the
// resource deterministically (e.g. via canonical JSON) before calling
// this, so that the same logical resource always hashes the same way.
func HashResource(resourceID string, serializedConfig []byte) ResourceHash {
	return ResourceHash{
		ResourceID: resourceID,
		Hash:       HashBytes(serializedConfig),
	}
}

// ComputeRootHash builds a Merkle tree over a set of resource hashes and
// returns the lowercase-hex root hash. This becomes the snapshot
// manifest's "rootHash": a single value that changes if *any* resource's
// hash changes, is added, or is removed, without requiring the verifier to
// compare every resource hash individually to detect tampering.
//
// Leaves are sorted by ResourceID first so that the root hash is
// deterministic regardless of the order resources were discovered in.
//
// It is an error to pass two ResourceHash entries with the same
// ResourceID: InfraKey's invariant is one ResourceID maps to exactly one
// resource, and silently accepting duplicates would let a malformed or
// tampered snapshot hide a resource behind a same-ID collision instead of
// surfacing as a build/serialization bug.
func ComputeRootHash(hashes []ResourceHash) (string, error) {
	if len(hashes) == 0 {
		return "", errors.New("crypto: cannot compute root hash of zero resources")
	}

	sorted := make([]ResourceHash, len(hashes))
	copy(sorted, hashes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ResourceID < sorted[j].ResourceID })

	for i := 1; i < len(sorted); i++ {
		if sorted[i].ResourceID == sorted[i-1].ResourceID {
			return "", fmt.Errorf("crypto: duplicate ResourceID %q in hash set", sorted[i].ResourceID)
		}
	}

	leaves := make([][]byte, len(sorted))
	for i, rh := range sorted {
		leaves[i] = leafDigest(rh.ResourceID, rh.Hash)
	}

	root := merkleRoot(leaves)
	return hex.EncodeToString(root), nil
}

// leafDigest hashes a resource ID together with its content hash, so that
// swapping which resource a given hash belongs to would also change the
// leaf digest.
func leafDigest(resourceID, hash string) []byte {
	h := sha256.New()
	h.Write([]byte("infrakey:leaf:"))
	h.Write([]byte(resourceID))
	h.Write([]byte{0}) // separator to avoid ID/hash concatenation ambiguity
	h.Write([]byte(hash))
	return h.Sum(nil)
}

// merkleRoot computes a binary Merkle root over leaves. If there's an odd
// number of nodes at a level, the last node is duplicated (Bitcoin-style)
// rather than promoted unhashed, so every root is the output of at least
// one hash of two children.
func merkleRoot(leaves [][]byte) []byte {
	level := leaves
	for len(level) > 1 {
		var next [][]byte
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			var right []byte
			if i+1 < len(level) {
				right = level[i+1]
			} else {
				right = level[i] // duplicate last node
			}
			next = append(next, parentDigest(left, right))
		}
		level = next
	}
	return level[0]
}

func parentDigest(left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte("infrakey:node:"))
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

// VerifyRootHash recomputes the Merkle root over hashes and reports
// whether it matches expectedRootHash (hex-encoded). Use this in
// `infrakey verify` to check a manifest's rootHash against the resources
// actually present on the vault.
func VerifyRootHash(hashes []ResourceHash, expectedRootHash string) (bool, error) {
	got, err := ComputeRootHash(hashes)
	if err != nil {
		return false, err
	}
	// Root hashes are public integrity values rather than secrets.
	// A constant-time comparison is therefore unnecessary here.
	gotBytes, err := hex.DecodeString(got)
	if err != nil {
		return false, err
	}
	wantBytes, err := hex.DecodeString(expectedRootHash)
	if err != nil {
		return false, errors.New("crypto: expectedRootHash is not valid hex")
	}
	return bytes.Equal(gotBytes, wantBytes), nil
}
