# Threat Model

> **Status: Describes the `crypto/` package as implemented, plus the trust
> decisions that still need to be made in layers built on top of it
> (`state/`, `config/`, `media/`). Where a mitigation is not yet wired up,
> this document says so explicitly rather than assuming it.**

## 1. What InfraResc Is Protecting

InfraResc's recovery artifact is expected to leave the organization's
custody at some point — sitting on a USB drive in a safe, mailed to an
off-site location, or handed to whoever performs the recovery. The threat
model therefore has to assume the artifact itself is **not** a trusted
environment.

Three properties matter for that artifact:

```text
CONFIDENTIALITY   Can someone who finds/steals the vault read the
                  infrastructure configuration inside it?

INTEGRITY         Can someone who has the vault modify its contents
                  without that modification being detected?

AUTHENTICITY      Can someone convince a recovery operator that a
                  vault (or part of one) came from InfraResc, when
                  it didn't?
```

`crypto/` provides the primitives for all three. Whether those primitives
are actually load-bearing depends on how `state/`, `config/`, and `media/`
use them — see Section 5.

## 2. Assets

| Asset | Where it lives | Sensitivity |
|---|---|---|
| Infrastructure configuration (VPC layout, security group rules, IAM policy documents, resource IDs) | Inside the encrypted snapshot | Confidential — reveals attack surface of the target AWS environment |
| Snapshot integrity (the fact that a resource hasn't been silently altered) | Merkle root hash in the manifest | Integrity-critical — a tampered snapshot could cause a bad recovery |
| Ed25519 private signing key | Wherever `config`/`init` ends up storing it (not yet built) | Highly sensitive — compromise lets an attacker forge manifests |
| Ed25519 public key used for verification | Local config on the recovery workstation, **not** the vault | Must be tamper-resistant, not secret |
| Vault passphrase | Operator's memory / password manager | Highly sensitive — compromise lets an attacker decrypt the vault |
| AWS credentials used during `recover --execute` | Operator's environment at recovery time | Out of scope for `crypto/`; never stored in the vault by design |

## 3. Trust Boundaries

```text
                    TRUSTED                    UNTRUSTED
                       |                            |
   Recovery operator's |      Physical recovery     |
   local config         |         medium (USB,       |
   (trusted public key) |      cloud object, etc.)   |
                       |                            |
   Passphrase in the    |      Encrypted vault:      |
   operator's head      |        - EncryptedBlob     |
                       |        - SignedManifest     |
   AWS credentials at   |        - resource files    |
   recovery time         |                            |
                       |                            |
```

Everything to the right of the line is assumed to be readable and
modifiable by an attacker. `crypto/`'s job is to make sure that assumption
is contained rather than catastrophic: an attacker who can read or edit
the protected vault contents cannot recover the plaintext without the
passphrase, and cannot produce a vault that passes authenticity
verification without the trusted signing key. This is narrower than "gains
nothing" — the artifact's unencrypted metadata (file sizes, resource
counts, timestamps, whatever the manifest exposes outside the encrypted
blob), and the mere ability to destroy or withhold the artifact
(a denial-of-service on availability, not confidentiality or integrity),
are not covered by this guarantee. See §4.7 for the availability case
specifically.

## 4. Threat Scenarios and Current Mitigations

### 4.0 At a Glance

| Threat | Attacker capability | Impact | Mitigation | Status |
|---|---|---|---|---|
| Vault theft | Read-only physical access | Confidentiality loss | Argon2id + AES-256-GCM | Implemented |
| Vault modification | Full artifact write access | Integrity loss | AES-GCM auth tag + Merkle root + Ed25519 signature | Partially implemented — see §5 |
| Key replacement (self-signed forgery) | Full artifact control | Authenticity loss | Independently-supplied trusted public key | Primitive implemented; trusted-key storage pending |
| KDF parameter tampering | Full artifact write access | Denial of service on unlock | Bounded Argon2id parameters, checked pre-derivation | Implemented |
| Replay of an old valid manifest | Possession of a previously-valid artifact | Stale recovery | Snapshot version/timestamp check | Not implemented |
| Future algorithm break (e.g. quantum) | N/A (forward-looking) | Long-term confidentiality/authenticity loss | Versioned envelope format | Design affordance only — no migration path yet |

The rest of this section walks through each row in detail.

### 4.1 Attacker obtains the physical vault (USB lost, stolen, mailed to wrong address)

**Goal:** read the infrastructure configuration inside.

