# Key Management

> **Status: Describes the key-handling primitives that exist in `crypto/`
> today, and the storage/lifecycle decisions that are still open because
> `config/` has not been built yet. Sections marked "Proposed" are design
> intent, not implemented behavior.**

## 1. Two Independent Key Types

InfraResc uses two unrelated kinds of key material, serving different
purposes. They should never be confused or derived from one another.

```text
                    KEY MATERIAL
                         |
           +-------------+-------------+
           |                           |
   SYMMETRIC (AES-256)         ASYMMETRIC (Ed25519)
           |                           |
   Confidentiality              Authenticity
   of vault contents            of the manifest
           |                           |
   Derived on demand            Generated once,
   from a passphrase            stored, reused
   via Argon2id                 across snapshots
```

## 2. Symmetric Encryption Keys

**Never stored, anywhere, ever.** This is the load-bearing design decision
for confidentiality: the vault contains no key that would let someone who
merely possesses the vault decrypt it.

```text
Operator passphrase
        |
        v
  Argon2id (memory-hard KDF)
        |
        v
  256-bit AES key  ------->  used once, then zeroed
```

* `NewKDFParams` generates a fresh random 16-byte salt for every
  encryption operation — the same passphrase encrypting two different
  snapshots produces two unrelated keys. See
  `TestEncryptProducesUniqueNoncesAndCiphertexts`.
* `DeriveKey` is the single function that ever calls Argon2id, and it
  refuses to run with out-of-bounds parameters (see `ValidateKDFParams`
  and [`threat-model.md`](./threat-model.md) §4.4).
* `zero()` overwrites the derived key's bytes after use
  (`encryption.go`). This is best-effort hygiene, not a guarantee — Go's
  garbage collector can still leave copies elsewhere in memory — but it
  shortens the window the key exists in a recoverable form.
* The KDF parameters (time, memory, threads, salt) travel inside
  `EncryptedBlob` itself, so a vault is self-describing: `DecryptWithPassphrase`
  never has to guess which cost parameters were used to lock it, even if
  `DefaultArgon2*` changes in a future release.

**Passphrase quality is out of scope for `crypto/`.** A weak passphrase
undermines everything above regardless of how expensive Argon2id makes
each guess. Passphrase strength policy (minimum length, entropy checks,
generation assistance) belongs in `cli`/`config`, not here.

**Passphrase rotation:** re-encrypting a snapshot under a new passphrase is
just calling `EncryptWithPassphrase` again with the new passphrase — a
fresh salt and KDF parameters are generated automatically. There is
currently no CLI command that does this; it would call directly into
`crypto/`.

## 3. Ed25519 Signing Keys

Unlike the AES key, the Ed25519 keypair is **not** re-derived on demand —
it's generated once (`GenerateKeyPair`) and reused to sign every manifest
produced by that InfraResc installation, so that a recovery workstation can
verify manifests it has never seen before, as long as it already trusts
that installation's public key.

```text
     GenerateKeyPair()
            |
   +--------+--------+
   |                 |
PrivateKey       PublicKey
   |                 |
Signs manifests   Verifies manifests
   |                 |
NEVER on the      Recorded on the recovery
recovery vault    workstation independently
                  of the vault (Proposed —
                  see §4)
```

### 3.1 Private key

* Used only via `KeyPair.Sign`.
* Must never be written to the recovery medium. `SignManifest` takes a
  `*KeyPair` and only ever writes `kp.PublicKey` into the resulting
  `SignedManifest` — the private key itself never touches the serialized
  output.
* **Storage location is not yet implemented.** There is no `config`
  package in the repository yet. Candidate designs, in increasing order of
  robustness:
  1. A file on the machine that runs `snapshot`/`init`, permissions
     restricted to the owning user (`0600` equivalent), outside the vault
     directory entirely.
  2. An OS-native secret store (Keychain / Credential Manager / Secret
     Service) if InfraResc later needs to run unattended.
  3. A hardware-backed key (secure element, TPM, external signing device)
     — listed as a future direction in the project's technical stack, not
     yet designed for InfraResc specifically.
