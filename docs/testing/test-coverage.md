# InfraResc Test Coverage

> Status: testing work completed on the `test` branch. This document records what is intentionally covered, what remains untested, and which areas were deliberately left alone.

## 1. Testing approach

The test suite is intentionally **behavior-focused rather than coverage-count-focused**.

The goal of the current tests is to protect the contracts that are already implemented without creating a large mock-heavy suite around unfinished AWS and recovery workflows.

The current testing layers are:

```text
Pure logic
  ├── graph
  └── state serialization

AWS boundary helpers
  ├── resource identity
  ├── relationship helpers
  └── snapshot collector contracts

Authentication/config
  └── provider delegation + AWS config writing

CLI
  └── command registration + important flags

Local integration
  └── snapshot serialization → graph construction
```

The suite deliberately does **not** pretend that placeholder workflows are implemented.

---

## 2. Coverage by component

### 2.1 Graph

**Test file:** `graph/graph_test.go`

Covered:

- Resource-to-node construction.
- Dependency ordering.
- Missing dependency detection.
- Dependency-cycle detection.
- Inclusion of independent nodes.

These tests cover the current graph resolver's core behavioral contract.

**Important limitation:** `graph.Build()` currently receives only `[]state.Resource`. It does not translate `Snapshot.Edges` into `Node.Dependencies`. Therefore the tests do not claim that AWS-discovered relationships currently drive recovery ordering.

**Still needed later:**

- Snapshot edges → graph dependencies.
- Deterministic recovery ordering.
- Recovery-specific dependency semantics once those are defined.

---

### 2.2 State and snapshot serialization

**Test file:** `state/serialization_test.go`

Covered:

- Complete snapshot JSON round-trip.
- Preservation of resource/configuration data through serialization.
- Omission of optional warnings when empty.
- Rejection of invalid JSON.

This protects the current JSON representation without asserting a canonical serialization format that has not yet been implemented.

**Still needed later:**

- Canonical/deterministic serialization.
- Schema/version migration behavior.
- Validation of normalized recovery configuration.

---

### 2.3 AWS discovery

**Test file:** `aws/discovery_test.go`

Covered:

- Resource ID extraction from representative ARNs:
  - Lambda.
  - RDS instance.
  - RDS cluster.
  - DynamoDB table.
  - S3 bucket.
  - slash-based EC2 resources.
  - plain resource identifiers.
  - malformed ARN fallback.
- Nil-safe string extraction.

The tests focus on deterministic transformation logic rather than making live Resource Explorer calls.

**Still needed later:**

- Deterministic API-level discovery tests.
- Pagination behavior against a controllable AWS boundary.
- Handling of representative Resource Explorer responses.

---

### 2.4 AWS relationships

**Test file:** `aws/relationships_test.go`

Covered:

- Resource filtering by type.
- Duplicate ID removal.
- Relationship batching/chunking.
- Edge deduplication.
- String pointer extraction.
- Service-resource ID extraction from ARNs.

These tests cover the pure helper behavior used by relationship discovery.

**Still needed later:**

- API-level relationship discovery using controlled AWS responses.
- End-to-end conversion of discovered relationships into recovery dependencies.
- Explicit distinction between observed relationships and recovery prerequisites.

---

### 2.5 AWS snapshot collection

**Test file:** `aws/snapshot_test.go`

Covered:

- Conversion of collected AWS values into `ResourceConfig`.
- Preservation of resource identity during config conversion.
- Snapshot resource-type extraction.
- Resource map construction.
- Snapshot warning construction.
- Collector behavior when service clients are unavailable, including warning generation and initialized snapshot collections.

The tests intentionally avoid constructing a large fake implementation of every AWS SDK service.

**Still needed later:**

- Controlled service API tests.
- Normalization from raw AWS SDK responses into portable recovery configuration.
- Service-specific recovery adapters.
- Explicit tests for unsupported/external recovery artifacts.

---

### 2.6 Authentication and AWS profile configuration

**Test file:** `auth/auth_test.go`

Covered:

- Removal of only the targeted AWS profile block.
- Creation of an AWS profile.
- Replacement of an existing profile without duplication.
- Region replacement.
- File permission enforcement (`0600`).
- Required-field validation.
- `Manager.Status` delegation through the `AuthProvider` seam.

The provider interface is used where practical, so the manager behavior can be tested without launching the real AWS CLI.

**Still needed later:**

- A command-runner seam if deterministic tests of `aws login/logout/sts` invocation are required.
- End-to-end browser/AWS CLI authentication testing in an environment with AWS CLI credentials.

---

### 2.7 CLI

**Test file:** `cli/commands_test.go`

Covered:

- Registration of the current top-level command groups:
  - `auth`
  - `scan`
  - `snapshot`
  - `diff`
  - `recover`
  - `media`
  - `verify`
- Important `scan` and `snapshot` flags.

This is intentionally a smoke-level test. It checks that the CLI surface is wired together without attempting to test every Cobra implementation detail.

**Still needed later:**

- Command-level tests once commands perform meaningful work.
- File-output tests for `snapshot`.
- Recovery-plan/execute command tests once those workflows exist.
- Media and verification command tests once their underlying functionality exists.

---

### 2.8 Local integration pipeline

**Test file:** `integration/pipeline_test.go`

