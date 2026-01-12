# CloudSlash v2.0.0

![Version](https://img.shields.io/badge/version-2.0.0-blue?style=flat-square) ![License](https://img.shields.io/badge/license-AGPLv3-lightgrey?style=flat-square) ![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey?style=flat-square)

> **"Infrastructure that heals itself."**

CloudSlash is an autonomous infrastructure optimization platform designed for enterprise-scale cloud environments. Unlike passive observability tools that simply report metrics, CloudSlash leverages advanced mathematical modeling and graph topology analysis to actively solve resource inefficiency problems.

It functions as a forensic auditor and autonomous agent, correlating disparate data sources—CloudWatch metrics, network traffic logs, infrastructure-as-code (IaC) definitions, and git provenance—to identify, attribute, and remediate waste with mathematical certainty.

## Executive Summary

Modern cloud environments suffer from "Resource Sprawl"—ghost assets that incur cost but deliver zero business value. Traditional tools mostly report these metrics. **CloudSlash actuates them.**

By combining **Linear Programming** (for fleet optimization) with **Abstract Syntax Tree (AST) Parsing** (for code attribution), CloudSlash delivers a closed-loop system that finds waste, identifies the exact engineer responsible, and provides the mathematically optimal remediation plan.

---

## Installation

CloudSlash is distributed as a single static binary. No external dependencies (Python, Node.js, etc.) are required.

### macOS & Linux

```bash
curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/install.ps1 | iex
```

### Configuration

The tool utilizes the standard AWS Credential Chain. Ensure your environment is configured:

```bash
export AWS_PROFILE=production
export AWS_REGION=us-east-1
```

_(Optional)_ Create a `cloudslash.yaml` in your root directory for persistent settings.

---

## Key Capabilities

### 1. The Autonomy Engine (Optimization)

Instead of simple "right-sizing" rules, the Autonomy Engine treats infrastructure as a **Mixed-Integer Linear Programming (MILP)** problem.

- **The Solver:** Calculates the perfect mix of EC2 instances to satisfy CPU/RAM requirements while minimizing cost.
- **Tetris Engine:** Uses **2D Bin Packing** (Best Fit Decreasing algorithm) to visualize how workloads are fragmented across Kubernetes clusters, identifying "slack" capacity that metrics miss.
- **The Oracle:** A Bayesian Risk model that tracks Spot Instance interruptions per-AZ, assigning a verifiable "Reliability Score" to every potential instance type.

### 2. The Provenance Engine (Attribution)

Links every runtime resource ID (e.g., `i-012345`) back to its **Genetic Code**.

- **Terraform AST Parsing:** Parses `main.tf` files to find the exact resource definition block.
- **Git Blame Integration:** Queries the `.git` directory to identify the specific commit hash, date, and author who introduced the resource.
- **Technical Debt Attribution:** Flags resources created >1 year ago as "Fossilized Code."

### 3. The Accountability Engine (Identity)

Maps abstract IAM ARNs to real humans.

- **Identity Resolution:** Correlates `arn:aws:iam::123:user/jdoe` -> `jdoe@company.com` -> `SlackID: U12345`.
- **Direct Notification:** Sends "Waste Reports" directly to the responsible engineer via Slack, bypassing generic channels to ensure action is taken.

### 4. The Lazarus Protocol (Remediation)

Standardizes the lifecycle of remediation to prevent data loss.

- **Lifecycle:** `Active` -> `Stopped` -> `Snapshot` -> `Detached` -> `Archived`.
- **Resurrection:** Automatically generates `terraform import` blocks effectively allowing you to "Undo" a deletion by restoring state management in seconds.

<details>
<summary><strong>View Heuristics Catalog (Deep Dive)</strong></summary>

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

</details>

---

## Reporting & Output

- **Dashboard (HTML):** Interactive report with charts (`cloudslash-out/dashboard.html`).
- **Data Export:** Raw `waste_report.csv` and `waste_report.json`.
- **Executive Summary:** Markdown audit brief (`executive_summary.md`).
- **Remediation Scripts:**
  - `resource_deletion.sh`: Bash script for bulk cleanup (AWS CLI).
  - `fix_terraform.sh`: Script to remove items from `.tfstate`.

## Command Reference

### `scan`

The primary entry point. Orchestrates the graph resolution and waste detection.

```bash
cloudslash scan [flags]
```

**Flags:**

- `--headless`: Disables the TUI. Best for cron jobs.
- `--region <list>`: Comma-separated target regions (e.g., `us-east-1,eu-central-1`).
- `--no-metrics`: Skip CloudWatch API calls (faster, but less accurate).
- `--mock`: Run against internal simulation data (evaluation mode).

### `nuke`

The interactive remediation console. Reads the graph and prompts for deletion.

```bash
cloudslash nuke
```

_Warning: This effectively runs `aws <service> delete-...`. Use with caution._

### `export`

Extracts internal graph data for external auditing tools.

```bash
cloudslash export
```

---

## Support the Project

CloudSlash is open source and free. If you'd like to support development or need priority help:

- **[Support ($9.99/mo)](https://github.com/sponsors/DrSkyle)** - Priority Support & Gratitude.
- **[Super Support ($19.99/mo)](https://github.com/sponsors/DrSkyle)** - Prioritized Feature Requests & Sponsor Recognition.

---

## License & Commercial Use

**CloudSlash is AGPLv3.**

You are free to use it, modify it, and run it internally for your business. _If you modify CloudSlash and provide it as a service/product to others, you must open-source your changes._

**Enterprise Exemption:** If your organization requires a commercial license for AGPL compliance (e.g., embedding CloudSlash in proprietary software without releasing source code), exemptions are available. Contact: [sales@cloudslash.io](mailto:sales@cloudslash.io).

_Copyright © 2026 Dr. Skyle & The CloudSlash Authors._
