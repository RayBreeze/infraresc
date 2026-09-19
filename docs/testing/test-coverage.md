# InfraResc Test Coverage

> **Current baseline:** the focused test suite is now part of the main codebase and is verified by CI with `go test ./...` on Go 1.27.1. This document records what is covered, what is deliberately deferred, and where the architecture still needs stronger testing seams.

## 1. Testing philosophy

InfraResc is an infrastructure-recovery tool, so the important question is not simply "how much code is covered?" but whether the tests protect the contracts that make a snapshot safe to reason about and eventually recover from.

The current suite follows four principles:

1. **Behavior over implementation details.**
2. **Deterministic tests over live-AWS tests wherever possible.**
3. **Small, explicit seams instead of a large mock hierarchy.**
4. **Do not test placeholder workflows as if they were implemented.**

The current suite contains **38 top-level test functions** across these areas:

| Area | Test file(s) | Tests | What is protected |
|---|---|---:|---|
| Graph | `graph/graph_test.go` | 8 | construction, edge-to-dependency conversion, ordering, determinism, missing dependencies, cycles |
| State | `state/serialization_test.go` | 3 | JSON round-trip, optional-field behavior, invalid JSON |
| AWS discovery | `aws/discovery_test.go` | 2 | ARN/resource identity extraction and nil-safe strings |
| AWS relationships | `aws/relationships_test.go` | 5 | filtering, batching, edge deduplication, pointer extraction, service ARN parsing |
| AWS snapshot/collector | `aws/snapshot_test.go`, `aws/collector_test.go` | 7 | config conversion, resource indexing/filtering, warnings, collector initialization, edge normalization |
| Auth/config | `auth/auth_test.go` | 4 | profile editing, replacement, permissions, validation, provider delegation |
| CLI | `cli/commands_test.go` | 2 | command registration and important flags |
| Local integration | `integration/pipeline_test.go` | 1 | snapshot serialization → graph construction → dependency ordering |
| Crypto artifacts/envelope | `crypto/artifact_test.go`, `crypto/envelope_test.go` | 6 | implemented artifact/envelope behavior, tamper/wrong-password rejection, metadata validation |
| **Total** | | **38** | |

The test count is descriptive rather than a quality target. Adding tests merely to increase the number would not improve this suite.

---

## 2. Graph

**Tests:** `graph/graph_test.go`

Covered:

- Resource-to-node construction.
- Conversion of snapshot edges into graph dependencies through `BuildWithEdges`.
- Dependency ordering.
- Deterministic ordering of independent nodes.
- Deterministic graph snapshots.
- Missing dependency detection.
- Dependency-cycle detection.
- Inclusion of independent nodes.

This is one of the strongest parts of the current suite because it tests actual recovery-relevant behavior rather than getters or data-structure trivia.

### Important boundary

The graph tests establish that **when edges are supplied**, the graph can represent and resolve those dependencies. They do not establish that every AWS relationship discovered by the AWS layer is semantically a recovery prerequisite.

That distinction matters:

```text
AWS observed relationship
        ↓
edge normalization
        ↓
recovery dependency semantics
        ↓
graph ordering
```

The first two stages have coverage; the semantic mapping is still an architectural responsibility of the recovery design.

---

## 3. State and snapshot serialization

**Tests:** `state/serialization_test.go`

Covered:

- Full snapshot JSON round-trip.
- Preservation of resources, edges, graph state, configs, and warnings.
- Omission of empty optional warnings.
- Rejection of malformed JSON.

The tests intentionally do **not** claim that the serialization is canonical or suitable for cryptographic hashing.

### Still needed

- Schema/version migration tests.
- Canonical/deterministic serialization if the application requires it.
- Validation of normalized, recovery-ready configuration.

---

## 4. AWS resource discovery

**Tests:** `aws/discovery_test.go`

Covered:

- Lambda ARN identity extraction.
- RDS instance ARN identity extraction.
- RDS cluster ARN identity extraction.
- DynamoDB table ARN identity extraction.
- S3 bucket identity extraction.
- Slash-delimited EC2 resource identity.
- Plain resource identifiers.
- Malformed/non-ARN fallback behavior.
- Nil-safe string extraction.

The tests deliberately isolate deterministic transformation logic instead of requiring Resource Explorer or AWS credentials.

### Not covered

- Resource Explorer API calls.
- Pagination.
- AWS API errors/retries.
- Real account discovery.

Those require a controlled AWS boundary or a dedicated integration environment.

---

## 5. AWS relationship discovery

**Tests:** `aws/relationships_test.go`

Covered:

- Resource grouping/filtering by CloudFormation resource type.
- Duplicate resource-ID removal.
- Relationship batching.
- Non-positive batch-size handling.
- Edge deduplication.
- Pointer-to-string extraction.
- DynamoDB stream ARN parsing.
- SQS queue ARN parsing.
- Lambda function ARN parsing.
- Empty/malformed service ARNs.

