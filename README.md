# CloudSlash

![Version](https://img.shields.io/badge/version-v1.3.1-blue)

**AWS Resource Reclamation & Terraform State Reconciliation**

CloudSlash correlates CloudWatch metrics with resource topology to identify idle infrastructure that standard "Status" checks miss. It bridges the gap between the actual AWS environment and your local Terraform state.

Unlike SaaS billing dashboards, CloudSlash runs locally and generates actionable scripts to clean up state files.

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
cloudslash scan
```

> **Detailed Guide:** Check out the [Operator's Guide](WALKTHROUGH.md) for advanced usage, heuristics, and troubleshooting.

## Key Capabilities (v1.3.0+)

**Terraform State Remediation**
Maps identified waste back to your local Terraform state. Generates a generated `fix_terraform.sh` script to remove resources from state.

> **Safety First:** The script automatically creates a timestamped backup (`tfstate_backup_TIMESTAMP.json`) of your state file before touching anything.

**Deep Resource Inspection**
Goes beyond simple uptime checks:

- **Hollow NAT Gateways**: Identifies gateways with zero active backend hosts.
- **Safe-Release EIPs**: Cross-references Elastic IPs with Route53 DNS records to ensure IPs aren't serving traffic before release.

**Audit Reporting**
Generates `waste_report.csv` containing resource ARNs, cost estimates, risk scores, and owner tags for ingestion into other tools or spreadsheets.

**Executive Summary (Pro)**
Generates `executive_summary.md` suitable for management reviews or "Pen Test" style infrastructure audits.

## Comparison

CloudSlash is designed for engineering workflows, not finance teams.

| Feature              |        CloudSlash         |  Trusted Advisor   | Vantage / CloudHealth |
| :------------------- | :-----------------------: | :----------------: | :-------------------: |
| **Remediation**      | Terraform State Import/Rm |       Manual       |        Manual         |
| **DNS Safety Check** |  Route53 Cross-reference  |         No         |          No           |
| **Execution Model**  |    Local CLI (Private)    | SaaS (AWS Console) |  SaaS (Third Party)   |
| **Pricing**          |         Open Core         | Enterprise Support | Monthly Subscription  |

## License Model

| Feature                                         | Community Edition (Free) | Pro / Enterprise |
| :---------------------------------------------- | :----------------------: | :--------------: |
| **Full Forensic Scan**                          |            ✅            |        ✅        |
| **CSV Reporting (waste_report.csv)**            |            ✅            |        ✅        |
| **Interactive TUI**                             |            ✅            |        ✅        |
| **Terraform Fix Generation (fix_terraform.sh)** |            ❌            |        ✅        |
| **Executive Summary (.md)**                     |            ❌            |        ✅        |
| **Headless Mode (CI/CD integration)**           |            ❌            |        ✅        |
| **Priority Support**                            |            ❌            |        ✅        |

> **Mission:** CloudSlash is bootstrapped. A portion of commercial license revenue is donated to orphanages and animal rescue shelters in developing communities.
>
> [**Support Our Mission & Upgrade**](https://checkout.freemius.com/app/22411/plan/37525/) | [**Follow Impact Updates**](https://x.com/dr__skyle)

## Build from Source

**Prerequisites:** Go 1.21+

```bash
git clone https://github.com/DrSkyle/cloudslash.git
cd cloudslash
go build -o /usr/local/bin/cloudslash ./cmd/cloudslash
```

## Uninstall

```bash
sudo rm /usr/local/bin/cloudslash
rm -rf ~/.cloudslash
```

Made with love ❤️ by DrSkyle
