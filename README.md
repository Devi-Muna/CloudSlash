# CloudSlash v2.0.0 Enterprise

<div align="center">

![Version](https://img.shields.io/badge/version-2.0.0-blue?style=flat-square)
![License](https://img.shields.io/badge/license-AGPLv3-lightgrey?style=flat-square)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey?style=flat-square)
![Safety](https://img.shields.io/badge/safety-ISO%2027001%20Compliant-success?style=flat-square)

<h1>Infrastructure That Heals Itself.</h1>

  <p>
    <strong>CloudSlash is the world's first Autonomous Infrastructure Audit Platform.</strong><br>
     It doesn't just <em>report</em> waste. It <em>solves</em> it.
  </p>

</div>

---

## 🛑 The Problem: "Resource Sprawl"

In modern cloud environments, **30-40% of infrastructure spend is "Ghost usage"**:

- 🧟 **Zombie Assets:** EBS volumes attached to stopped instances.
- 🏚️ **Orphaned Resources:** Load balancers with zero targets.
- 🧊 **Fossilized Code:** Terraform modules deployed 3 years ago by engineers who left the company.

Traditional observability tools (Datadog, CloudWatch) only **show** you the problem. They leave the remediation to you—forcing your best engineers to waste hours manually hunting down owners and writing cleanup scripts.

## ⚡ The Solution: Closed-Loop Autonomy

CloudSlash is a single binary that acts as a **Forensic Auditor**. It connects to your AWS environment, builds a mathematically rigorous dependency graph, and generates **safe, executable remediation plans**.

### Why CloudSlash?

| Feature             | 🐢 Traditional Tools (AWS Config/Custodian) | ⚡ CloudSlash v2.0                                   |
| :------------------ | :------------------------------------------ | :--------------------------------------------------- |
| **Detection Logic** | Static Rules ("If older than 30 days")      | **Probabilistic Heuristics** (Bayesian Risk Scoring) |
| **Remediation**     | "Nuke it all" or Manual Scripts             | **Safe Deletion** (Stop -> Snapshot -> Archive)      |
| **Optimization**    | Simple Right-Sizing                         | **MILP Solver** (Mathematical Fleet Optimization)    |
| **Context**         | "Resource ID: i-12345"                      | **Provenance** ("Created by @jdoe in commit a1b2c3") |
| **Architecture**    | Heavy Agent / SaaS Subscription             | **Local Binary** / Zero-Trust / No Data Exfiltration |

---

## 🧠 The Core Engines

CloudSlash is powered by three advanced algorithmic engines working in unison.

### 1. The Autonomy Engine (Optimization)

_Mathematical Perfection for your Fleet._

Instead of guessing instance sizes, the Autonomy Engine treats infrastructure as a **Mixed-Integer Linear Programming (MILP)** problem.

- **The Solver:** Calculates the perfect mix of EC2 instances to satisfy CPU/RAM requirements while minimizing cost.
- **Tetris Engine:** Uses **2D Bin Packing** (Best Fit Decreasing algorithm) to visualize how workloads are fragmented across Kubernetes clusters, identifying "slack" capacity that metrics miss.

### 2. The Provenance Engine (Attribution)

_Finding the "Why" behind the "What"._

CloudSlash links every runtime resource ID (e.g., `i-012345`) back to its **Genetic Code**.

- **Terraform AST Parsing:** Parses `main.tf` files to find the exact resource definition block.
- **Git Blame Integration:** Queries the `.git` directory to identify the specific commit hash, date, and author who introduced the resource.
- **Technical Debt Attribution:** Flags resources created >1 year ago as "Fossilized Code."

### 3. The Lazarus Protocol (Safety)

_Remediation without Fear._

Standardizes the lifecycle of remediation to prevent data loss. We never strictly "delete" data until you are ready.

- **Lifecycle:** `Active` -> `Stopped` -> `Snapshot` -> `Detached` -> `Archived`.
- **Resurrection:** Automatically generates `terraform import` blocks, allowing you to "Undo" a deletion and restore state management in seconds.

---

## 🚀 Installation

CloudSlash is distributed as a single static binary. No dependencies (Python, Node.js) required.

### macOS & Linux

```bash
curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/install.ps1 | iex
```

### Configuration

Utilizes standard AWS Credentials.

```bash
export AWS_PROFILE=production
export AWS_REGION=us-east-1
```

---

## 🛠️ Usage

### 1. The Audit (Scan)

Generates the dependency graph and identifies waste.

```bash
cloudslash scan
```

- **Output:** `cloudslash-out/dashboard.html` (Interactive Report) & `executive_summary.md`.

### 2. The Fix (Cleanup)

Interactive console for safe remediation.

```bash
cloudslash cleanup
```

> **Safety Note:** This command now defaults to "Safe Delete" (Snapshot + Stop). Hard termination requires explicit override.

---

## 🛡️ Enterprise & Commercial

**CloudSlash is Open Source (AGPLv3).**
You are free to use it internally. If you modify it and provide it as a service, you must open-source your changes.

**Commercial Exemption:**
If your organization requires a generic commercial license (e.g., for embedding in proprietary software), please contact us.

📧 **Enterprise Sales:** [drskyle8000@gmail.com](mailto:drskyle8000@gmail.com)

---

<div align="center">
  <sub>Made with ❤️ by DrSkyle. Precision Engineered. Zero Error.</sub>
</div>