The ARN tests are important because AWS service ARN resource formats are not uniform.

### Not covered

The suite does not currently execute the complete relationship discovery process against controlled AWS SDK responses. The production code still depends heavily on concrete AWS SDK clients, so a large mock suite would be more work than value at this stage.

---

## 6. AWS snapshot collection and edge normalization

**Tests:** `aws/snapshot_test.go`, `aws/collector_test.go`

Covered:

- Conversion of an AWS response value into a `ResourceConfig`.
- Preservation of resource identity during configuration conversion.
- Filtering resources by CloudFormation type.
- Resource indexing by ID.
- Snapshot warning construction.
- Snapshot metadata initialization.
- Initialization of snapshot slices.
- Warning generation when service clients are unavailable.
- Conversion of native relationship IDs into canonical resource ARNs.
- Duplicate/self/missing edge filtering.
- Rejection of ambiguous native IDs.

This gives the current AWS-to-state boundary useful deterministic coverage.

### Important limitation

The collector tests do **not** prove that every EC2/S3/Lambda/DynamoDB/RDS configuration field is correctly captured. They test collector contracts and shared transformation helpers, not every AWS SDK field.

That is intentional. A test for every SDK field would create a large maintenance burden without necessarily improving confidence in InfraResc's behavior.

### Larger architectural gap

The current `ResourceConfig.Properties` representation is still largely based on AWS SDK response structures. A future recovery implementation should introduce:

```text
AWS SDK response
       ↓
normalized portable configuration
       ↓
resource-specific recovery adapter
       ↓
AWS create/update input
```

That boundary will be one of the most important future testing seams.

---

## 7. Authentication and AWS profile configuration

**Tests:** `auth/auth_test.go`

Covered:

- Removing only the requested AWS profile block.
- Creating a profile.
- Replacing an existing profile.
- Updating the region without duplicating the profile.
- Enforcing `0600` permissions on the generated AWS config.
- Rejecting missing/blank profile names and regions.
- Testing `Manager.Status` through the `AuthProvider` interface.

The provider seam is useful because manager behavior can be tested without invoking the real AWS CLI.

### Not covered

The concrete provider still launches the AWS CLI directly. Deterministic tests for:

- `aws login`
- `aws logout`
- `aws sts get-caller-identity`

would benefit from a small command-runner seam.

That seam should be introduced only if those paths become important enough to justify it.

---

## 8. CLI

**Tests:** `cli/commands_test.go`

Covered:

- Registration of the current top-level commands:
  - `auth`
  - `scan`
  - `snapshot`
  - `diff`
  - `recover`
  - `media`
  - `verify`
- `scan --profile/-p`.
- `snapshot --profile/-p`.
- `snapshot --output/-o`.

This is deliberately a smoke-level test. It verifies the public command surface without coupling the suite to Cobra's internal implementation.

### Not covered

- Actual command execution.
- Snapshot file output.
- Recovery planning/execution commands.
- Media commands.
- Diff behavior.
- Verification behavior.

Those should gain tests when the underlying workflows become real.

---

## 9. Local integration pipeline

**Test:** `integration/pipeline_test.go`

The current integration test exercises:

```text
Snapshot
   ↓
JSON serialization
   ↓
JSON deserialization
   ↓
Graph construction with edges
   ↓
Dependency resolution
   ↓
Graph snapshot
```

It verifies that resource/edge information survives the serialization boundary and can subsequently drive graph construction and ordering.

This is currently the most valuable integration test because it crosses package boundaries while remaining deterministic and AWS-free.

### Next integration boundary

Once recovery planning exists, the natural next test should become:

```text
snapshot
   ↓
normalized configuration
   ↓
recovery graph
   ↓
recovery plan
```

That can remain AWS-free and should be added before introducing live recovery tests.

---

## 10. Crypto artifacts and envelopes

**Tests:** `crypto/artifact_test.go`, `crypto/envelope_test.go`

The current suite has focused tests for the **implemented crypto artifact/envelope behavior**:

- Artifact round-trip.
- Artifact metadata.
- Snapshot hash binding.
- Artifact deserialization.
- Empty-payload rejection.
- Unsupported artifact-version rejection.
- Encryption/decryption round-trip.
- Wrong-password rejection.
- Ciphertext tampering rejection.
- Unsupported envelope metadata rejection.
- Empty-password rejection.

These tests validate existing primitives and artifact behavior.

They do **not** attempt to resolve broader cryptographic architecture questions such as:

- canonical serialization policy,
- trusted-key storage,
- key-management policy,
- replay protection,
- end-to-end trust/verification design.

