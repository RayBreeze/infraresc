# Proposed Recovery Flow

> **Status: Proposed — requires team review before implementation**
>
> This document defines the proposed technical workflow for the InfraResc recovery subsystem. It is intentionally written as a design proposal rather than a statement that all described functionality already exists in the repository.

## 1. Objective

The recovery subsystem should take a verified InfraResc snapshot and reconstruct the supported AWS infrastructure represented by that snapshot.

The recovery process must:

* operate from the stored snapshot rather than requiring the original infrastructure to still exist;
* understand resource dependencies;
* detect resources or dependencies that cannot be recreated;
* avoid blindly creating resources;
* provide a human-readable recovery plan before mutation;
* create resources in dependency-safe order;
* maintain a mapping between snapshot identities and newly created AWS IDs;
* validate the resulting infrastructure;
* report partial failures explicitly.

The intended high-level flow is:

```text
                Recovery Artifact
                       |
                       v
              Load Snapshot
                       |
                       v
              Verify Integrity
                       |
                       v
          Verify Manifest / Signature
                       |
                       v
             Build State Model
                       |
                       v
            Build Dependency Graph
                       |
                       v
          Resolve Recovery Ordering
                       |
                       v
        Check Resource Recoverability
                       |
                       v
              Generate Plan
                       |
                 Operator Review
                       |
              +--------+--------+
              |                 |
            Abort             Approve
                                |
                                v
                         Execute Plan
                                |
                                v
                      Track ID Mappings
                                |
                                v
                       Validate Resources
                                |
                                v
                     Validate Dependencies
                                |
                                v
                         Recovery Report
```

---

# 2. Recovery Inputs

The recovery engine should consume a verified snapshot and the information necessary to interpret it.

### Required logical inputs

```text
Snapshot
  |
  +-- Snapshot metadata
  +-- Resource definitions
  +-- Resource configurations
  +-- Resource dependencies
  +-- Region/account information
```

The current `state.Snapshot` model contains:

* `ID`
* `CreatedAt`
* `AccountID`
* `Region`
* `Resources`

Each `state.Resource` contains:

* `ID`
* `Type`
* `Name`
* `Region`
* `Configuration`
* `Dependencies`

These models form the current basis for the recovery design.

---

# 3. Phase 1 — Load and Verify the Recovery Artifact

Recovery must not operate on unverified state.

The first stage should:

1. Locate the selected recovery artifact.
2. Read its manifest and snapshot metadata.
3. Validate the artifact structure.
4. Verify resource/file hashes.
5. Recompute and compare the snapshot root hash.
6. Verify the manifest signature against the trusted public key.
7. Decrypt the snapshot if encryption is enabled.
8. Deserialize the snapshot.
9. Reject the recovery operation if verification fails.

Conceptually:

```text
Recovery Media
      |
      v
Structural Validation
      |
      v
Hash Verification
      |
      v
Signature Verification
      |
      v
Decryption
      |
      v
Snapshot
```

A failed integrity or authenticity check should stop recovery rather than allow potentially corrupted state to drive resource creation.

The current cryptographic package already provides the primitives required for much of this stage:

* AES-256-GCM;
* Argon2id;
* SHA-256;
* Merkle-style root hashing;
* Ed25519 signatures.

The exact artifact format and verification orchestration still need to be finalized.

---

# 4. Phase 2 — Build the Recovery State

Once the snapshot has been verified and decoded, the recovery subsystem should construct an in-memory representation of the resources.

Example:

```text
Resource
  |
  +-- ID: ec2:i-123
  +-- Type: EC2
  +-- Region: ap-south-1
  +-- Configuration:
  |     instance type
  |     AMI
  |     subnet
  |     security groups
  |
  +-- Dependencies:
        subnet-123
        sg-456
```

At this stage the system should validate basic state consistency:

* resource IDs are unique;
* required fields are present;
* resource types are supported;
* dependency references resolve;
* resource regions are valid;
* configurations contain the information required by the resource adapter.

Malformed state should prevent plan generation.

---

# 5. Phase 3 — Build the Dependency Graph

The recovery engine should convert the snapshot's resource dependencies into a directed graph.

For example:

```text
VPC
 |
 +--> Subnet
 |      |
 |      +--> EC2
 |
 +--> Route Table
 |
 +--> Security Group
```

Another example:

```text
EC2
 |
 +--> Target Group
          |
          +--> Load Balancer
                    |
                    +--> DNS
```

The graph represents **creation prerequisites**, not merely descriptive relationships.

