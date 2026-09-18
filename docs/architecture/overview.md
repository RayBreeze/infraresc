# InfraResc — Project Overview

## 1. Purpose

InfraResc is a CLI-first system for creating a portable recovery representation of an AWS infrastructure environment and using that representation to reconstruct supported infrastructure after a destructive incident.

The core idea is to preserve **infrastructure state and relationships**, rather than relying on a particular Infrastructure-as-Code tool as the recovery mechanism.

InfraResc is intended to:

1. Discover the deployed AWS resources in a target environment.
2. Normalize those resources into a common infrastructure-state model.
3. Capture dependencies between resources as a graph.
4. Persist the resulting state as a snapshot.
5. Protect the snapshot with encryption and integrity/authenticity mechanisms.
6. Store or transport the protected snapshot through recovery media.
7. Inspect and compare snapshots.
8. Generate a dependency-aware recovery plan.
9. Execute the approved plan through AWS APIs.
10. Validate the resulting infrastructure.

The system therefore treats the last-known-good infrastructure state as a **portable recovery artifact**.

---

## 2. Problem

A production AWS environment is not a single resource. It is a collection of interconnected resources whose configuration and relationships determine whether the application can operate.

A simplified environment may look like:

```text
                    Route 53
                        |
                        v
                       ALB
                    +---+---+
                    |       |
                    v       v
                  EC2      EC2
                    |
                    v
                   RDS
                    |
                    v
                   S3
```

Recovering such an environment requires more than knowing that these resources existed. The recovery system must also understand relationships such as:

* which subnet belongs to which VPC;
* which route tables are associated with which subnets;
* which security groups are attached to compute resources;
* which load balancer targets which instances;
* which DNS records point to which endpoints;
* which resources must exist before another resource can be created.

InfraResc therefore models both **resource configuration** and **resource dependencies**.

---

## 3. Core Concept

The system can be viewed as a pipeline:

```text
AWS Environment
      |
      v
Resource Discovery
      |
      v
Normalized State
      |
      v
Dependency Graph
      |
      v
Snapshot
      |
      v
Encryption + Integrity Protection
      |
      v
Offline / Portable Recovery Artifact
      |
      v
Verification
      |
      v
Recovery Planning
      |
      v
AWS API Execution
      |
      v
Recovered Infrastructure
      |
      v
Validation
```

AWS APIs remain the source of truth for observing and manipulating the actual cloud environment. The stored artifact is a portable representation of the observed infrastructure state.

---

## 4. Architecture

The repository is currently organized around the following logical components:

```text
infraresc/
|
+-- cli/          Command-line interface
|
+-- aws/          AWS client, discovery and resource mapping
|
+-- state/        Resource and snapshot data models
|
+-- graph/        Dependency graph construction and ordering
|
+-- crypto/       Encryption, hashing and signatures
|
+-- media/        Recovery-media abstraction
|
+-- recovery/     Recovery planning, execution and validation
|
+-- config/       Application configuration
|
+-- docs/         Technical and project documentation
```

The exact implementation of several components is still under development. This document describes the intended architecture while distinguishing it from functionality already present in the repository.

---

## 5. Current Implementation Status

The current repository is a Go implementation using:

* Go
* AWS SDK for Go v2
* Cobra for the CLI
* Argon2id for passphrase-based key derivation
* AES-256-GCM for authenticated encryption
* SHA-256 hashing
* Ed25519 signatures
* JSON serialization for the current state model

The repository currently contains working foundations for:

### CLI

The Cobra command structure already contains commands for:

* `scan`
* `snapshot`
* `media`
* `verify`
* `diff`
* `recover`

The command handlers are currently largely placeholders.

### AWS layer

An AWS SDK client wrapper exists and loads the standard AWS configuration.

The discovery and collection interfaces exist, but actual resource discovery is not yet implemented.

### State layer

The repository defines a normalized `Resource` model containing:

* resource ID;
* resource type;
* name;
* region;
* configuration;
* dependencies.