**Mitigation:** `EncryptWithPassphrase` / `DecryptWithPassphrase`
(`encryption.go`). Contents are AES-256-GCM encrypted under a key derived
via Argon2id from an operator-supplied passphrase. Without the passphrase,
the attacker has ciphertext and a public salt — no path to the plaintext
short of brute-forcing the passphrase itself, which is exactly what
Argon2id's memory-hardness is intended to make expensive.

**Residual risk:** the strength of this mitigation is only as good as the
passphrase. `crypto/` doesn't enforce passphrase quality — that's a
`cli`/`config` concern.

### 4.2 Attacker modifies vault contents in place (tampered USB, compromised transport, malicious insider with physical access)

**Goal:** cause the recovery operator to unknowingly recover from
altered/malicious infrastructure state.

**Mitigation, layer 1 — confidentiality boundary catches gross tampering:**
AES-GCM is an authenticated-encryption scheme. Flipping any bit in the ciphertext (or
in the additional authenticated data, if used) causes `DecryptWithPassphrase`
to fail outright rather than return corrupted plaintext. See
`TestDecryptTamperedCiphertextFails`, `TestDecryptTamperedAdditionalDataFails`.

**Mitigation, layer 2 — integrity of the resource set:** `ComputeRootHash`
(`hashing.go`) builds a Merkle root over every resource's identity-committing
hash (`ResourceID + content hash`, domain-separated). Adding, removing,
modifying, or **relabeling** a resource (same content, different ID) all
change the root. See `TestComputeRootHashChangesOnTamper`,
`TestComputeRootHashChangesOnResourceIdentityChange`,
`TestComputeRootHashDetectsAddedOrRemovedResource`.

**Mitigation, layer 3 — authenticity of the manifest itself:**
`VerifySignedManifest` (`signature.go`) checks the manifest's Ed25519
signature against a trusted public key. See Section 4.3 for why this only
works if the trusted key comes from somewhere the attacker doesn't control.

### 4.3 Attacker controls the entire vault, including whatever "trusted" key material ships inside it

This is the scenario that broke earlier drafts of the signature design and
is worth stating precisely.

**Attack:** an attacker who can rewrite the whole artifact generates a
fresh Ed25519 keypair, signs whatever manifest they want with it, and
embeds the matching public key in the vault. If the verifier trusts
whatever public key is sitting next to the signature, this passes
verification — the forgery is internally self-consistent.

**Mitigation:** `VerifySignedManifest` makes `trustedPublicKey` a mandatory
argument; passing `nil` is a hard error (`TestVerifySignedManifestRequiresTrustedKey`).
The only function that trusts an embedded key with no external check is
`VerifySignedManifestTOFU`, which is explicitly named and documented as a
first-use-only escape hatch, not something the recovery path should ever
call.

**Open item:** this mitigation is only real if `trustedPublicKey` actually
comes from somewhere outside the attacker's control. `crypto/` cannot
enforce that — it's a decision `config`/`init` has to make and hasn't yet
(see Section 5).

### 4.4 Attacker tampers with the vault's own key-derivation parameters

**Attack:** modify `EncryptedBlob.KDF` (e.g. inflate `Memory` or `Time`) so
that a recovery operator's machine spends excessive CPU/RAM just trying to
unlock the vault — a denial-of-service against the recovery process itself,
at the worst possible moment.

**Mitigation:** `ValidateKDFParams` bounds-checks every KDF parameter
(time, memory, threads, salt length, key length) before `DeriveKey` ever
calls Argon2id. `DecryptWithPassphrase` runs this and other cheap
structural checks (nonce length, minimum ciphertext length, version,
algorithm) before deriving a key at all. See
`TestDeriveKeyRejectsMaliciousParams`,
`TestDecryptRejectsMalformedStructureBeforeKeyDerivation`.

### 4.5 Attacker replays an old, previously-valid signed manifest

**Attack:** substitute a legitimately-signed but outdated manifest (e.g.
from before a security group was locked down) for the current one. The
signature verifies — it's genuinely valid — but it now describes a stale,
possibly less secure state.

**Mitigation: none yet, at the `crypto/` layer, by design.** Signature
verification proves *authenticity and integrity of what's being verified*,
not *recency*. Preventing replay requires the caller to check something
outside the signature itself — a monotonically increasing snapshot ID or
timestamp recorded separately, and a policy for what "too old" means. This
belongs in `state/` or `recovery/`, not `crypto/`.

### 4.6 Attacker with a quantum computer, at some future date

Ed25519 and AES-256 are not post-quantum secure. `SignedManifest` and
`EncryptedBlob` both carry explicit `Version`/`Algorithm` fields precisely
so that a future algorithm migration has somewhere to declare itself
without ambiguity about how to read vaults written under the old scheme.
No migration path is implemented yet; this is a design affordance, not a
current mitigation.