If resource B requires resource A to exist before B can be created, the graph must represent:

```text
A -> B
```

The current graph implementation already supports dependency traversal and detects:

* missing dependencies;
* dependency cycles.

The recovery planner should use this graph to derive a deterministic execution order.

---

# 6. Phase 4 — Resolve Recovery Order

The graph should be topologically ordered before execution.

A simplified order may look like:

```text
1. VPC
2. Internet Gateway
3. Subnets
4. Route Tables
5. Security Groups
6. IAM Roles
7. Compute
8. Target Groups
9. Load Balancers
10. DNS
```

This is an illustrative ordering only. The final order must be derived from the actual dependency relationships of the supported AWS resource types.

The implementation must not hard-code one universal ordering when the graph itself can express the actual prerequisites.

---

# 7. Phase 5 — Recoverability Analysis

Before generating executable actions, the planner should determine whether each resource can actually be reconstructed.

This is a critical stage.

A snapshot may contain configuration for a resource whose external prerequisite is no longer available.

For example:

```text
EC2
 |
 +--> AMI
       |
       +--> unavailable
```

The planner should report:

```text
RECOVERY BLOCKED

Resource: EC2 / web-server
Required dependency: AMI
Required ID: ami-123456
Status: unavailable

Affected resources:
- web-server-1
- web-server-2
```

The system should not silently skip the dependency and claim successful recovery.

Potential recoverability statuses include:

* `READY`
* `WARNING`
* `BLOCKED`
* `UNSUPPORTED`

The exact enum and semantics should be finalized during implementation.

---

# 8. Phase 6 — Generate the Recovery Plan

The planner should transform the dependency graph and recoverability analysis into an explicit plan.

Conceptually:

```text
RecoveryPlan
 |
 +-- Snapshot ID
 +-- Target account/region
 +-- Actions
 +-- Dependency order
 +-- Warnings
 +-- Blocking errors
 +-- Resource mappings
```

An action should conceptually describe:

```text
Action
 |
 +-- logical resource
 +-- resource type
 +-- operation
 +-- dependencies
 +-- configuration
 +-- expected outputs
 +-- warnings
```

Example:

```text
RECOVERY PLAN

[1] CREATE VPC
    logical ID: vpc-main

[2] CREATE SUBNET
    logical ID: subnet-public
    depends on: vpc-main

[3] CREATE SECURITY GROUP
    logical ID: sg-web
    depends on: vpc-main

[4] CREATE EC2
    logical ID: web-server
    depends on:
      subnet-public
      sg-web

[5] CREATE LOAD BALANCER
    logical ID: public-alb
    depends on:
      subnet-public
      sg-web

Warnings:
  - Original EC2 ID cannot be preserved.
  - AMI availability must be confirmed.

No AWS resources modified.
```

---

# 9. Dry-Run Requirement

The planner should be usable without modifying AWS.

The intended CLI interaction is:

```bash
infraresc recover plan
```

The command should:

1. load the selected snapshot;
2. verify it;
3. construct the graph;
4. analyze recoverability;
5. produce the plan;
6. display warnings/errors;
7. perform no AWS mutations.

This gives the operator an opportunity to inspect the proposed recovery before execution.

The exact CLI syntax may change as the command structure evolves; the important design requirement is that **planning and execution are separate operations**.

---

# 10. Phase 7 — Operator Approval

Execution should require an explicit operator decision.

```text
Recovery Plan
     |
     v
Review
     |
  +--+--+
  |     |
Abort  Approve
        |
        v
     Execute
```

The system should not interpret the existence of a valid plan as authorization to modify AWS.

Whether approval is provided through a CLI confirmation prompt, a flag, or another mechanism is an implementation decision still to be finalized.

---

# 11. Phase 8 — Execute the Plan

The executor should process recovery actions in dependency order.

For each action:

```text
Plan Action
    |
    v
Resolve dependency IDs
    |
    v
Construct AWS API request
    |
    v
Call AWS API
    |
    v
Capture returned resource ID
    |
    v
Store logical -> physical mapping
    |
    v
Continue
```

The executor should use AWS SDK service clients rather than Terraform or CloudFormation.

For example:

```text
Recovery Executor
      |
      +--> EC2 API
      +--> VPC API
      +--> RDS API
      +--> S3 API
      +--> IAM API
      +--> ELB API
      +--> Route 53 API
      +--> ...
```

The exact service clients and operations should be implemented behind resource-specific recovery adapters rather than placing all AWS API logic inside one large executor.

