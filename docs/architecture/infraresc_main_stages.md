# InfraResc --- Main Program Stages

## 1. Overview

InfraResc is designed as an end-to-end cloud infrastructure recovery
system. The complete lifecycle is divided into **six major user-facing
workflows**, supported by a foundational **Authentication &
Configuration layer**.

The overall implementation order is:

``` text
AUTH / CONFIG
      ↓
DISCOVERY
      ↓
SNAPSHOT
      ↓
GRAPH + SECURITY
      ↓
RECOVERY MEDIA
      ↓
VERIFY / DIFF
      ↓
RECOVERY
```

The recommended engineering order is:

``` text
Auth → Discovery → State/Snapshot → Dependency Graph
→ Cryptography → Media → Verification → Recovery
```

------------------------------------------------------------------------

# 2. Stage Model

## Stage 0 --- Authentication & Configuration

### Purpose

Establish the AWS identity and runtime configuration InfraResc will use
for discovery, snapshotting, verification, and recovery.

### Responsibilities

-   Authenticate with AWS.
-   Resolve the active AWS account.
-   Resolve the active/default region.
-   Load InfraResc configuration.
-   Configure filters and output locations.
-   Establish the security/key configuration required by later stages.

### Potential CLI

``` bash
infraresc login
infraresc config
```

### Workflow

``` text
User
  ↓
InfraResc CLI
  ↓
AWS authentication
  ↓
Resolve AWS account / region
  ↓
Load InfraResc configuration
  ↓
Runtime context
```

### Important Design Principle

InfraResc should preferably use the AWS SDK's credential/provider chain
rather than storing long-lived AWS access keys itself.

------------------------------------------------------------------------

# 3. Stage 1 --- Discovery

## Purpose

Discover the infrastructure currently existing in the target AWS
environment.

### Responsibilities

-   Discover supported AWS resources.
-   Query AWS APIs.
-   Collect resource metadata.
-   Collect relevant configuration.
-   Identify relationships/dependencies.
-   Normalize AWS-specific responses into InfraResc's portable resource
    model.

### Current Repository Location

``` text
aws/
├── client.go
├── collector.go
├── discovery.go
└── mapper.go
```

### Workflow

``` text
AWS Account
    ↓
AWS SDK
    ↓
Service Collectors
    ↓
Raw AWS Resources
    ↓
Resource Mapper
    ↓
state.Resource[]
```

### Output

``` go
type Resource struct {
    ID            string
    Type          string
    Name          string
    Region        string
    Configuration map[string]any
    Dependencies  []string
}
```

### Initial Implementation Strategy

Do not attempt to support every AWS service immediately.

Start with a small, dependency-rich vertical slice such as:

``` text
VPC
 ├── Subnet
 ├── Route Table
 ├── Security Group
 └── EC2
```

Once the complete workflow works, additional AWS resource collectors can
be added incrementally.

------------------------------------------------------------------------

# 4. Stage 2 --- Snapshot

## Purpose

Convert discovered infrastructure into a portable, reproducible
representation.

### Responsibilities

-   Normalize discovered resources.
-   Generate snapshot metadata.
-   Serialize resources.
-   Store account and region information.
-   Produce a versioned snapshot format.
-   Ensure deterministic/canonical serialization where cryptographic
    integrity is required.

### Current Repository Location

``` text
state/
├── manifest.go
├── resource.go
├── serialization.go
└── snapshot.go
```

### Workflow

``` text
Discovered Resources
        ↓
Normalization
        ↓
Canonical Representation
        ↓
Snapshot
        ↓
Serialized Snapshot
```

### Snapshot Model

The current model contains:

``` text
Snapshot
├── ID
├── CreatedAt
├── AccountID
├── Region
└── Resources[]
```

### Recommended Extension

The snapshot format should be explicitly versioned before it becomes a
long-lived recovery format.

Example:

``` go
type Snapshot struct {
    SchemaVersion string     `json:"schema_version"`
    ID            string     `json:"id"`
    CreatedAt     time.Time  `json:"created_at"`
    Provider      string     `json:"provider"`
    AccountID     string     `json:"account_id"`
    Region        string     `json:"region"`
    Resources     []Resource `json:"resources"`
}
```

