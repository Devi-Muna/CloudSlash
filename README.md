# CloudSlash

### The Forensic Cloud Accountant for AWS.

![License: AGPLv3](https://img.shields.io/badge/License-AGPLv3-blue.svg) ![Build: Passing](https://img.shields.io/badge/Build-Passing-Success.svg) ![Go Report: A+](https://img.shields.io/badge/Go_Report-A+-Success.svg) ![Version: v1.3.0](https://img.shields.io/badge/Version-v1.3.0-blue.svg)

> "CloudSlash correlates CloudWatch telemetry with resource topology to find waste that 'Status' checks miss. It bridges the gap between AWS Reality and Terraform State."

![CloudSlash TUI Demo](docs/demo.gif)

## Quick Start (Immediate Gratification)

**macOS / Linux:**

```bash
curl -sL https://cloudslash.io/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://cloudslash.io/install.ps1 | iex
```

**Run Analysis:**

```bash
cloudslash scan
```

---

## Key Capabilities (v1.3.0 Feature Stack)

CloudSlash goes beyond simple "unused" checks by analyzing the **lifecycle** and **relationships** of your infrastructure.

| Domain                  | Capability               | Detection Logic                                                                                                                     |
| :---------------------- | :----------------------- | :---------------------------------------------------------------------------------------------------------------------------------- |
| **The State Doctor**    | **Terraform Bridge**     | Maps waste to local Terraform state. Generates `fix_terraform.sh` to surgically remove resources without breaking state files.      |
| **The Network Surgeon** | **Forensics Engine**     | Identifies "Hollow" NAT Gateways (0 active backend hosts) and Safe-Release Elastic IPs (Cross-referenced with Route53 DNS records). |
| **The Storage Janitor** | **S3 & EBS Hygiene**     | Finds S3 Multipart Upload debris, "Ghost" EBS Snapshots, and legacy `gp2` volumes (recommending `gp3` upgrades).                    |
| **The App Cleaner**     | **Compute Optimization** | Prunes Lambda versions (safely respecting Aliases) and calculates DynamoDB Provisioned vs. On-Demand breakeven points.              |
| **The Log Hunter**      | **Telemetry Audit**      | Metric-based detection of abandoned CloudWatch Log Groups (>30 days silence).                                                       |

---

## Why CloudSlash?

Most tools are just billing dashboards. CloudSlash is an **engineering tool**.

| Feature                     |       CloudSlash        |  Trusted Advisor   | Vantage / CloudHealth |
| :-------------------------- | :---------------------: | :----------------: | :-------------------: |
| **Execution Model**         | **Local CLI (Private)** | SaaS (AWS Console) |   SaaS (3rd Party)    |
| **Terraform Remediation**   |   **✓ Yes (Native)**    |        ✖ No        |         ✖ No          |
| **DNS "Safety Lock"**       |   **✓ Yes (Route53)**   |        ✖ No        |         ✖ No          |
| **"Hollow" Resource Logic** |  **✓ Yes (Topology)**   |        ✖ No        |         ✖ No          |
| **Cost**                    |  **Open Core (Free)**   | Enterprise Support |      $$$$/month       |

> **Note:** CloudSlash is the only tool that runs locally and fixes Terraform state directly.

---

## License Model (Open Core Strategy)

CloudSlash is dedicated to the philosophy of **Sustainable Open Source**.

- **Community Edition (AGPLv3):** Free for personal use, auditing, and contribution. Full forensic engine included.
- **Commercial License:** Unlocks `fix_terraform.sh` automation, PDF/CSV Reporting, and Priority Support.
- **Enterprise:** Indemnification, SLA, and White-labeling rights.

### Social Impact Pledge

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
rm /usr/local/bin/cloudslash
rm -rf ~/.cloudslash
```
