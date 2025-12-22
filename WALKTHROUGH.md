# CloudSlash Technical Manual (v1.2.3)

This document describes the operational procedures, core architecture, and usage workflows of CloudSlash. It serves as the primary reference for system administrators and DevOps engineers using the tool for infrastructure analysis and forensic cleanup.

## 1. System Overview

CloudSlash functions as a local forensic analysis engine. Unlike SaaS platforms that require broad IAM roles and data export, CloudSlash runs entirely on your local machine or CI/CD runner. It builds an in-memory dependency graph of your AWS infrastructure to identify inefficiencies, security risks, and financial waste.

### Key Capabilities

- **Zero-Trust Analysis**: Verifies resource utilization via CloudWatch metrics rather than trusting static status tags.
- **Forensic Graphing**: Maps dependencies (e.g., Snapshots -> Volumes -> Instances) to safely identify orphaned resources without breaking active chains.
- **Code Correlation**: Traces identified waste back to its origin in your Terraform state files and CloudTrail logs.

## 2. Command Reference

CloudSlash provides a suite of commands tailored for interactive audit, automated scanning, and controlled remediation.

### The Scan Command (Default)

The `cloudslash` command (or `cloudslash scan`) initiates a forensic sweep of the environment. It authenticates using the standard AWS credential chain.

**Usage:**

```bash
cloudslash scan --region us-east-1
```

**Behavior:**

1.  **Discovery**: Enumerates resources across EC2, RDS, S3, and ELB.
2.  **Telemetry**: Fetches 30-day metric history (CPU, I/O, Network) for every asset.
3.  **Graph Construction**: Builds the dependency graph to calculate blast radius.
4.  **Reporting**: Outputs findings to the `cloudslash-out/` directory.

### The Export Command

The `cloudslash export` command allows you to generate forensic artifacts without launching the TUI. It is designed for integration with external BI tools or archiving systems.

**Usage:**

```bash
cloudslash export
```

**Artifacts Generated:**

- `waste_report.csv`: A structured dataset of all identified waste, including monthly cost estimates and risk scores.
- `waste_report.json`: A hierarchical JSON dump of the waste graph, suitable for programmatic parsing.
- `dashboard.html`: A standalone HTML report for visualized consumption.

### The Nuke Command (Safety Brake)

The `cloudslash nuke` command triggers the interactive remediation protocol. This is known as the "Safety Brake" mechanism. Unlike automated cleanups that can be dangerous, Nuke forces a manual verification step for every deletion.

**Usage:**

```bash
cloudslash nuke
```

**Workflow:**

1.  System re-scans the environment to ensure state is current.
2.  Identified waste is presented sequentially.
3.  User must explicitly confirm deletion (y/N) for each resource.
4.  Deletion API calls are issued immediately upon confirmation.

### The Update Command

To ensure you are using the latest heuristics and pricing models, use the built-in update mechanism.

**Usage:**

```bash
cloudslash update
```

## 3. Advanced Tagging & Suppression

CloudSlash supports a robust suppression system using the `cloudslash:ignore` tag. This allows operators to whitelist resources that are false positives (e.g., warm standbys) or temporary experimental resources.

### Suppression Logic

Apply the tag `cloudslash:ignore` with one of the following value formats to control suppression behavior:

**1. Permanent Ignore**

- **Value:** `true`
- **Effect:** The resource will never be flagged as waste.

**2. Absolute Date Expiry**

- **Value:** `YYYY-MM-DD` (e.g., `2025-12-31`)
- **Effect:** The resource is ignored until the specified date. After this date, it will be re-evaluated.

**3. Relative Grace Period (New)**

- **Value:** `Nd` or `Nh` (e.g., `30d`, `72h`)
- **Effect:** The resource is ignored if its creation time is within the specified duration (e.g., created less than 30 days ago). This is useful for "fresh" dev resources that haven't had time to prove utilization yet.

**4. Cost Threshold**

- **Value:** `cost<N` (e.g., `cost<10.00`)
- **Effect:** The resource is ignored if its monthly cost is below N dollars.

## 4. Remediation & Forensics

When waste is identified, CloudSlash provides multiple paths for remediation, respecting the "Infrastructure as Code" integrity.

### Terraform Logic (Reverse-Terraform)

For teams using Terraform, deleting resources via the AWS Console (or Nuke) introduces configuration drift. CloudSlash solves this by analyzing the local `terraform.tfstate` file.

**Mechanism:**
The engine generates a script `cloudslash-out/fix_terraform.sh`. This script contains precise `terraform state rm` commands targeting the specific resource addresses of identified waste. Running this script removes the waste from your state file, ensuring your next `terraform apply` effectively destroys the resource (or cleans up the reference).

### Owner Forensics (The Blame Game)

To prevent waste recurrence, you must identify the source. CloudSlash analyzes CloudTrail logs to find the IAM Principal responsible for creating the resource.

- **Data Point:** The `Owner` field in reports will display the IAM ARN (e.g., `arn:aws:iam::123:user/dev-ops`).
- **Scope:** Traces back to original `RunInstances`, `CreateVolume`, or `CreateBucket` events.

## 5. Artifact Reference

All analysis outputs are stored in the `./cloudslash-out` directory.

- `dashboard.html`: Visual report for stakeholders.
- `waste_report.csv`: Flat data file for inventory tracking.
- `waste_report.json`: Deep data file for automation.
- `fix_terraform.sh`: Script to align Terraform state.
- `ignore_resources.sh`: Script to automatically tag current waste as ignored (bulk suppression).

## 6. Detection Logic Details (New in v1.2.3)

CloudSlash employs multi-stage verification to ensure zero false positives.

### Zombie EKS Control Planes

EKS clusters incur a base cost of ~$72/month even without worker nodes. CloudSlash identifies these "Zombie Control Planes" using a composite check:

1.  **Status Check**: Cluster must be `ACTIVE` and created more than 7 days ago.
2.  **Capacity Verification (The Triad)**:
    - **Managed Node Groups**: Must have zero groups or groups with `desiredSize` 0.
    - **Fargate Profiles**: Must have zero profiles defined.
    - **Self-Managed Nodes**: We query all EC2 instances for the tag `kubernetes.io/cluster/<name>`. If 0 instances are found, the cluster is deemed empty.
3.  **Safety Net**: Clusters tagged with `karpenter.sh/discovery` are automatically ignored to protect auto-scaling environments that scale to zero.

## 7. Security Model

CloudSlash is designed with a "Least Privilege" mindset.

- **Read-Only**: The `scan` and `export` commands require only `ViewOnlyAccess` or `ReadOnlyAccess` permissions.
- **Local Execution**: All graph processing, cost calculation, and credential usage occur within the process memory. No data is transmitted to third-party servers.
- **Credential Chain**: It utilizes the standard AWS SDK credential provider chain (Env Vars -> Profile -> Instance Metadata), ensuring compatibility with existing MFA and SSO workflows.