### Important Design Principle

Discovery answers:

> What exists right now?

Snapshot answers:

> What exact infrastructure state are we preserving?

------------------------------------------------------------------------

# 5. Stage 3 --- Dependency Graph & Security

This stage contains two closely related internal workflows:

1.  Dependency analysis
2.  Snapshot security

------------------------------------------------------------------------

## 5.1 Dependency Graph

### Purpose

Determine the order in which resources must be recreated during
recovery.

### Current Repository Location

``` text
graph/
├── graph.go
├── dependency.go
└── resolver.go
```

### Workflow

``` text
Snapshot Resources
       ↓
Dependency Extraction
       ↓
Graph Construction
       ↓
Cycle Detection
       ↓
Dependency Resolution
       ↓
Recovery Order
```

### Example

Given:

``` text
EC2
 ↓
Subnet
 ↓
VPC
```

The recovery order must become:

``` text
VPC
 ↓
Subnet
 ↓
EC2
```

### Current Graph Capabilities

The graph resolver already provides the basic mechanisms for:

-   Building nodes.
-   Tracking dependencies.
-   Detecting dependency cycles.
-   Detecting missing dependencies.
-   Producing a dependency-resolved order.

### Required Improvement

Recovery ordering should eventually be deterministic.

Avoid relying directly on Go map iteration order:

``` go
for id := range g.Nodes
```

because map iteration is not deterministic.

A stable ordering should be introduced for reproducible plans and
testing.

------------------------------------------------------------------------

# 6. Stage 3.2 --- Security & Integrity

## Purpose

Ensure recovery artifacts cannot silently be modified or corrupted.

### Current Repository Location

``` text
crypto/
├── encryption.go
├── hashing.go
├── signature.go
└── tests
```

### Workflow

``` text
Snapshot
   ↓
Canonical Serialization
   ↓
Hash
   ↓
Digital Signature
   ↓
Encryption
   ↓
Protected Artifact
```

### Security Goals

-   Confidentiality
-   Integrity
-   Authenticity
-   Tamper detection
-   Verification before recovery

### Manifest

The current manifest model contains:

``` text
Manifest
├── Version
├── Snapshot
├── Algorithm
├── Files
└── RootHash
```

### Recommended Integrity Model

The final recovery artifact should establish a verifiable chain:

``` text
Manifest
   │
   ├── Snapshot metadata
   ├── File hashes
   ├── Root hash
   ├── Algorithm metadata
   └── Digital signature
```

Recovery must verify the artifact before trusting its contents.

------------------------------------------------------------------------

# 7. Stage 4 --- Recovery Media

## Purpose

Package the protected infrastructure snapshot into portable physical
recovery media.

### Concept

InfraResc is intended to support offline/air-gapped recovery scenarios
where the recovery artifact may be stored independently of the original
cloud environment.

### Supported Media Concept

``` text
USB Drive
External HDD
Optical Media
Other Portable Storage
```

### Workflow

``` text
Secured Snapshot
       ↓
Package
       ↓
Generate Manifest
       ↓
Write Artifact
       ↓
Recovery Media
```

### Example Media Structure

``` text
infraresc-media/
├── manifest.json
├── snapshot.enc
├── hashes/
├── signature
├── metadata.json
└── README.txt
```

### Media Commands

``` bash
infraresc media create
infraresc media inspect
infraresc media validate
```

### Design Requirements

Media creation should be:

-   Self-describing.
-   Verifiable.
-   Portable.
-   Versioned.
-   Resistant to partial/corrupted writes.
-   Independent of the original machine where practical.

------------------------------------------------------------------------

# 8. Stage 5 --- Verification & Diff

This stage provides independent validation of stored infrastructure
state.

------------------------------------------------------------------------

## 8.1 Verification

### Purpose

Confirm that a snapshot or recovery medium is trustworthy before
recovery.

### Workflow

``` text
Recovery Media
      ↓
Read Manifest
      ↓
Verify File Hashes
      ↓
Verify Root Hash
      ↓
Verify Digital Signature
      ↓
Validate Encryption Metadata
      ↓
Artifact Accepted / Rejected
```

