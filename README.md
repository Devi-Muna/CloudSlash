# CloudSlash

### The Forensic Cloud Accountant for AWS.

![License: AGPLv3](https://img.shields.io/badge/License-AGPLv3-blue.svg) ![Build: Passing](https://img.shields.io/badge/Build-Passing-Success.svg) ![Go Report: A+](https://img.shields.io/badge/Go_Report-A+-Success.svg) ![Version: v1.3.0](https://img.shields.io/badge/Version-v1.3.0-blue.svg)

> "CloudSlash correlates CloudWatch telemetry with resource topology to find waste that 'Status' checks miss. It bridges the gap between AWS Reality and Terraform State."

![CloudSlash TUI Demo](docs/demo.gif)

## Quick Start

Get running in seconds. No dependencies required.

**macOS / Linux**

```bash
curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.sh | bash
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.ps1 | iex
```

**Run Analysis**

```bash
cloudslash scan
```

[Read the Official Guide](WALKTHROUGH.md)

---

## Key Capabilities (v1.3.0 SSSS+)

CloudSlash is an **engineering tool** built for precision forensics.

| Domain                  | Capability               | Detection Logic                                                                                                                     |
| :---------------------- | :----------------------- | :---------------------------------------------------------------------------------------------------------------------------------- |
| **The State Doctor**    | **The Scalpel** 🔒       | Maps waste to local Terraform state. Generates a SAFE `fix_terraform.sh` with automatic backups to surgically remove zombies.       |
| **The Evidence Locker** | **Forensic Reporting**   | Generates `waste_report.csv` with full forensic evidence (Risk Score, Owner ARN, Cost, Action) for every single resource.           |
| **The Network Surgeon** | **Forensics Engine**     | Identifies "Hollow" NAT Gateways (0 active backend hosts) and Safe-Release Elastic IPs (Cross-referenced with Route53 DNS records). |
| **The Executive Brief** | **Executive Summary** 🔒 | Generates a "Pen Test" style audit report (`executive_summary.md`) for management.                                                  |

---

## Why CloudSlash?

Most tools are just billing dashboards. CloudSlash is a **Forensic Weapon**.

| Feature                     |       CloudSlash        |  Trusted Advisor   | Vantage / CloudHealth |
| :-------------------------- | :---------------------: | :----------------: | :-------------------: |
| **The Scalpel (Terraform)** |  **✓ Yes (Surgical)**   |        ✖ No        |         ✖ No          |
| **DNS "Safety Lock"**       |   **✓ Yes (Route53)**   |        ✖ No        |         ✖ No          |
| **Execution Model**         | **Local CLI (Private)** | SaaS (AWS Console) |   SaaS (3rd Party)    |
| **Cost**                    |  **Open Core (Free)**   | Enterprise Support |      $$$$/month       |

> **Note:** CloudSlash is the only tool that runs locally and fixes Terraform state directly.

---

## License Model

| Feature                              | Community Edition (Free) | Pro / Enterprise |
| :----------------------------------- | :----------------------: | :--------------: |
| **Full Forensic Scan**               |            ✅            |        ✅        |
| **The Evidence Locker (CSV)**        |            ✅            |        ✅        |
| **Interactive TUI**                  |            ✅            |        ✅        |
| **The Scalpel (`fix_terraform.sh`)** |            ❌            |        ✅        |
| **The Executive Brief (`.md`)**      |            ❌            |        ✅        |
| **Headless Mode (`--headless`)**     |            ❌            |        ✅        |
| **Support**                          |        Community         |     Priority     |

> **SOCIAL IMPACT PLEDGE**
> CloudSlash is 100% independent and bootstrapped. We pledge a portion of every Commercial License to support orphanages and animal rescue shelters in developing communities.

---

## Installation & Uninstallation

**Manual Install (Go):**

```bash
git clone https://github.com/DrSkyle/cloudslash.git
cd cloudslash
go build -o /usr/local/bin/cloudslash ./cmd/cloudslash
```

**Uninstallation:**

```bash
sudo rm /usr/local/bin/cloudslash
rm -rf ~/.cloudslash
```
