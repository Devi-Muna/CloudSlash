# CloudSlash User Guide

This guide details the operation of CloudSlash for identifying AWS infrastructure waste and performing safe remediation.

---

## 1. Quick Start

### Installation

Refer to the [README](README.md) for installation instructions specific to the operating system.

### Authentication

CloudSlash utilizes standard AWS credentials located in `~/.aws/credentials` or `~/.aws/config`.

1.  **Configure AWS CLI**: `aws configure`
2.  **SSO Login** (if applicable): `aws sso login`
3.  **Permissions**: Ensure the user or role has `ReadOnlyAccess` and `ViewOnlyAccess`.

---

## 2. Workflows

### Standard Scan (Single Account)

Run CloudSlash to scan the default profile configured in `~/.aws/credentials`.

```bash
cloudslash
```

### Multi-Account Scan

To scan **every AWS account** defined in the local configuration:

```bash
cloudslash --license PRO-YOUR-KEY --all-profiles
```

- **Behavior**: Iterates through all profiles in `~/.aws/config`, checking for waste in each, and making a consolidated report.

### Cost Analysis

CloudSlash queries the AWS Price List API to calculate the monthly cost of identified waste.

- **Example**: A 50GB `gp2` volume unused for 30 days is estimated at ~$5.00/mo waste.

---

## 3. Advanced Features (v1.2)

### Terraform Code Auditor

Instead of modifying state, CloudSlash now tells you _where_ to delete code.

- **How it works**: Parses `terraform.tfstate` to map the Resource ID (e.g., `vol-0a...`) to a Terraform address (`aws_ebs_volume.logs`), then scans your `.tf` files to find the definition.
- **Output**: Displayed in the TUI as `Defined in: modules/storage/main.tf:45`.

### The Time Machine

Heuristic that finds snapshots of waste volumes.

- **Logic**: If Volume A is waste, and Snapshot B was created from Volume A, then Snapshot B is also waste.
- **Impact**: Recursively calculates hidden storage costs.

### Safety Brake (`cloudslash nuke`)

Interactive cleanup mode.

- **Usage**: `cloudslash nuke`
- **Flow**: Presents each waste item and asks for explicit confirmation before calling the AWS Delete API.

---

## 4. Artifacts and Remediation

After a scan completes, the `cloudslash-out/` directory contains the following artifacts:

### 1. Dashboard (`dashboard.html`)

Open in any web browser. Contains charts (Cost by Resource Type) and a sortable risk table.

### 2. Safeguard (`safe_cleanup.sh`)

Helper script for remediation.

- **Logic**: For every resource ID listed in the waste report, it attempts to create a snapshot or backup before deletion.

### 3. Terraform (`waste.tf`)

(Pro Feature) Generates Terraform code representing the waste resources, allowing them to be imported via `terraform import` managed destruction via IaC.

### 4. SaaS Killer Export (`waste_report.csv` / `.json`)

(Pro Feature) Generates raw data exports in CSV and JSON format, suitable for external consulting reports or custom dashboards.

### 5. Suppression (`ignore_resources.sh`)

If specific resources should be retained but excluded from reports:

1.  Review the `cloudslash-out/ignore_resources.sh` script.
2.  Run it: `bash cloudslash-out/ignore_resources.sh`
3.  It applies a `cloudslash:ignore=true` tag.
4.  Subsequent scans will respect this tag and exclude the resource from waste reports.

---

## 4. Heuristic Logic

| Resource          | Logic                                                           |
| :---------------- | :-------------------------------------------------------------- |
| **EBS Volumes**   | State is `available` OR attached to instance stopped > 30 days. |
| **EC2 Instances** | Max CPU < 5% over 7 days (Right-Sizing).                        |
| **NAT Gateways**  | Traffic < 1GB/mo AND Active Connections < 5.                    |
| **S3 Buckets**    | Incomplete Multipart Uploads > 7 days old.                      |
| **RDS**           | DB connections == 0 for 7 days OR status `stopped`.             |
| **Elastic IPs**   | Not attached to any network interface.                          |

---

## 5. Command Reference

| Flag                     | Description                                                            |
| :----------------------- | :--------------------------------------------------------------------- |
| `--license [KEY]`        | Activate Pro features.                                                 |
| `--region [REGION]`      | Override default region (e.g., `us-west-2`).                           |
| `--all-profiles`         | Scan all configured AWS profiles.                                      |
| `--tfstate [PATH]`       | Path to local `.tfstate` for drift detection.                          |
| `--required-tags [LIST]` | Comma-separated list of tags that must be present (e.g., `Env,Owner`). |
| `--slack-webhook [URL]`  | Send summary report to Slack.                                          |

### Commands

- **scan**: Run a headless scan without the TUI (useful for CI/CD).
- **update**: Update CloudSlash to the latest version.
- **completion**: Generate autocompletion script for shell.