A `Snapshot` model and JSON serialization/deserialization are also present.

### Graph layer

The repository already contains a dependency graph abstraction and dependency-order resolution with detection for:

* missing dependencies;
* dependency cycles.

### Cryptographic layer

The repository contains implementations for:

* Argon2id key derivation;
* AES-256-GCM encryption/decryption;
* SHA-256 resource hashing;
* Merkle-style snapshot root hashing;
* Ed25519 manifest signing and verification.

These mechanisms are intended to protect recovery artifacts stored on untrusted physical media.

### Recovery layer

The recovery package is part of the intended architecture, but the planner, executor and validator still need to be implemented.

---

## 6. Recovery Philosophy

Recovery is intended to reconstruct **functionally equivalent infrastructure**, not necessarily reproduce the exact AWS resource IDs from the original environment.

For example:

```text
Original environment:

logical resource: web-server
AWS ID: i-012345
```

may become:

```text
Recovered environment:

logical resource: web-server
AWS ID: i-098765
```

The recovery engine must therefore maintain mappings between logical resources from the snapshot and newly created AWS resources.

This is particularly important when resource creation results in provider-assigned identifiers.

---

## 7. Recovery Safety

Recovery should not immediately modify AWS.

The intended workflow is:

```text
Snapshot
   |
   v
Verify
   |
   v
Resolve dependencies
   |
   v
Check recoverability
   |
   v
Generate recovery plan
   |
   v
Operator review
   |
   +---- reject ----> stop
   |
   v
Execute
   |
   v
Validate
```

A dry-run planning stage should identify problems such as unavailable dependencies, unsupported resources, unavailable source artifacts, or configuration that cannot be reproduced before destructive or mutating operations are performed.

---

## 8. MVP Scope

The proposed initial resource scope is:

* VPC
* Subnets
* Route Tables
* Internet Gateway
* Security Groups
* EC2
* IAM Roles
* S3
* RDS
* Application Load Balancer
* Route 53

Additional services may be added later.

The MVP should prioritize a small, demonstrable end-to-end recovery path over broad AWS service coverage.

---

## 9. What InfraResc Is Not

InfraResc is not intended to replace:

* Terraform;
* CloudFormation;
* AWS Config;
* AWS Backup;
* Git;
* CI/CD systems;
* application-level disaster recovery;
* database or object-data backup systems.

Its intended role is different: **portable recovery of infrastructure state and relationships**.

Application data and runtime state require separate recovery mechanisms.

---

## 10. Documentation Structure

The `docs/` directory is organized so that high-level architecture and individual implementation details can evolve independently.

```text
docs/
|
+-- architecture/
|   +-- overview.md
|   +-- proposed-recovery-flow.md
|   +-- data-flow.md
|   +-- dependency-model.md
|
+-- components/
|   +-- aws-discovery.md
|   +-- state-model.md
|   +-- graph-engine.md
|   +-- cryptography.md
|   +-- snapshot-format.md
|   +-- recovery-planner.md
|   +-- recovery-executor.md
|   +-- recovery-validator.md
|
+-- aws/
|   +-- supported-resources.md
|   +-- resource-mapping.md
|   +-- iam-requirements.md
|
+-- security/
|   +-- threat-model.md
|   +-- key-management.md
|   +-- integrity-model.md
|
+-- operations/
|   +-- snapshot-workflow.md
|   +-- recovery-runbook.md
|   +-- disaster-demo.md
```

Not every document needs to exist immediately. The architecture and recovery-flow documents should be treated as the design baseline from which the implementation-specific documents are derived.

---

## 11. Design Principle

The central design principle is:

> **Capture. Verify. Plan. Reconstruct. Validate.**

InfraResc should prefer deterministic infrastructure discovery, explicit dependency modeling and operator-visible recovery decisions over opaque automation.

AI or other higher-level assistance may eventually help explain infrastructure or recovery results, but it should not replace the deterministic AWS state collection or recovery logic.