### 4.7 Attacker with only metadata visibility, or the ability to destroy the artifact

Two things confidentiality and integrity mechanisms don't cover:

**Metadata leakage.** Encryption protects the *contents* of the encrypted
blob. It says nothing about whatever isn't inside that blob — file sizes on
the recovery medium, timestamps, a resource count if one is ever stored in
plaintext manifest fields, or traffic patterns if a vault is ever
transmitted over a network. An attacker limited to observing these can
infer things (roughly how large the infrastructure is, roughly when it was
snapshotted) without ever breaking AES-GCM or Argon2id. Whether this
matters depends on what InfraResc chooses to leave outside the encrypted
blob when the manifest format is finalized — a decision not yet made.

**Destruction / withholding.** An attacker who can access the physical
medium can destroy it, or an attacker who intercepts it in transit can
simply not deliver it. This is a straightforward availability attack:
confidentiality and integrity mechanisms make forged or read contents
useless to the attacker, but they cannot make a destroyed or missing
artifact usable to the defender. Mitigating this is an operational
concern (multiple copies, off-site storage, redundant recovery media)
rather than something `crypto/` can address — cryptography can guarantee
that an artifact hasn't been tampered with, not that an artifact exists at
all.

## 5. Open Trust-Placement Decisions (Not Yet Resolved)

These are the items `crypto/` cannot close on its own:

1. **Where does the trusted Ed25519 public key live?** It must be recorded
   somewhere outside the vault artifact at `init` time and read from that
   location — not derived from anything the vault carries — every time
   `recover` or `verify` runs. Not yet implemented; `config/` does not
   exist in the repository yet.

2. **Where does the Ed25519 private signing key live?** Never on the vault.
   Candidate approaches (operator's local machine, a separate secured
   location, eventual hardware-backed storage) are discussed in
   [`key-management.md`](./key-management.md).

3. **How does the state layer guarantee canonical serialization?**
   `HashResource` hashes whatever bytes it's given; it does not serialize
   resources itself. If two components ever produce different bytes for
   the same logical resource, the whole integrity chain (`hashing.go` →
   Merkle root → `SignedManifest`) becomes meaningless despite every
   individual primitive being correct. This is `state/serialization.go`'s
   responsibility and is not yet implemented — `state/resource.go`
   currently stores `Configuration` as a raw `map[string]any` with no
   canonicalization step before it would reach `HashResource`.

4. **Replay/staleness policy** (Section 4.5) — not yet designed.

## 6. What's Explicitly Out of Scope for `crypto/`

* AWS credential handling during `recover --execute`.
* Passphrase strength enforcement or storage (password managers, etc.).
* Physical security of the recovery medium itself.
* Network security — InfraResc's design intentionally avoids needing the
  vault to travel over a network at all for the core recovery path.

## 7. Security Guarantees Summary

For a fast status check without reading the rest of this document:

```text
Guarantees currently provided by crypto/:

1. Vault contents are confidential without the passphrase.
2. Ciphertext tampering is detected by AES-GCM.
3. Resource-set changes (additions, removals, content edits) are
  detectable through the Merkle root.
4. Resource identity is committed to the Merkle leaves, so relabeling
  a resource is also detectable.
5. Manifest authenticity can be verified — PROVIDED the caller supplies
  an independently trusted Ed25519 public key rather than trusting
  whatever key is embedded in the artifact.
6. Malicious or malformed KDF parameters are bounded and rejected
  before any expensive key derivation runs.
7. Malformed structural input (bad nonce/ciphertext/signature/key
  lengths) is rejected explicitly rather than causing undefined
  behavior.

Guarantees NOT yet provided end-to-end (primitives exist in crypto/,
but the surrounding system doesn't yet use them correctly):

1. Trusted public-key storage — no config/init exists yet to record a
  public key outside the vault. Without this, VerifySignedManifest's
  mandatory trustedPublicKey argument has nowhere trustworthy to come
  from yet.
2. Canonical resource serialization — state/ does not yet guarantee
  that the same logical resource always produces the same bytes
  before hashing. See integrity-model.md §5.
3. Snapshot replay/staleness protection — a validly-signed old
  manifest is not currently distinguishable from the current one.
4. Ed25519 private-key storage and rotation policy — not yet
  implemented; see key-management.md §3.1 and §5.
```

This list should be kept in sync as the open items above are closed —
treat an unchecked item here as a standing TODO for whichever component
is responsible for it, not just a documentation note.