Those concerns remain outside this testing pass.

---

## 11. Live AWS integration

There are currently no tests that intentionally make real AWS calls.

That is appropriate for the default unit/integration suite because live AWS tests would introduce:

- credentials as a test dependency,
- network dependence,
- account-specific state,
- cost,
- nondeterminism,
- potential resource mutation.

The preferred future architecture is:

```text
production AWS client
          │
          ├── small interface seam
          │
controlled test implementation
          ↓
deterministic AWS workflow tests
```

A deliberately selected AWS-compatible environment can also be considered later, but it should not be introduced simply to increase test count.

---

## 12. Recovery, diff, and media

### Recovery

The meaningful recovery pipeline is not fully implemented yet:

```text
Snapshot
   ↓
Normalize configuration
   ↓
Build recovery graph
   ↓
Recoverability analysis
   ↓
Recovery plan
   ↓
Operator approval
   ↓
AWS execution
   ↓
Recovery validation
```

The current tests therefore stop before pretending that recovery exists.

Future tests should cover each stage independently, with the execution layer isolated behind a controllable AWS boundary.

### Diff

The current `diff` workflow is not yet a substantive diff engine.

When implemented, tests should cover:

- added resources,
- removed resources,
- changed configuration,
- changed dependencies,
- deterministic output.

### Media

The media workflow is not yet a substantive persistence/validation system.

When implemented, tests should cover:

- artifact/package creation,
- manifest layout,
- missing/corrupted artifacts,
- inspection,
- validation,
- version/compatibility behavior.

The Linux CI build has non-Windows stubs for the Windows-specific external-media functions so that the CLI package remains cross-platform compilable. Those stubs are compatibility behavior, not a replacement for media workflow tests.

---

## 13. Architecture seams: current assessment

### Strong seams

```text
Auth Manager
      ↓
AuthProvider interface

State
      ↓
serialization

State
      ↓
Graph / BuildWithEdges

AWS helpers
      ↓
state transformation

CLI
      ↓
runtime / AWS collector
```

These seams are already sufficient for meaningful:

- unit tests,
- state/graph integration tests,
- authentication/config tests,
- deterministic AWS transformation tests.

### Weak seam

The AWS discovery, relationship, and snapshot layers still depend substantially on concrete AWS SDK clients.

That makes this easy:

```text
AWS-free data
      ↓
transformation
      ↓
state
      ↓
graph
```

but makes this harder:

```text
controlled AWS response
      ↓
AWS API workflow
      ↓
relationships
      ↓
snapshot
      ↓
recovery
```

The correct response is **not** to add a huge mocking framework. Introduce small interfaces only at the AWS boundaries that become important to deterministic integration tests.

---

## 14. Current testing backlog

### Highest priority when the corresponding implementation lands

1. Snapshot edge semantics → recovery dependency semantics.
2. Deterministic recovery ordering.
3. Normalized AWS configuration.
4. Recovery planning.
5. Recoverability analysis.
6. Recovery execution through a controlled AWS seam.
7. Recovery validation.
8. Diff engine.
9. Media artifact creation/inspection/validation.
10. Schema/version migration.

### Useful later

- AWS API pagination/error-path tests.
- CLI behavior tests for implemented commands.
- Runtime integration tests.
- Cross-platform tests for platform-specific functionality.

### Avoid unless justified

- One test for every AWS SDK field.
- Tests that merely restate standard-library behavior.
- Tests for trivial getters/setters.
- Large mock hierarchies that reproduce AWS SDK behavior.
- Coverage-driven tests with no meaningful behavioral contract.

---

## 15. CI baseline

The repository's Go test workflow runs:

```text
go test ./...
```

using Go **1.27.1** on `ubuntu-latest`.

The complete suite has been verified by GitHub Actions after the current testing changes.

No claim is made here about live AWS recovery testing; CI is intentionally AWS-credential-free.

---

## 16. Overall assessment

The current suite is a **focused behavioral regression suite**, not a full end-to-end recovery test system.

It provides meaningful protection for the parts of InfraResc that are currently implemented:

- graph construction and ordering,
- snapshot/state persistence,
- AWS identity and relationship transformations,
- snapshot collector contracts,
- authentication/configuration,
- CLI wiring,
- local cross-package state → graph behavior,
- implemented artifact/envelope behavior.

Its main limitation is not "too few tests." The larger limitation is that several important product-level contracts do not exist yet:

```text
portable recovery configuration
            ↓
recovery semantics
            ↓
recovery planner
            ↓
AWS execution
            ↓
validation
```

Those should become the next major testing targets as the implementation matures.

> **Testing objective:** keep the current suite small, deterministic, and behavior-focused, while expanding it at the architectural boundaries that become real parts of the recovery system.