### Command

``` bash
infraresc verify
```

### Verification Principle

Recovery should never begin merely because a recovery artifact can be
read.

The artifact should first pass integrity and authenticity checks.

------------------------------------------------------------------------

## 8.2 Diff

### Purpose

Compare two infrastructure states and identify changes.

### Workflow

``` text
Snapshot A
    │
    ├──────────────┐
    │              │
    ▼              ▼
Normalize      Normalize
    │              │
    └──────┬───────┘
           ↓
        Compare
           ↓
    ┌──────┼────────┐
    ↓      ↓        ↓
 Added  Removed  Changed
```

### Command

``` bash
infraresc diff
```

### Example Output Categories

``` text
Added resources
Removed resources
Modified resources
Configuration changes
Dependency changes
```

Diff should operate on the portable snapshot representation rather than
directly comparing raw AWS SDK responses.

------------------------------------------------------------------------

# 9. Stage 6 --- Recovery

## Purpose

Recreate the infrastructure represented by a verified snapshot.

Recovery is deliberately divided into **planning** and **execution**.

------------------------------------------------------------------------

## 9.1 Recovery Plan

### Command

``` bash
infraresc recover plan
```

### Workflow

``` text
Recovery Media
      ↓
Verify
      ↓
Decrypt
      ↓
Deserialize
      ↓
Load Snapshot
      ↓
Build Dependency Graph
      ↓
Resolve Recovery Order
      ↓
Generate Recovery Plan
```

### Example

``` text
1. Create VPC
2. Create Subnet
3. Create Route Table
4. Create Security Group
5. Create EC2
```

The plan should clearly show what InfraResc intends to create, modify,
skip, or report as unresolved.

------------------------------------------------------------------------

# 10. Recovery Execution

## Command

``` bash
infraresc recover execute
```

### Workflow

``` text
Recovery Plan
      ↓
Validate Target Environment
      ↓
Validate Permissions
      ↓
Validate Dependencies
      ↓
Human Confirmation
      ↓
Execute Resources in Dependency Order
      ↓
Capture Results
      ↓
Validate Recovered Infrastructure
```

### Important Safety Principle

`recover plan` and `recover execute` should remain separate.

The plan is a reviewable artifact.

Execution should require an explicit user action and should not silently
modify infrastructure merely because a recovery artifact was supplied.

------------------------------------------------------------------------

# 11. Complete InfraResc Architecture

``` text
                         ┌─────────────────────────┐
                         │      InfraResc CLI       │
                         │        Cobra             │
                         └────────────┬────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    │                 │                 │
                    ▼                 ▼                 ▼
              AUTH / CONFIG      DISCOVERY          WORKFLOWS
                    │                 │                 │
                    │                 ▼                 │
                    │          ┌─────────────┐          │
                    │          │ AWS Client  │          │
                    │          └──────┬──────┘          │
                    │                 │                 │
                    │          Service Collectors      │
                    │                 │                 │
                    │                 ▼                 │
                    │          Resource Mapper         │
                    │                 │                 │
                    │                 ▼                 │
                    │          state.Resource[]        │
                    │                 │                 │
                    │                 ▼                 │
                    │             SNAPSHOT             │
                    │                 │                 │
                    │        ┌────────┴────────┐        │
                    │        ▼                 ▼        │
                    │      GRAPH             CRYPTO     │
                    │        │                 │        │
                    │        ▼                 ▼        │
                    │ Recovery Order     Hash/Sign/     │
                    │                    Encrypt        │
                    │        │                 │        │
                    │        └────────┬────────┘        │
                    │                 ▼                 │
                    │           MEDIA MANAGEMENT        │
                    │                 │                 │
                    │                 ▼                 │
                    │        Physical Recovery Media    │
                    │                 │                 │
                    │                 ▼                 │
                    │          VERIFY / DIFF             │
                    │                 │                 │
                    │                 ▼                 │
                    │             RECOVERY              │
                    │                 │                 │
                    │       ┌─────────┴─────────┐       │
                    │       ▼                   ▼       │
                    │     PLAN                EXECUTE   │
                    │       │                   │       │
                    └───────┴───────────────────┴───────┘
```

