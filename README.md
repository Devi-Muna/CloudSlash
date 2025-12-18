# CloudSlash

**Infrastructure Waste Analysis for AWS**

CloudSlash identifies idle, orphaned, and underutilized resources in your AWS environment. Unlike tools that rely solely on "Status" checks, CloudSlash correlates CloudWatch metrics with resource topology to find actual waste (e.g., available volumes with no IOPS, NAT Gateways with low throughput).

![License](https://img.shields.io/badge/license-Commercial-blue.svg)
![Platform](https://img.shields.io/badge/platform-Mac%20%7C%20Linux%20%7C%20Windows-lightgrey)

## Core Capabilities

- **Zero Trust Scanning**: Verifies utilization via telemetry rather than metadata.
- **Read-Only**: Operates with `ViewOnlyAccess`. No write permissions required.
- **Drift Detection**: Compares live infrastructure against Terraform state.
- **Heuristic Analysis**:
  - **Zombie EBS**: Detects available volumes or attached volumes with 0 IOPS/30 days.
  - **Idle NAT Gateways**: Identifies gateways costing hourly rates but processing minimal traffic.
  - **S3 Multipart Uploads**: Finds incomplete uploads consuming storage space.
- **Remediation**: Generates `waste.tf` and `import.sh` for safe, managed cleanup.

## Installation

### macOS / Linux

Open your terminal and run:

```bash
curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.sh | bash
```

### Windows (PowerShell)

Run as Administrator (optional, but recommended for PATH updates):

```powershell
irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.ps1 | iex
```

> **Note:** CloudSlash is installed globally. You can run it from any terminal, anytime—even after rebooting. 🔄

## Usage

**Trial Mode (Free)**
Runs locally. Scans your setup and reports findings in the terminal UI. IDs are redacted.

```bash
./cloudslash
```

**Pro Mode**
Unlocks Resource IDs and Terraform generation.
[Purchase License](https://cloudslash.pages.dev)

```bash
./cloudslash -license YOUR-KEY
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

## Architecture

Built in Go. Uses an in-memory graph to model resource relationships. The TUI is powered by Bubble Tea for responsive, real-time feedback during scans.
