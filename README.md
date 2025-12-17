# CloudSlash 🗡️☁️

**The Forensic Accountant for AWS Infrastructure.**

> "Most tools are Janitors. CloudSlash is a Forensic Accountant."

CloudSlash is a CLI tool that scans your AWS infrastructure for "Shadow IT" and waste using a Zero Trust approach. It assumes your tags are lying and your console is outdated. It uses deep-scan heuristics to find the truth.

![License](https://img.shields.io/badge/license-Commercial-blue.svg)
![Platform](https://img.shields.io/badge/platform-Mac%20%7C%20Linux%20%7C%20Windows-lightgrey)

## 🚀 Features

- **Zero Trust Scanning**: Relies on CloudWatch telemetry and Graph Topology, not just "Status: Available".
- **Read-Only**: strictly adheres to `ViewOnlyAccess`. We never write to your cloud.
- **Shadow State Reconciliation**: Compares your live infrastructure against your Terraform State (`.tfstate`) to find drift.
- **Deep-Scan Heuristics**:
  - **Zombie EBS**: Volumes attached to instances stopped for > 30 days.
  - **NAT Gateway Vampires**: Gateways with < 1GB traffic/month.
  - **S3 Multipart Ghosts**: Incomplete uploads hidden in buckets.
  - **RDS & ELB**: Stopped DBs and unused Load Balancers.
- **Reverse Terraform**: Generates `waste.tf` and `import.sh` to help you clean up safely.

## 📦 Installation

Download the latest binary for your platform from the [Releases](https://github.com/yourusername/cloudslash/releases) page.

👉 **[Read the Full User Walkthrough](WALKTHROUGH.md)** for a deep dive into Multi-Account Scanning, Dashboards, and Safe Remediation.

## ⚡ Quick Start (Trial Mode)

Run the tool to see how much money you are burning. No license key required.

## 🚀 Quick Start

### 1. Download

Download the binary for your OS from the [Releases Page](https://github.com/DrSkyle/CloudSlash/releases).

### 2. Run (Trial Mode)

By default, CloudSlash runs in **Trial Mode**. It will show you what waste exists but hides the IDs and disables the fix generation.

```bash
# macOS / Linux
chmod +x cloudslash
./cloudslash

# Windows
.\cloudslash.exe
```

### 3. Activate Pro Mode

To unlock the full features (Real IDs, Terraform Generation), you need a license key.

```bash
./cloudslash -license PRO-YOUR-KEY
```

## 🛠 Features

| Feature                    |  Trial Mode   | Pro Mode |
| :------------------------- | :-----------: | :------: |
| **Scan EC2, S3, RDS, ELB** |      ✅       |    ✅    |
| **Heuristic Analysis**     |      ✅       |    ✅    |
| **TUI Dashboard**          |      ✅       |    ✅    |
| **View Resource IDs**      | ❌ (Redacted) |    ✅    |
| **Generate `waste.tf`**    |      ❌       |    ✅    |
| **Generate `import.sh`**   |      ❌       |    ✅    |

## 📜 Licensing

CloudSlash is sold under a standard commercial license.

[**Get a Pro License Key**](https://cloudslash-web.pages.dev/#download) directly from our website.

## 🔒 Security

- **Read-Only:** CloudSlash only requires `ReadOnlyAccess`.
- **Local Execution:** All analysis happens on your machine. No data is sent to our servers.
- **Open Core:** The core logic is transparent and audit-friendly.

## 🛠️ Architecture

- **Swarm Engine**: Uses AIMD (Additive Increase, Multiplicative Decrease) to scan 500+ accounts/resources in seconds without throttling.
- **In-Memory DAG**: Models your infrastructure as a graph to find disconnected subgraphs (waste).
- **Matrix TUI**: A cyberpunk-style terminal interface built with Bubble Tea.
