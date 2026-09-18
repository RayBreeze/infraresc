# Integrity Model

> **Status: Describes the hashing and signing mechanisms implemented in
> `crypto/`. The "single canonicalization contract" this model depends on
> is a `state/` responsibility and is not yet implemented — see §5.**

## 1. What "Integrity" Means Here

A recovery snapshot has integrity if a verifier can detect, with high
confidence, any of the following:

* a resource's configuration was modified after the snapshot was taken;
* a resource was added or removed from the snapshot;
* a resource's identity was changed while its content stayed the same
  (or vice versa);
* the manifest describing the snapshot was modified or replaced;
* the entire artifact was produced by something other than a trusted
  InfraResc installation.

Each of these is caught by a different mechanism, layered from individual
resources up to the whole artifact.

## 2. The Integrity Chain

```text
Logical Resource
      |
      v
Canonical Serialization    <-- NOT YET IMPLEMENTED (see §5)
      |
      v
  SHA-256                  HashResource() / HashBytes()
      |
      v
ResourceHash{ID, Hash}
      |
      v
Domain-Separated,          ComputeRootHash()
Identity-Committing
Merkle Tree
      |
      v
  Root Hash (hex)   ------->  stored as the manifest's rootHash
      |
      v
  Ed25519 Signature         SignManifest()
      |
      v
  SignedManifest
```

Every step from `SHA-256` down to `SignedManifest` is implemented and
tested in `crypto/`. The top step — canonical serialization — is not, and
its absence is a real gap; see §5 before treating this chain as complete.

## 3. Resource-Level Hashing

`HashResource(resourceID, serializedConfig)` (`hashing.go`) produces a
`ResourceHash{ResourceID, Hash}` pair, where `Hash` is the hex-encoded
SHA-256 digest of whatever bytes it's given for that resource.

This function does not serialize the resource itself — that's an explicit
design boundary (see `doc.go`, point 6, and §5 below). It hashes bytes; it
does not decide what the bytes mean.

## 4. Merkle Root Construction

`ComputeRootHash` builds a binary Merkle tree over a set of
`ResourceHash` values and returns a single root hash for the entire
snapshot's resource set.

### 4.1 Determinism regardless of discovery order

Resources are sorted by `ResourceID` before the tree is built
(`TestComputeRootHashOrderIndependent`), so the root hash depends only on
*which* resources exist and their content — not on the order AWS happened
to return them in during discovery.

### 4.2 Leaves commit to identity, not just content

A naive Merkle tree over content hashes alone has a gap: two resources
with identical configuration but different IDs would produce identical
leaves, so swapping which resource an unchanged hash "belongs to" wouldn't
change the root. `leafDigest` closes this by hashing `ResourceID` together
with the content hash:

```text
leaf = SHA256("infrakey:leaf:" || ResourceID || 0x00 || ContentHash)
```

Verified by `TestComputeRootHashChangesOnResourceIdentityChange`: identical
content hash, different `ResourceID`, different root.

### 4.3 Domain separation

Leaf digests and internal node digests use distinct prefixes
(`"infrakey:leaf:"` vs. `"infrakey:node:"`) before hashing. Without this,
a leaf value and an internal-node value could, at least structurally, be
computed the same way — a classic ambiguity in naive Merkle constructions,
where nothing distinguishes "this is a leaf" from "this is two children
hashed together." Domain separation ensures that leaf and internal-node
digests are computed under distinct hash domains, preventing the two
constructions from being interchangeable by simple structural ambiguity.
This is a defense against a specific, well-known class of Merkle-tree
confusion, not a blanket guarantee against every conceivable attack on the
tree.

### 4.4 Odd-length levels

When a tree level has an odd number of nodes, the last node is duplicated
(matching the common Bitcoin-style convention) rather than promoted
unhashed to the next level — every value at every level is the output of
at least one hash of two children, including the case of a single-resource
snapshot (`TestComputeRootHashSingleResource`).

### 4.5 Duplicate resource IDs are rejected outright

`ComputeRootHash` treats two entries with the same `ResourceID` as an
error, not a silently-accepted edge case
(`TestComputeRootHashRejectsDuplicateResourceID`). InfraResc's invariant is
one `ResourceID` maps to exactly one resource; a duplicate is either a bug
upstream or a tampering attempt, and either way should surface loudly
rather than get resolved by whichever hash happened to sort first.

### 4.6 What a different root hash actually indicates

A different root hash indicates that the committed resource set differs
from the expected resource set — addition, removal, identity swap, or
content modification — **assuming SHA-256 remains collision- and
second-preimage-resistant, and assuming both root hashes were computed
under the same canonicalization rules.** This is a precise but important
distinction: a cryptographic hash doesn't mathematically *prove* set
equality or inequality in an absolute sense. It provides evidence that is
computationally infeasible to fake under the stated assumptions about
SHA-256 — which is the standard, well-founded basis for treating hash
comparison as reliable, but it's a security assumption, not a
mathematical certainty independent of that assumption.