* **Compromise impact:** whoever holds the private key can produce
  manifests that verify successfully against the corresponding public key
  — meaning they can forge a "legitimate-looking" recovery snapshot. This
  is the single most sensitive piece of key material in the system.

### 3.2 Public key

* Embedded in every `SignedManifest` for convenience
  (`SignedManifest.PublicKey`), but **the embedded copy is never the root
  of trust** — see `VerifySignedManifest`'s requirement for an
  independently-supplied `trustedPublicKey`, and
  [`threat-model.md`](./threat-model.md) §4.3 for why.
* The actual trusted copy needs to live somewhere a vault-level attacker
  cannot modify: recorded once at `init` time, read from local
  configuration on the recovery workstation thereafter. **Not yet
  implemented** — this is the most important open item in `config/`.
* Not secret. It can be freely copied, backed up, or even published,
  as long as its *integrity* (not being silently swapped for a different
  key) is protected wherever it's stored.

### 3.3 Trust-on-first-use, and why it's isolated

`VerifySignedManifestTOFU` exists for exactly one moment: accepting an
**existing** artifact's embedded public key when there is no independent
trust record for it yet — for example, importing a vault produced by a
different InfraResc installation, where the recipient has never recorded
that installation's key before and has to make a first-contact trust
decision. It is *not* what `init` uses on a vault's own first manifest —
see §4 for why `init` doesn't need TOFU at all, since it already generated
the keypair itself. `VerifySignedManifestTOFU` is deliberately a separate,
distinctly-named function rather than a `nil`able argument on
`VerifySignedManifest`, so that a call to it anywhere in a codebase reads
as a flag for review: *is this really a first-contact trust decision, or
did something skip building the actual trust store?*

## 4. Proposed: Key Lifecycle at `init` Time

Not yet implemented — sketched here so `config/init` has a concrete target:

```text
infrakey init
      |
      v
GenerateKeyPair()
      |
      +----------------------------+
      |                            |
PrivateKey                    PublicKey
      |                            |
Store locally,               Becomes the initial trust
restricted permissions,      anchor directly: record it
never written to vault       in local config AND embed
                              it in vault manifests
```

At `init` time there is nothing to bootstrap trust *from* — the process
that just called `GenerateKeyPair` already holds both halves of the
keypair with certainty, so the first manifest it signs can simply be
checked against that freshly-generated public key directly. This isn't a
trust-on-first-use situation at all; it's the trust anchor being created,
not discovered.

`VerifySignedManifestTOFU` is for a different situation: accepting an
**existing** artifact's embedded key before any external trust record for
it exists — for example, importing a vault that some other InfraResc
installation produced, where the recipient has no independent record of
that installation's public key yet and has to make a first-contact
trust decision. That's a materially weaker position than `init` (you're
trusting a key you didn't generate, on an artifact you didn't create), so
using it should prompt an explicit "am I willing to trust whoever/whatever
produced this artifact?" decision — never silently, and never on InfraResc's
own `verify`/`recover` path for a vault it already has a trust record for.

From this point forward, every `verify`/`recover` invocation on a vault
whose key was already recorded — whether at that vault's own `init` or at
a prior import — should load the recorded public key from local config
and call `VerifySignedManifest` with it. `VerifySignedManifestTOFU` should
not appear anywhere in that steady-state path.

## 5. Key Rotation (Proposed)

Not yet designed in detail. Open questions to resolve before implementing:

* Should an InfraResc installation support multiple valid public keys
  simultaneously (to allow rotation without invalidating old, still-valid
  snapshots)?
* If a private key is suspected compromised, is there a revocation
  mechanism, or does trust simply move to a newly generated keypair for
  all *future* snapshots while old ones are re-verified manually?