---

# 12. Logical-to-Physical Resource Mapping

This is a central recovery requirement.

AWS resource IDs are provider-assigned and may change after recovery.

Example:

```text
Snapshot:

logical resource: web-server
original ID: i-012345
```

After recovery:

```text
logical resource: web-server
new AWS ID: i-098765
```

The recovery engine should maintain:

```text
Logical Resource ID
        |
        v
Original Resource ID
        |
        v
Recovered Resource ID
```

For example:

```text
web-server -> i-012345 -> i-098765
web-sg     -> sg-123    -> sg-789
web-subnet -> subnet-1  -> subnet-9
```

When a later resource refers to `web-sg`, the executor should resolve that logical reference to the newly created `sg-789`.

This prevents the recovery engine from incorrectly sending stale snapshot IDs to AWS.

---

# 13. Handling Partial Failure

Recovery of a large environment may not be atomic.

For example:

```text
VPC              SUCCESS
Subnet           SUCCESS
Security Group   SUCCESS
EC2              FAILED
RDS              SUCCESS
ALB              BLOCKED
DNS              BLOCKED
```

The executor must record the result of every action.

A recovery report should distinguish at least:

* successfully created;
* already present;
* failed;
* skipped;
* blocked by dependency;
* unsupported.

The system should never report the overall recovery as successful if required resources remain failed or blocked.

The exact rollback policy is an open design question. The MVP may choose to report partial recovery without automatically deleting successfully created resources, provided that behavior is explicitly documented.

---

# 14. Idempotency and Existing Resources

A recovery attempt may encounter resources that already exist.

The executor therefore needs a policy for:

* resource already exists;
* matching resource exists;
* resource exists but configuration differs;
* resource was created by an earlier interrupted recovery.

Possible operations include:

```text
CREATE
REUSE
UPDATE
SKIP
FAIL
```

The exact policy must be agreed before implementing the executor.

For the MVP, a conservative approach is preferable: **do not silently overwrite unrelated existing resources**.

---

# 15. Phase 9 — Post-Recovery Validation

Successful AWS API calls do not necessarily mean the recovered application topology is correct.

Validation should occur after execution.

The validator should check:

### Resource existence

Does each required resource exist?

### Configuration

Does the recovered resource match the recoverable configuration from the snapshot?

### Dependency relationships

Are references connected to the correct recovered resources?

### Network relationships

Are subnets, route tables, gateways and security groups associated correctly?

### Compute relationships

Are instances associated with the intended subnet and security groups?

### Load balancing

Are target groups and load balancers configured with the expected relationships?

### DNS

Do recovered DNS records point to the recovered endpoints where applicable?

The validation result should distinguish:

```text
VALID
INVALID
WARNING
UNVERIFIABLE
```

Some properties may not be exactly reproducible or externally verifiable and should therefore be reported rather than treated as silent success.

---

# 16. Final Recovery Report

The recovery process should produce a final machine-readable and human-readable result.

Example:

```text
INFRARESC RECOVERY REPORT

Snapshot:
  cv-2026-09-16-001

Status:
  PARTIAL SUCCESS

Resources:
  Created:    14
  Reused:      2
  Failed:      1
  Blocked:     3
  Skipped:     0

Validation:
  Passed:      12
  Warnings:     2
  Failed:       1

Blocking issue:
  Original AMI unavailable.

Affected resources:
  - web-server-1
  - web-server-2
```

The important property is that the system reports the actual recovery state instead of reducing the result to a single success/failure value.

---

# 17. Proposed Recovery Architecture

The recovery subsystem should be separated into three primary responsibilities.

```text
                 Snapshot
                    |
                    v
             +--------------+
             |    Planner   |
             +--------------+
                    |
                    v
             Recovery Plan
                    |
                    v
             +--------------+
             |   Executor   |
             +--------------+
                    |
                    v
              AWS Resources
                    |
                    v
             +--------------+
             |  Validator   |
             +--------------+
                    |
                    v
            Recovery Report
```

### Planner

Responsible for:

* loading recovery state;
* validating state;
* building dependency graph;
* resolving order;
* checking recoverability;
* generating actions;
* producing warnings and blockers.

### Executor

Responsible for:

* receiving an approved plan;
* resolving logical resource references;
* invoking AWS APIs;
* recording returned resource IDs;
* tracking action results;
* handling failures.

### Validator

Responsible for:

