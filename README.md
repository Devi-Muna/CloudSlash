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

## 📦 Installation & Quick Start

### macOS (Apple Silicon / M1 / M2)

```bash
curl -L -o cloudslash https://github.com/DrSkyle/CloudSlash/releases/latest/download/cloudslash-darwin-arm64
chmod +x cloudslash
./cloudslash
```

### macOS (Intel)

```bash
curl -L -o cloudslash https://github.com/DrSkyle/CloudSlash/releases/latest/download/cloudslash-darwin-amd64
chmod +x cloudslash
./cloudslash
```

### Linux

```bash
curl -L -o cloudslash https://github.com/DrSkyle/CloudSlash/releases/latest/download/cloudslash-linux-amd64
chmod +x cloudslash
./cloudslash
```

### Windows (PowerShell)

```powershell
Invoke-WebRequest -Uri https://github.com/DrSkyle/CloudSlash/releases/latest/download/cloudslash-windows-amd64.exe -OutFile cloudslash.exe
.\cloudslash.exe
```

---

## 📖 User Guide

👉 **[Read the Full User Walkthrough](WALKTHROUGH.md)** for a deep dive into Multi-Account Scanning, Dashboards, and Safe Remediation.

## 🛠 Features

| Feature                    |  Trial Mode   | Pro Mode |
| :------------------------- | :-----------: | :------: |
| **Scan EC2, S3, RDS, ELB** |      ✅       |    ✅    |
| **Heuristic Analysis**     |      ✅       |    ✅    |
| **TUI Dashboard**          |      ✅       |    ✅    |
| **View Resource IDs**      | ❌ (Redacted) |    ✅    |
| **Generate `waste.tf`**    |      ❌       |    ✅    |
| **Generate `import.sh`**   |      ❌       |    ✅    |

### Activate Pro Mode

To unlock full features (Real IDs, Terraform Generation), you need a license key.
[**Get a Pro License Key**](https://cloudslash-web.pages.dev/#download)

```bash
./cloudslash -license PRO-YOUR-KEY
```

## 📜 Licensing

CloudSlash is sold under a standard commercial license.

## 🔒 Security

- **Read-Only:** CloudSlash only requires `ReadOnlyAccess`.
- **Local Execution:** All analysis happens on your machine. No data is sent to our servers.
- **Open Core:** The core logic is transparent and audit-friendly.

## 🛠️ Architecture

- **Swarm Engine**: Uses AIMD (Additive Increase, Multiplicative Decrease) to scan 500+ accounts/resources in seconds without throttling.
- **In-Memory DAG**: Models your infrastructure as a graph to find disconnected subgraphs (waste).
- **Matrix TUI**: A cyberpunk-style terminal interface built with Bubble Tea.
