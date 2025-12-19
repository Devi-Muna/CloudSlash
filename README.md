# CloudSlash

**The Forensic Accountant for your AWS Infrastructure**

> **Status:** Precision Engineered. Zero Error.

CloudSlash identifies idle, orphaned, and underutilized resources in your AWS environment. Unlike tools that rely solely on "Status" checks, CloudSlash correlates CloudWatch metrics with resource topology to find actual waste (e.g., available volumes with no IOPS, NAT Gateways with low throughput).

![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)
![Platform](https://img.shields.io/badge/platform-Mac%20%7C%20Linux%20%7C%20Windows-lightgrey)
![Version](https://img.shields.io/badge/version-v1.1.8-brightgreen)

> **New to CloudSlash?** Check out the complete [**User Walkthrough**](WALKTHROUGH.md) for a step-by-step guide.

## Open Source & Commercial (Dual Licensed)

CloudSlash is **Open Source** (AGPLv3) to guarantee transparency and community freedom.

- **Community Edition (AGPLv3):** Free for personal use, audit, and contribution. If you modify the source and distribute it (even internally within a large org), you may be required to share your changes.
- **Commercial License (Standard):** Unlocks automated reporting, Terraform remediation, and support. Usage does not trigger copyleft provisions when used as an internal tool.
- **Enterprise License (AGPL Exception):** For organizations (e.g., Banks, Fintech) requiring **Indemnification** and a total exemption from AGPL. This license allows you to embed CloudSlash source code into proprietary, closed-source banking platforms.

> **Fair Use Check:** Yes, a developer could manually edit the source code for personal learning. **Let them.** This "Open Core" model sustains the project.

## Core Capabilities

- **Zero Trust Scanning**: Verifies utilization via telemetry rather than metadata.
- **Read-Only**: Operates with `ViewOnlyAccess`. No write permissions required.
- **Drift Detection**: Compares live infrastructure against Terraform state.
- **Heuristic Analysis**:
  - **Zombie EBS**: Detects available volumes or attached volumes with 0 IOPS/30 days.
  - **Idle NAT Gateways**: Identifies gateways costing hourly rates but processing minimal traffic.
  - **S3 Multipart Uploads**: Finds incomplete uploads consuming storage space.
  - **Fossil Snapshots**: RDS Snapshots unlinked from any active cluster.
  - **Orphaned ELBs**: Load Balancers with zero requests.
  - **Loose EIPs**: Unassociated Elastic IPs.
- **Remediation**: Generates `waste.tf` and `import.sh` for safe, managed cleanup.

## Installation

### macOS / Linux

Open your terminal and run the precision installer:

```bash
curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.sh | bash
```

### Windows (PowerShell)

Run as Administrator (optional, but recommended for PATH updates):

```powershell
irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.ps1 | iex
```

> **Note:** CloudSlash installs to `/usr/local/bin` (Unix) or `%LOCALAPPDATA%` (Windows) and is available globally.

## Usage

CloudSlash uses a modern, "Future-Glass" CLI interface.

### 1. Interactive Mode (Default)

Simply run the command to verify your environment and start the TUI.

```bash
cloudslash
```

### 2. Headless Scan (CI/CD)

Run without the UI for automated pipeline integration.

```bash
cloudslash scan --region us-west-2
```

### 3. Pro Mode (License)

Unlock full reporting and Terraform generation. [Get a License](https://cloudslash.pages.dev).

```bash
cloudslash --license YOUR_KEY_HERE
```

### 4. Auto-Update

CloudSlash checks for updates automatically. To upgrade manually:

```bash
cloudslash update
```

## Security

- **IAM Scope:** Requires only `ReadOnlyAccess`.
- **Data Privacy:** Analysis is performed locally (Edge Compute). No credential or graph data leaves your machine.

## Uninstallation

To remove CloudSlash completely:

**macOS / Linux:**

```bash
sudo rm /usr/local/bin/cloudslash
```

**Windows:**

```powershell
Remove-Item "$env:LOCALAPPDATA\CloudSlash" -Recurse -Force
```

## Advanced Features (v1.1)

### 🛡️ Reverse-Terraform (Pro)

CloudSlash generates a `fix_terraform.sh` script in the output directory.

1. Run a scan: `cloudslash --license ...`
2. Inspect the script: `cat cloudslash-out/fix_terraform.sh`
3. Execute to safely remove waste from state: `bash cloudslash-out/fix_terraform.sh`

### 🕵️ Forensics (Pro)

Automatically enabled for licensed users.

- **TUI**: Look for the "Owner" column.
- **Red "UNCLAIMED"**: No tags, no CloudTrail creation event found (orphan).
- **Green "IAM:user"**: Identified creator via CloudTrail.

### 🏷️ Suppression (Tags)

CloudSlash allows you to "ack" waste without deleting it.

1. Run a scan.
2. Execute `bash cloudslash-out/ignore_resources.sh`.
3. This applies the `cloudslash:ignore=true` tag to all currently identified waste.
4. Future scans will skip these resources (or un-mark them as waste).

## Architecture

Built in Go. Uses an in-memory graph to model resource relationships. The TUI is powered by Bubble Tea.
The CLI is built with `Cobra` and `Viper` for maximum extensibility.
