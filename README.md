# CloudSlash v2.0.0: The Autonomy Release

![Version](https://img.shields.io/badge/version-2.0.0-blue?style=flat-square) ![License](https://img.shields.io/badge/license-AGPLv3-lightgrey?style=flat-square) ![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey?style=flat-square)

> **"Infrastructure that heals itself."**

CloudSlash v2.0.0 represents a paradigm shift from passive cloud observability to **autonomous infrastructure optimization**. It is an enterprise-grade forensic engine designed to mathematically solve the problem of cloud waste using constraint satisfaction algorithms, graph topology analysis, and localized provenance tracking.

---

## 📚 Table of Contents

1.  [Executive Summary](#executive-summary)
2.  [Installation](#installation)
3.  [Core Engines](#core-engines)
    - [1. Autonomy Engine (MILP Solver)](#1-autonomy-engine)
    - [2. Provenance Engine (Git Forensics)](#2-provenance-engine)
    - [3. Accountability Engine (Identity)](#3-accountability-engine)
    - [4. Lazarus Protocol (Remediation)](#4-lazarus-protocol)
4.  [Heuristics Catalog (Deep Dive)](#heuristics-catalog)
5.  [Command Reference](#command-reference)
    - [`scan`](#command-scan)
    - [`nuke`](#command-nuke)
    - [`export`](#command-export)
6.  [Integration Guide](#integration-guide)
7.  [Data Models](#data-models)

---

## Executive Summary

Modern cloud environments suffer from "Resource Sprawl"—ghost assets that incur cost but deliver zero business value. Traditional tools mostly report these metrics. **CloudSlash actuates them.**

By combining **Linear Programming** (for fleet optimization) with **Abstract Syntax Tree (AST) Parsing** (for code attribution), CloudSlash delivers a closed-loop system that finds waste, identifies the exact engineer responsible, and provides the mathematically optimal remediation plan.

---

## Installation

CloudSlash is distributed as a single, static binary. No dependencies (Python, Node.js) are required.

### 🍎 macOS / 🐧 Linux

```bash
curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/install.sh | bash
```

### 🪟 Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/install.ps1 | iex
```

### ⚙️ Configuration

The tool utilizes the standard AWS Credential Chain. Ensure your environment is configured:

```bash
export AWS_PROFILE=production
export AWS_REGION=us-east-1
```

_Optional:_ Create a `cloudslash.yaml` in your root directory for persistent settings.

---

## Core Engines

### 1. Autonomy Engine

_The Brain._
Instead of simple "right-sizing" rules, the Autonomy Engine treats infrastructure as a **Mixed-Integer Linear Programming (MILP)** problem.

- **The Solver:** Calculates the perfect mix of EC2 instances to satisfy CPU/RAM requirements while minimizing cost.
- **Tetris Engine:** Uses **2D Bin Packing** (Best Fit Decreasing algorithm) to visualize how workloads are fragmented across Kubernetes clusters, identifying "slack" capacity that metrics miss.
- **The Oracle:** A Bayesian Risk model that tracks Spot Instance interruptions per-AZ, assigning a verifiable "Reliability Score" to every potential instance type.

### 2. Provenance Engine

_The Auditor._
Links every runtime resource ID (e.g., `i-012345`) back to its **Genetic Code**.

- **Terraform AST Parsing:** Parses `main.tf` files to find the exact resource definition block.
- **Git Blame Integration:** Queries the `.git` directory to identify the specific commit hash, date, and author who introduced the resource.
- **Legacy Detection:** Flags resources created >1 year ago as "Fossilized Code."

### 3. Accountability Engine

_The Enforcer._
Maps abstract IAM ARNs to real humans.

- **Identity Resolution:** Correlates `arn:aws:iam::123:user/jdoe` -> `jdoe@company.com` -> `SlackID: U12345`.
- **Direct Notification:** Sends "Waste Reports" directly to the responsible engineer via Slack, bypassing generic channels to ensure action is taken.

### 4. Lazarus Protocol

_The Safety Net._
Standardizes the lifecycle of remediation to prevent data loss.

- **Lifecycle:** `Active` -> `Stopped` -> `Snapshot` -> `Detached` -> `Archived`.
- **Resurrection:** Automatically generates `terraform import` blocks effectively allowing you to "Undo" a deletion by restoring state management in seconds.

---

## Heuristics Catalog

CloudSlash v2.0 includes 15+ specialized heuristic definitions located in `internal/heuristics`.

| ID        | Heuristic Name    | Logic Description                                               |
| :-------- | :---------------- | :-------------------------------------------------------------- |
| **H-001** | `ZombieEBS`       | Unattached EBS volumes > 7 days old.                            |
| **H-002** | `HollowNAT`       | NAT Gateways with 0 bytes processed in 14 days.                 |
| **H-003** | `GhostLB`         | ELBs with 0 dependent targets or 0 active connections.          |
| **H-004** | `FossilAMI`       | AMIs > 2 years old not in use by any active ASG.                |
| **H-005** | `LogHoarder`      | CloudWatch Log Groups with > 10GB storage and 0 reads.          |
| **H-006** | `EIPWaste`        | Elastic IPs unattached or attached to stopped instances.        |
| **H-007** | `StaleLambda`     | Functions with `LastModified` < 30 days and `Invocations` == 0. |
| **H-008** | `SnapRot`         | EBS Snapshots > 1 year old referencing deleted volumes.         |
| **H-009** | `MultipartDebris` | Incomplete S3 Multipart uploads older than 7 days.              |
| **H-010** | `GP2Legacy`       | Detects `gp2` volumes eligible for free migration to `gp3`.     |

---

## Command Reference

### `scan`

The primary entry point. Orchestrates the graph resolution and waste detection.

**Syntax:**

```bash
cloudslash scan [flags]
```

**Flags:**

- `--headless`: Disables the TUI. Best for cron jobs.
- `--region <list>`: Comma-separated target regions (e.g., `us-east-1,eu-central-1`).
- `--no-metrics`: Skip CloudWatch API calls (faster, but less accurate).
- `--mock`: Run against internal simulation data (evaluation mode).
- `--slack-webhook <url>`: specific one-time webhook override.

**Output Artifacts:** (in `./cloudslash-out/`)

- `waste_report.json`: Full forensic dump.
- `dashboard.html`: Interactive visualization.
- `resource_deletion.sh`: Bash script for bulk cleanup.
- `fix_terraform.sh`: Script to remove items from `.tfstate`.

### `nuke`

The interactive remediation console. Reads the graph and prompts for deletion.

**Syntax:**

```bash
cloudslash nuke
```

_Warning: This effectively runs `aws <service> delete-...`. Use with caution._

**Workflow:**

1.  Initiates a fresh scan.
2.  Filters for High-Confidence Waste (100% Probability).
3.  Calculates **Blast Radius** (Dependents).
4.  Prompts user: `[ACTION] DELETE this resource? [y/N]`

### `export`

Extracts internal graph data for external auditing tools.

**Syntax:**

```bash
cloudslash export
```

Currently aliases to `scan --headless` but focused on ensuring CSV/JSON consistency.

---

## Data Models

### Audit Report Schema (`waste_report.json`)

The `audit_detail` key (formerly `forensic_evidence`) serves as the immutable record of waste.

```json
{
  "scan_id": "scan-1736691452",
  "resources": [
    {
      "id": "vol-0abc123",
      "type": "AWS::EC2::Volume",
      "cost_monthly": 45.0,
      "provenance": {
        "author": "jane.doe@example.com",
        "commit": "a1b2c3d",
        "file": "modules/storage/main.tf:45"
      },
      "audit_detail": {
        "heuristic": "ZombieEBS",
        "confidence": 1.0,
        "reason": "Volume state 'available' for > 7 days"
      }
    }
  ]
}
```

---

## Integration Guide

### Slack Webhooks

CloudSlash sends Rich Block Kit messages. Configure by setting:

```bash
export CLOUDSLASH_SLACK_WEBHOOK="https://hooks.slack.com/..."
```

**Features:**

- **Morning Brief:** Summary of yesterday's waste velocity.
- **Budget Alarm:** Triggers if spend velocity acceleration > 20%.

### Terraform Workflow

1.  Run `cloudslash scan` in your Terraform root.
2.  Script detects `terraform.tfstate`.
3.  Generates `cloudslash-out/fix_terraform.sh`.
4.  User reviews script -> executes -> runs `terraform plan` to confirm state convergence.

---

## Licensing

**AGPLv3**. Source code is available for audit. Enterprise licenses available for indemnification and support.

_Copyright © 2026 Dr. Skyle & The CloudSlash Authors._