Covered pipeline:

```text
state.Snapshot
      ↓
JSON serialization
      ↓
JSON deserialization
      ↓
graph.Build()
      ↓
graph.ResolveOrder()
```

The test verifies that a realistic snapshot survives serialization and that its resources can subsequently enter the graph layer.

This is currently the most useful integration test because it crosses package boundaries without requiring AWS credentials, network access, or a heavyweight mock environment.

**Important limitation:** snapshot edges are currently not consumed by `graph.Build()`, so the integration test intentionally does not claim relationship-driven recovery ordering.

---

## 3. What is deliberately not tested

### 3.1 Cryptography

**Deliberately untouched.**

The existing `crypto/` test suite is outside this testing pass. No new crypto gaps were introduced or addressed by these batches.

In particular, this pass does **not** attempt to solve:

- canonical serialization for hashing,
- trusted-key storage,
- replay protection,
- key-management policy,
- end-to-end crypto/media integration.

Those are architectural/application-layer concerns around the existing crypto primitives.

---

### 3.2 Live AWS integration

No tests currently make real AWS calls.

Reasons:

1. They would require credentials and a real AWS environment.
2. Results would depend on external infrastructure.
3. They would be expensive and potentially destructive if extended into recovery.
4. Several AWS components currently depend directly on concrete SDK clients.

A future AWS integration layer should use either controlled service seams or a deliberately selected AWS-compatible test environment rather than scattering mocks throughout the codebase.

---

### 3.3 Recovery

Recovery is not meaningfully implemented yet, so there is no useful recovery-execution test suite to add.

The following remain future work:

```text
Snapshot
   ↓
Normalize configuration
   ↓
Build recovery graph
   ↓
Resolve dependencies
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

Testing should be added as those stages become real, rather than testing placeholder CLI output.

---

### 3.4 Media

`media create`, `media inspect`, and `media validate` currently expose CLI placeholders.

No substantial media tests were added because there is no implemented media format or persistence workflow to test yet.

Once implemented, the important tests should cover:

- package creation,
- manifest/artifact layout,
- corrupted/missing files,
- inspection,
- validation,
- compatibility/version handling.

---

### 3.5 Diff

The current `diff` command is a placeholder.

No substantive diff tests were added.

Once the diff engine operates on normalized snapshots, it should be tested independently of the CLI for:

- added resources,
- removed resources,
- changed configurations,
- dependency changes,
- deterministic output.

---

## 4. Architecture seams for future integration testing

The current architecture is sufficient for **local package-level and state/graph integration testing**, but not yet for deep deterministic AWS integration tests.

### Existing useful seams

```text
Auth Manager
    ↓
AuthProvider interface

Snapshot/state
    ↓
plain Go data structures

State
    ↓
JSON serialization

State
    ↓
Graph

CLI
    ↓
runtime / AWS collector
```

### Current weak seam

The AWS layer largely holds concrete AWS SDK clients.

That makes this easy:

```text
AWS-free data → transformation → state → graph
```

but makes this harder:

```text
controlled AWS response
       ↓
AWS discovery
       ↓
relationships
       ↓
snapshot
       ↓
recovery
```

We should **not** introduce a large mocking framework merely to increase test count.

A better future approach is to introduce small interfaces at the AWS boundaries that actually need deterministic integration tests, when those workflows become important.

---

## 5. Testing backlog

### High priority when implementation lands

1. Snapshot edges → graph dependencies.
2. Deterministic graph/recovery ordering.
3. Normalized AWS configuration.
4. Recovery planning.
5. Recovery execution through a controlled AWS seam.
6. Recovery validation.
7. Diff engine.
8. Media creation/inspection/validation.

### Medium priority

- AWS API pagination/error-path integration tests.
- CLI command behavior around implemented workflows.
- Schema/version migration tests.
- Runtime integration tests.

### Lower priority / avoid unless justified

- Tests for trivial getters/setters.
- Tests that duplicate standard-library behavior.
- One test per AWS SDK field.
- Large mock suites that reproduce the AWS SDK instead of testing InfraResc behavior.

---

## 6. Current test philosophy

The target is not “maximum number of tests.”

The target is:

> **Enough tests to detect regressions in meaningful InfraResc behavior while keeping the suite deterministic, maintainable, and aligned with the implementation actually present in the repository.**

As recovery functionality is implemented, the test suite should grow around the system's architectural contracts rather than around placeholder commands.

---

## 7. Current status summary

| Component | Current tests | Status |
|---|---|---|
| Graph | Core resolver behavior | Covered |
| State | Serialization/round-trip | Covered |
| AWS discovery | Pure identity helpers | Covered |
| AWS relationships | Pure helper behavior | Covered |
| AWS snapshots | Collector contracts/helpers | Covered |
| Auth/config | Provider seam + profile writing | Covered |
| CLI | Command tree + key flags | Smoke covered |
| Local integration | State → graph pipeline | Covered |
| Crypto | Existing suite only | Deliberately untouched |
| Live AWS integration | None | Deliberately deferred |
| Media | None | Not implemented |
| Diff | None | Placeholder |
| Recovery planner | None | Not implemented |
| Recovery executor | None | Not implemented |
| Recovery validator | None | Not implemented |

This document should be updated when a new implemented workflow creates a meaningful testing seam.