* querying recovered AWS resources;
* comparing them against expected state;
* checking relationships;
* identifying mismatches;
* producing the final recovery result.

---

# 18. Resource Adapter Model

As the number of supported AWS resource types grows, the recovery implementation should avoid a monolithic executor containing service-specific logic for every resource.

A possible abstraction is:

```text
Recovery Executor
       |
       v
Resource Adapter
       |
 +-----+------+------+
 |            |      |
 EC2         VPC    RDS
 Adapter     Adapter Adapter
 |            |      |
 v            v      v
AWS SDK      AWS SDK AWS SDK
```

A resource adapter would conceptually know:

* how to validate the resource configuration;
* what prerequisites it requires;
* how to construct the AWS create request;
* how to resolve dependency references;
* how to capture the resulting AWS ID;
* how to validate the recovered resource.

The exact Go interfaces should be defined before implementation.

---

# 19. Dependency and Identity Model

The recovery system therefore has two related but distinct concepts:

### Dependency

Answers:

> "What must exist before this resource can be recovered?"

### Identity

Answers:

> "Which recovered resource represents this logical resource?"

Example:

```text
Logical resource: web-server

Dependencies:
  subnet-public
  sg-web
  ami-base

Identity mapping:
  web-server
      |
      +-- original: i-012345
      +-- recovered: i-098765
```

Keeping these concepts separate will make the recovery engine easier to reason about.

---

# 20. Failure Cases the Planner Should Detect

The planner should explicitly account for cases including:

### Missing dependency

```text
EC2 -> subnet-x

subnet-x not present in snapshot
```

Result: blocked.

### Dependency cycle

```text
A -> B
B -> A
```

Result: invalid recovery graph.

### Unsupported resource type

```text
Resource type: SomeUnsupportedService
```

Result: unsupported/warning or blocker depending on whether other resources depend on it.

### Missing external artifact

Examples include:

* unavailable AMI;
* unavailable database backup;
* unavailable referenced object;
* unavailable certificate;
* unavailable provider-managed resource.

Result: warning or blocker depending on whether recovery can proceed.

### Existing conflicting resource

A resource with the same logical role may already exist but have incompatible configuration.

Result: require explicit handling rather than silently overwriting it.

### AWS API failure

Examples:

* insufficient permissions;
* quota exceeded;
* service unavailable;
* invalid configuration.

Result: failed action with diagnostic information and dependent actions blocked as appropriate.

---

# 21. Security Requirements During Recovery

The recovery artifact must not contain long-lived AWS credentials.

Recovery credentials should be supplied through the normal AWS authentication mechanisms available to the recovery workstation/environment.

The intended security boundary is:

```text
Offline Artifact
     |
     | encrypted / signed
     v
Recovery Workstation
     |
     | authenticated AWS session
     v
AWS APIs
```

The recovery executor should request only the permissions required for the supported recovery operations.

The proposal distinguishes read-oriented scanning from write-oriented recovery permissions; this separation should be retained in the final implementation.

---

# 22. CLI-Level Recovery Flow

The intended user experience is:

```bash
# 1. Inspect available recovery state
infraresc media inspect

# 2. Verify the artifact
infraresc verify

# 3. Generate a plan
infraresc recover plan

# 4. Review warnings and blockers

# 5. Explicitly execute
infraresc recover execute

# 6. Review recovery/validation report
```

The exact flags and arguments remain subject to implementation.

The important design constraint is that **plan generation must not modify AWS**.

---

# 23. End-to-End Example

Consider:

```text
              Route 53
                  |
                  v
                 ALB
              /       \
             v         v
           EC2       EC2
             \       /
               RDS
                |
                S3
```

### Before disaster

InfraResc discovers the environment and creates:

```text
Snapshot
  |
  +-- Resources
  +-- Dependencies
  +-- Configuration
  +-- Integrity metadata
```

The protected snapshot is placed on the recovery medium.

### Disaster

The AWS resources are deleted or become unavailable.

### Recovery

InfraResc loads the snapshot:

```text
Recovery Medium
      |
      v
Verify
      |
      v
Snapshot
      |
      v
Dependency Graph
      |
      v
Recovery Plan
```

The plan identifies:

```text
VPC
  -> Subnets
  -> Security Groups
  -> EC2
  -> ALB
  -> Route 53
```

The operator approves.

The executor creates resources and records new IDs:

```text
logical: web-1
original: i-old
recovered: i-new
```

The validator then checks the reconstructed topology.

---

# 24. MVP Recovery Scope