A matching root hash likewise does not by itself tell you *what*, if
anything, changed when it doesn't match; a verifier that needs to localize
a detected change has to compare the full set of `ResourceHash` values,
not just the root. (`VerifyRootHash` supports exactly this: it recomputes
the root from a candidate resource set and compares against an expected
value.)

## 5. The Missing Piece: Canonical Serialization

This is the most important limitation of the current integrity model, and
it lives entirely outside `crypto/`.

`HashResource` will happily produce different hashes for two byte
representations of what a human would call "the same resource":

```json
{"name":"web","port":443}
```
```json
{
  "port": 443,
  "name": "web"
}
```

These are logically identical but are different byte sequences, so they
hash differently. If one part of InfraResc serializes a resource one way
and another part serializes it another way — or if the same resource is
serialized differently across two runs due to non-deterministic map
iteration, for example — the resulting hashes won't match even though
nothing about the underlying infrastructure changed. That would make
`verify` report tampering that never happened (or worse, mask a change
that did, if the mismatch is dismissed as "just a formatting difference").

**Current state:** `state/resource.go` stores a resource's configuration as
`map[string]any` with no defined canonicalization step before that data
would reach `HashResource`. To be precise about what's actually at issue:
Go's `encoding/json` has sorted `map[string]T` keys deterministically since
Go 1.12, so ordinary `json.Marshal` output is not the wild-west problem it
might sound like. The real gap is broader than key ordering — ordinary
JSON marshaling doesn't, on its own, pin down everything a canonicalization
contract needs: whether `1` and `1.0` are the same value, how arrays are
ordered (they're semantically ordered and should never be sorted the way
object keys can be), how Unicode is escaped, and — the InfraResc-specific
piece — whether the hash covers only the `configuration` field or also
`ResourceID`, `ResourceType`, and `Region`. Two components each calling
`json.Marshal` "correctly" can still disagree on one of these and produce
different bytes for what a human would call the same resource. The
argument for an explicit contract doesn't rest on `encoding/json` being
unreliable; it rests on "deterministic" and "canonical" not being the same
guarantee.

**Required before the integrity model can be trusted end-to-end:** a single
canonical serialization contract, implemented once in
`state/serialization.go` and used everywhere a resource is hashed. At
minimum it needs to define:

1. Object keys sorted deterministically.
2. Array ordering preserved (arrays are semantically ordered; unlike
   object keys, they should not be reordered).
3. A single, explicit rule for numeric representation (integer vs.
   floating-point formatting, no ambiguity between `1` and `1.0`).
4. UTF-8 encoding, with defined escaping rules.
5. The canonicalization algorithm itself versioned, so a future change to
   the rules doesn't silently invalidate every previously-computed hash.
6. The canonical form must cover more than just the `configuration` field
   — `ResourceID`, `ResourceType`, and `Region` are security-relevant and
   should be part of what gets hashed, not just the configuration blob, so
   that a metadata change (e.g. a resource's recorded region) isn't
   invisible to the hash even when the raw config bytes are untouched.

Until this exists, treat the integrity chain above as **correct but
incomplete**: every step from resource hash upward is solid, but what goes
into a resource hash in the first place isn't yet guaranteed to be stable.

## 6. Manifest-Level Authenticity

Once a root hash exists, it needs to be bound to a manifest and that
manifest needs to be provably untampered. `SignManifest` /
`VerifySignedManifest` (`signature.go`) provide this:

* `SignedManifest` carries `Version`, `Algorithm` (`"Ed25519"`), the raw
  manifest bytes, the signature, and the signer's public key.
* Verification is **only** meaningful when checked against a
  `trustedPublicKey` obtained independently of the artifact — see
  [`threat-model.md`](./threat-model.md) §4.3 for the attack this
  prevents, and [`key-management.md`](./key-management.md) §3.2 for where
  that trusted key needs to live (not yet implemented).

## 7. Note on `state.Manifest` vs. `crypto.SignedManifest`

The repository currently has two different types named similarly but
structurally unrelated:

```text
state.Manifest                    crypto.SignedManifest
  Version   string                  Version   uint8
  Snapshot  string                  Algorithm string
  Algorithm string                  Manifest  []byte
  Files     map[string]string       Signature []byte
  RootHash  string                  PublicKey []byte
```

These have not been connected yet. `state.Manifest` looks like it's meant
to become the plaintext content that eventually gets serialized and passed
into `SignManifest` as `manifestBytes` (and, likely, its `RootHash` field
is meant to be populated by `ComputeRootHash`) — but that wiring doesn't
exist in the codebase yet. Anyone picking up `state/` work next should
treat resolving this relationship as a prerequisite for the integrity
model actually functioning end-to-end, not an implementation detail to
defer.
