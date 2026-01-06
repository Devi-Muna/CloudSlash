# CloudSlash

![Version](https://img.shields.io/badge/version-v1.3.3-00FF99?style=flat-square) ![License](https://img.shields.io/badge/license-AGPLv3-blue?style=flat-square) ![Status](https://img.shields.io/badge/status-production-green?style=flat-square)

**The Forensic Cloud Accountant for AWS.**

CloudSlash is a **local-first, open-source** command line tool that bridges the gap between FinOps (Cost) and SecOps (Risk). It correlates CloudWatch metrics with network topology to identify "Zombie" infrastructure that standard tools miss—reducing both your AWS bill and your attack surface.

No SaaS. No Data Exfiltration. **100% Local.**

## Quick Start

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
cloudslash
```

> **Detailed Guide:** Check out the [Operator's Guide](WALKTHROUGH.md) for advanced usage, heuristics, and troubleshooting.

## Key Capabilities

All features are available in this Community Edition.

### **1. Deep Forensics & Risk Detection**

Goes beyond simple uptime checks to find security liabilities:

- **Hollow NAT Gateways**: Identifies gateways with zero active backend hosts (Cost + Network Noise).
- **Dangling Elastic IPs**: Cross-references EIPs with Route53 DNS records to prevent **Subdomain Takeover** attacks.
- **Zombie Load Balancers**: Finds ELBs with no healthy targets.
- **Abandoned EKS Clusters**: Detects control planes running with 0 nodes or 0 pods.

### **2. The State Doctor (Terraform Bridge)**

Maps identified waste back to your local Terraform state.

- Automatically generates module-aware `terraform state rm` commands.
- **Safety First:** Automatically creates a timestamped state backup (`tfstate_backup_TIMESTAMP.json`) before touching anything.

### **3. Reporting Engine**

- **Dashboard (HTML)**: A full interactive HTML report with charts and graphs (`cloudslash-out/dashboard.html`).
- **Data Export**: Raw `waste_report.csv` and `waste_report.json` for custom tooling.
- **Executive Summary**: A markdown formatted audit summary suitable for CTO/VP reviews (`executive_summary.md`).

## Build from Source

**Prerequisites:** Go 1.21+

```bash
git clone https://github.com/DrSkyle/cloudslash.git
cd cloudslash
go build -o /usr/local/bin/cloudslash ./cmd/cloudslash
```

## License & Commercial Use

**CloudSlash is AGPLv3.**

You are free to use it, modify it, and run it internally for your business.
_If you modify CloudSlash and provide it as a service/product to others, you must open-source your changes._

**Enterprise Exemption**:
If your organization requires a commercial license for AGPL compliance (e.g., embedding CloudSlash in proprietary software without releasing source code), exemptions are available.
Contact: `enterprise@cloudslash.com`

---

Made by DrSkyle