The recovery implementation should initially focus on a small end-to-end resource chain rather than attempting every AWS service.

The proposed MVP resource scope is:

* VPC
* Subnet
* Route Table
* Internet Gateway
* Security Group
* EC2
* IAM Role
* S3
* RDS
* Application Load Balancer
* Route 53

However, the implementation should be staged.

A practical progression is:

```text
Stage 1
VPC
 |
 +-- Subnet
 |
 +-- Route Table
 |
 +-- Internet Gateway

        ↓

Stage 2
Security Groups
 |
 +-- EC2

        ↓

Stage 3
Target Groups
 |
 +-- ALB

        ↓

Stage 4
S3 / RDS

        ↓

Stage 5
IAM / Route 53
```

The final ordering and exact subset should be confirmed by the team lead.

---

# 25. Implementation Boundaries

The proposed package responsibilities are:

```text
state/
    Defines what infrastructure state looks like.

graph/
    Defines how dependencies are represented and ordered.

aws/
    Discovers AWS state and provides AWS-facing abstractions.

crypto/
    Protects and verifies recovery artifacts.

media/
    Reads/writes/validates the physical recovery artifact.

recovery/
    Converts verified state into a plan,
    executes the plan,
    and validates the result.

cli/
    Exposes these capabilities to the operator.
```

The recovery package should not become responsible for:

* discovering arbitrary AWS resources;
* storing cryptographic secrets;
* implementing the CLI presentation layer;
* directly handling physical-media layout.

Those responsibilities belong to their respective components.

---

# 26. Open Design Decisions for Team Review

The following decisions should be confirmed before the recovery implementation is considered final:

1. **Exact recovery-plan data model**

   * What fields must a plan/action contain?

2. **Resource adapter interface**

   * How will EC2/VPC/RDS/etc. recovery implementations plug into the executor?

3. **Logical resource identity**

   * Is the existing AWS resource ID sufficient as the logical ID, or should the snapshot introduce a stable logical identifier?

4. **ID mapping representation**

   * Where and how should original-to-recovered mappings be persisted?

5. **Existing-resource policy**

   * When a resource already exists, should InfraResc reuse, update, skip or fail?

6. **Partial recovery policy**

   * Should recovery continue after an independent action fails?
   * Should dependent actions automatically become blocked?

7. **Rollback**

   * Does the MVP perform rollback, or only report partial recovery?

8. **External dependencies**

   * Which dependencies are considered recoverable from configuration alone?
   * Which require separate backups or artifacts?

9. **Cross-region recovery**

   * Is the MVP restricted to the original region?

10. **Cross-account recovery**

    * Can a snapshot be recovered into another AWS account?

11. **Approval mechanism**

    * How exactly is operator approval represented in the CLI?

12. **Recovery validation depth**

    * Which resource properties must match exactly?
    * Which properties are allowed to differ?

13. **Snapshot/media format**

    * What exact directory/file structure is stored on the recovery medium?

14. **Credential model**

    * Which AWS authentication mechanism is expected during recovery?

15. **MVP resource order**

    * Which AWS services must be fully recoverable for the hackathon demonstration?

---

# 27. Proposed Final Recovery Pipeline

Subject to the decisions above, the intended implementation can be summarized as:

```text
                    RECOVERY ARTIFACT
                           |
                           v
                  +-------------------+
                  | Verify Artifact   |
                  +-------------------+
                           |
                           v
                  +-------------------+
                  | Load Snapshot     |
                  +-------------------+
                           |
                           v
                  +-------------------+
                  | Validate State    |
                  +-------------------+
                           |
                           v
                  +-------------------+
                  | Build Graph       |
                  +-------------------+
                           |
                           v
                  +-------------------+
                  | Resolve Order     |
                  +-------------------+
                           |
                           v
                  +-------------------+
                  | Recoverability    |
                  | Analysis          |
                  +-------------------+
                           |
                           v
                  +-------------------+
                  | Recovery Planner  |
                  +-------------------+
                           |
                           v
                     RECOVERY PLAN
                           |
                    OPERATOR APPROVAL
                           |
                           v
                  +-------------------+
                  | Recovery Executor |
                  +-------------------+
                           |
                           v
                     AWS APIs
                           |
                           v
                  +-------------------+
                  | Recovery Validator|
                  +-------------------+
                           |
                           v
                  RECOVERY REPORT
```

The primary implementation goal should be a **deterministic, dependency-aware and operator-visible recovery process**, rather than an opaque "restore everything" operation.