------------------------------------------------------------------------

# 12. End-to-End Lifecycle

The complete InfraResc lifecycle is:

``` text
┌──────────────────────┐
│ 0. AUTH / CONFIG     │
└──────────┬───────────┘
           ↓
┌──────────────────────┐
│ 1. DISCOVER          │
│ AWS infrastructure   │
└──────────┬───────────┘
           ↓
┌──────────────────────┐
│ 2. SNAPSHOT          │
│ Portable state       │
└──────────┬───────────┘
           ↓
┌──────────────────────┐
│ 3. GRAPH + SECURE    │
│ Dependencies + crypto│
└──────────┬───────────┘
           ↓
┌──────────────────────┐
│ 4. MEDIA             │
│ Package + store      │
└──────────┬───────────┘
           ↓
┌──────────────────────┐
│ 5. VERIFY / DIFF     │
│ Integrity + compare  │
└──────────┬───────────┘
           ↓
┌──────────────────────┐
│ 6. RECOVER           │
│ Plan → Execute       │
└──────────────────────┘
```

------------------------------------------------------------------------

# 13. CLI-to-Architecture Mapping

  CLI                           Primary Stage   Responsibility
  ----------------------------- --------------- --------------------------------
  `infraresc login`             Stage 0         AWS authentication
  `infraresc config`            Stage 0         InfraResc configuration
  `infraresc scan`              Stage 1         Discover infrastructure
  `infraresc snapshot`          Stage 2         Create infrastructure snapshot
  `infraresc diff`              Stage 5         Compare snapshots
  `infraresc verify`            Stage 5         Verify integrity/authenticity
  `infraresc media create`      Stage 4         Create recovery media
  `infraresc media inspect`     Stage 4         Inspect media
  `infraresc media validate`    Stage 5         Validate media
  `infraresc recover plan`      Stage 6         Generate recovery plan
  `infraresc recover execute`   Stage 6         Execute recovery

------------------------------------------------------------------------

# 14. Recommended Implementation Roadmap

The project should be implemented as vertical slices rather than
completing every package independently.

## Phase 1 --- Foundation

``` text
Auth / Config
    ↓
AWS Client
    ↓
Account / Region Context
```

## Phase 2 --- First Discovery Slice

Implement:

``` text
VPC
Subnet
Route Table
Security Group
EC2
```

Produce:

``` text
[]state.Resource
```

## Phase 3 --- Snapshot

Implement:

``` text
Discovery
    ↓
Normalization
    ↓
Versioned Snapshot
    ↓
Canonical Serialization
```

## Phase 4 --- Recovery Graph

Implement:

``` text
Resources
    ↓
Dependencies
    ↓
Graph
    ↓
Deterministic Recovery Order
```

## Phase 5 --- Cryptographic Protection

Implement:

``` text
Snapshot
    ↓
Hash
    ↓
Sign
    ↓
Encrypt
    ↓
Manifest
```

## Phase 6 --- Media

Implement:

``` text
Package
    ↓
Write
    ↓
Inspect
    ↓
Validate
```

## Phase 7 --- Verification

Implement the complete verification chain before allowing recovery.

## Phase 8 --- Recovery Planning

Generate a human-readable, deterministic recovery plan.

## Phase 9 --- Recovery Execution

Execute the plan against a target AWS environment and validate the
resulting infrastructure.

------------------------------------------------------------------------

# 15. Core Engineering Principle

InfraResc should not be built as:

``` text
"Add AWS service → write recovery code → repeat"
```

Instead, it should establish a complete recovery pipeline first:

``` text
DISCOVER
   ↓
SNAPSHOT
   ↓
GRAPH
   ↓
SECURE
   ↓
STORE
   ↓
VERIFY
   ↓
PLAN
   ↓
RECOVER
   ↓
VALIDATE
```

Once this pipeline works for a small resource set, additional AWS
services become implementations of the existing discovery/state/recovery
interfaces rather than new architectural problems.

This complete vertical slice is the foundation of the project.
