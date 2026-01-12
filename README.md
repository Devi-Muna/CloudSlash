# CloudSlash v2.0.0

![Version](https://img.shields.io/badge/version-2.0.0-blue?style=flat-square) ![License](https://img.shields.io/badge/license-AGPLv3-lightgrey?style=flat-square) ![Build Status](https://img.shields.io/badge/build-passing-success?style=flat-square)

## Project Overview

CloudSlash is an autonomous infrastructure optimization platform designed for enterprise-scale cloud environments. Unlike passive observability tools that simply report metrics, CloudSlash leverages advanced mathematical modeling and graph topology analysis to actively solve resource inefficiency problems.

It functions as a forensic auditor and autonomous agent, correlating disparate data sources—CloudWatch metrics, network traffic logs, infrastructure-as-code (IaC) definitions, and git provenance—to identify, attribute, and remediate waste with mathematical certainty.

**Core Value Proposition:**

- **Autonomous Optimization:** Uses Mixed-Integer Linear Programming (MILP) to calculate mathematically optimal fleet compositions.
- **Forensic Attribution:** Links every cloud resource back to the specific line of code and engineer who created it.
- **Non-Destructive Remediation:** Enforces a "Safety First" lifecycle for retiring resources, prioritizing soft deletion and state restoration over immediate termination.

---

## Installation

CloudSlash is distributed as a single static binary with no external dependencies.

### macOS & Linux

Execute the following command in your terminal to download and install the latest binary:

```bash
curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.sh | bash
```

### Windows (PowerShell)

Run the following command in PowerShell:

```powershell
irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.ps1 | iex
```

### Prerequisites

- **AWS Credentials:** The tool utilizes the standard AWS credential chain (Environment Variables, `~/.aws/credentials`, or IAM Instance Profiles).
- **Terraform (Optional):** Required only for the State Analysis and Provenance Engine features.

---

## Key Capabilities

### 1. The Autonomy Engine

The core intelligence of CloudSlash v2.0, responsible for prescriptive optimization.

- **The Solver (MILP Optimizer):**
  Analyzes workload requirements (CPU, RAM, Network) against the cloud provider's catalog to determine the most cost-efficient instance fleet. It solves constraints for Availability Zone distribution and reserved capacity coverage using a Mixed-Integer Linear Programming model.

- **Tetris Engine (Bin Packing):**
  Visualizes and solves Kubernetes cluster fragmentation. Utilizing "Best Fit Decreasing" (BFD) algorithms, it calculates the optimal packing of Pods onto Nodes, quantifying the exact "waste" caused by inefficient scheduling rather than idle capacity.

- **The Oracle (Risk Assessment):**
  A Bayesian Probability engine that evaluates the stability of Spot Instances. It maintains a decaying "Risk Score" for every instance pool based on historical interruption data, ensuring that cost optimization never compromises availability.

### 2. The Provenance Engine

Provides deep auditability by linking runtime resources to their static definitions.

- **Git Forensics:** reverse-engineers active resource IDs to their origin in the Terraform state, then parses the `main.tf` Abstract Syntax Tree (AST) to identify the Git commit hash and author responsible for the resource.
- **Technical Debt Attribution:** Shifts the remediation focus from technical auditing to social accountability by notifying specific engineers of their unmanaged resources.

### 3. The Accountability Engine

- **Identity Resolution:** Automatically maps IAM User ARNs and Git Committer emails to corporate identity providers (e.g., Slack User IDs).
- **Direct Notification:** Facilitates targeted communication, allowing the platform to send optimization reports directly to resource owners rather than generic broadcast channels.

### 4. The Lazarus Protocol

- **Soft Deletion Lifecycle:** Implements a rigorous remediation standard where resources are never immediately destroyed.
  - **Instances:** Stopped and tagged.
  - **Volumes:** Snapshotted and detached.
- **Drift-Based Restoration:** Automatically generates Terraform `import` blocks (`terraform import ...`) for every remediated resource, enabling rapid state restoration in the event of a false positive.

---

## Functional Modules

CloudSlash includes specialized heuristic engines for various AWS services:

- **Traffic Cessation Analysis (NAT/ELB):** Identifies gateways and load balancers that have processed zero bytes of data over a configurable retention period (default: 7 days).
- **Storage Hygiene:** Detects unattached EBS volumes, incomplete S3 Multipart Uploads (hidden costs), and legacy `gp2` volumes eligible for `gp3` modernization.
- **Compute Rationalization:** Identifies idle EC2 instances and "stale" Lambda functions (deployed but never invoked).

---

## Usage Guidelines

The application is controlled via a command-line interface (CLI).

### 1. Standard Scan (TUI Mode)

Launches an interactive Terminal User Interface for exploring resources and visualizing topology.

```bash
cloudslash scan
```

### 2. Headless Mode (CI/CD)

Runs the analysis without a UI, suitable for automated pipelines. Output is logged to `stdout` and JSON reports.

```bash
cloudslash scan --headless --region us-east-1,us-west-2
```

### 3. Mock Mode (Evaluation)

Simulates a scan against an internal mock dataset to demonstrate engine capabilities without requiring AWS credentials.

```bash
cloudslash scan --mock
```

### Configuration

Configuration can be supplied via flags or a `config.yaml` file.

- `--region`: Comma-separated list of target regions.
- `--no-metrics`: Disables CloudWatch API calls to save costs (relies on state data only).

---

## Troubleshooting & FAQ

**Q: I am receiving "Access Denied" errors.**
**A:** CloudSlash requires `ReadOnlyAccess` to the target AWS account. For Remediation features, specific `Write` permissions (e.g., `ec2:StopInstance`) are required. Ensure your AWS credentials are valid.

**Q: The Terraform State analysis is empty.**
**A:** Ensure you are running CloudSlash from the root directory of your Terraform project, or that the system has access to the remote state backend defined in your `terraform` block.

**Q: How is "Waste" defined?**
**A:** Waste is strictly defined by heuristics located in `internal/heuristics`. Generally, a resource is flagged if it has zero utilization (CPU, Network, I/O) over the analysis window (default 7 days).

---

## Contribution Guidelines

We welcome contributions from the engineering community.

1.  **Fork & Clone:** Fork the repository and clone it locally.
2.  **Branching:** Create a feature branch (`feature/my-enhancement`).
3.  **Testing:** Ensure all unit tests pass (`make test`). New features must include accompanying tests.
4.  **Submission:** Open a Pull Request against the `main` branch. Provide a detailed description of the change and the problem it solves.

---

## Licensing

This software is licensed under the **AGPLv3**.
Copyright © 2026 CloudSlash Contributors.
