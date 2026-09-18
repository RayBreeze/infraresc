// Package crypto implements the cryptographic primitives used by InfraKey
// to protect and verify offline recovery snapshots.
//
// Three concerns are covered, matching the InfraKey design (see
// docs/technical-stack):
//
//   - encryption.go  — Argon2id key derivation + AES-256-GCM authenticated
//     encryption of vault contents.
//   - hashing.go     — SHA-256 hashing of individual resources and a Merkle
//     root hash used to fingerprint an entire snapshot.
//   - signature.go   — Ed25519 key generation, signing and verification,
//     used to prove a snapshot manifest has not been tampered with.
//
// Design goals:
//
//  1. No secret ever has to be stored on the recovery medium. Encryption
//     keys are derived on demand from an operator-supplied passphrase plus
//     a stored (public) salt and KDF parameters.
//  2. Every format written to disk is explicitly versioned and
//     self-describing: EncryptedBlob and SignedManifest each carry a
//     Version and Algorithm field alongside the parameters needed to
//     reverse them (salt, nonce, KDF cost parameters). A future InfraKey
//     release can change defaults, or even the underlying algorithm,
//     without ambiguity about how to read an older vault — and this
//     package refuses to process a version/algorithm it doesn't
//     explicitly implement rather than guessing.
//  3. Untrusted input is validated cheaply before anything expensive runs.
//     A vault envelope's KDF parameters, nonce length, and ciphertext
//     length are all attacker-controlled once the recovery medium is
//     considered untrusted, so they're bounds-checked before Argon2id (an
//     intentionally expensive operation) or AES-GCM ever executes. See
//     ValidateKDFParams and validateEncryptedBlobStructure.
//  4. A signature only proves something if it's checked against a key the
//     verifier obtained independently of the artifact being verified.
//     VerifySignedManifest therefore requires a trustedPublicKey argument
//     — the caller's own record of the vault's key (e.g. from
//     `infrakey init`), not whatever key happens to be embedded in the
//     signed manifest itself. The one exception, trust-on-first-use, is
//     its own clearly-named function (VerifySignedManifestTOFU) so that
//     using it anywhere but first-time setup is an obvious code smell.
//  5. Merkle leaves commit to resource identity, not just content: a leaf
//     hashes ResourceID together with the resource's content hash, so
//     swapping which resource an unchanged content hash belongs to still
//     changes the root. This package does not decide what goes into that
//     content hash, though — see point 6.
//  6. Nothing here decides *what* to encrypt/hash/sign — that is the job
//     of the state package. In particular, this package hashes whatever
//     bytes a caller gives it for a resource; it is the state package's
//     responsibility to define and enforce a single canonical
//     serialization (covering resource ID, type, region, configuration,
//     and any other security-relevant metadata) so that two components
//     never produce different bytes — and therefore different hashes —
//     for the same logical resource.
package crypto
