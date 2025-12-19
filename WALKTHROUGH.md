# CloudSlash User Guide

This guide details how to use CloudSlash to identify AWS infrastructure waste and remediate it safely.

---

## 1. Quick Start

### Installation

See the [README](README.md) for installation instructions for your OS.

### Authentication

CloudSlash uses standard AWS credentials found in `~/.aws/credentials` or `~/.aws/config`.

1.  **Configure AWS CLI**: `aws configure`
2.  **SSO Login** (if applicable): `aws sso login`
3.  **Permissions**: Ensure your user/role has `ReadOnlyAccess` and `ViewOnlyAccess`.

---

## 2. Workflows

### Standard Scan (Single Account)

Run CloudSlash to scan the default profile in `~/.aws/credentials`.

```bash
./cloudslash
```

### Multi-Account Scan ("God Mode")

To scan **every AWS account** defined in your local configuration:

```bash
./cloudslash -license PRO-YOUR-KEY -all-profiles
```

- **Behavior**: Iterates through all profiles in `~/.aws/config`, building a consolidated report of cross-account waste.

### Cost Analysis

CloudSlash queries the AWS Price List API to calculate the monthly cost of identified waste.

- **Example**: A 50GB `gp2` volume unused for 30 days = ~$5.00/mo waste.

---

## 3. Artifacts & Remediation

After a scan, check the `cloudslash-out/` directory:

### 1. Dashboard (`dashboard.html`)

Open in any browser. Contains charts (Cost by Resource Type) and a sortable risk table.

### 2. Safeguard (`safe_cleanup.sh`)

Helper script for remediation.

- **Logic**: For every resource ID listed in the waste report, it attempts to create a snapshot/backup before deletion.

### 3. Terraform (`waste.tf`)

(Pro Only) Generates Terraform code representing the waste resources, allowing you to `terraform import` them and manage their destruction via IaC.

### 4. Suppression (`ignore_resources.sh`)

If you want to keep the resources but stop CloudSlash from reporting them:

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

| Flag               | Description                                   |
| :----------------- | :-------------------------------------------- |
| `-license [KEY]`   | Activate Pro features.                        |
| `-region [REGION]` | Override default region (e.g., `us-west-2`).  |
| `-all-profiles`    | Scan all configured AWS profiles.             |
| `-tfstate [PATH]`  | Path to local `.tfstate` for drift detection. |
