# CloudSlash User Documentation

This reference manual documents the operational capabilities of CloudSlash, a zero-trust infrastructure analysis engine designed to identify and eliminate cloud waste with forensic precision.

---

## 1. Core Architecture

CloudSlash is deployed as a single, static binary with no external dependencies. The core engine is built on four pillars:

### Robust Installation Strategy

The installation subsystem (introduced in v1.2.2) employs a pipe-safe execution wrapper. This ensures that network interruptions or `sudo` password prompts do not truncate the script execution stream. It dynamically detects the host operating system (Darwin/Linux/Windows) and CPU architecture (AMD64/ARM64) to fetch the optimized binary, displaying real-time download progress via a standard progress bar.

### Dynamic Update System

The tool includes a self-awareness mechanism that checks the remote repository for new releases. It intelligently distinguishes between `stable` releases and `pre-release` (beta) tags, ensuring users can opt into bleeding-edge features without breaking production workflows. Run `cloudslash update` to perform an in-place upgrade.

### Zero-Dependency Design

CloudSlash requires no language runtimes (Python, Node.js) or external libraries. It communicates directly with the AWS API using the standard SDK, authenticating via the default `~/.aws/credentials` chain.

---

## 2. Cost Intelligence Subsystem

CloudSlash moves beyond simple resource counting by integrating directly with the AWS Price List API.

### Real-Time Valuation

For every identified waste resource, the engine queries the current on-demand pricing for that specific region and instance type. This provides a dollar-accurate valuation of the waste, rather than a generic estimate.

### Burn Rate Forensics

- **Daily Burn Rate**: Calculates the exact amount of capital lost every 24 hours.
- **Annual Projection**: Extrapolates the daily waste to a 365-day forecast, accounting for monthly storage costs and hourly instance rates.

---

## 3. Forensic Audit Capabilities

CloudSlash maps resources not just to their technical state, but to their human origins and code definitions.

### Owner Identification ("The Blame Game")

The engine correlates resource metadata with CloudTrail logs. It searches for `RunInstances`, `CreateVolume`, and similar events to identify the specific IAM User or Role responsible for creating the waste.

- **Output**: displayed in the TUI as `Creator: arn:aws:iam::123:user/jdoe`.

### Reverse-Terraform Mapping

This feature bridges the gap between the AWS Console and Infrastructure-as-Code.

1. The engine scans the local `terraform.tfstate` file.
2. It maps the AWS Resource ID (e.g., `vol-0af...`) to its Terraform resource address (e.g., `module.db.aws_ebs_volume.main`).
3. It generates a `fix_terraform.sh` script containing precise `terraform state rm` commands to surgically remove the waste from state without affecting other infrastructure.

### Fossil Snapshot Detection

Standard tools often miss "Fossil" snapshots—RDS or EBS snapshots that are no longer linked to an active volume or cluster. CloudSlash constructs a dependency graph to identify these orphaned chains, flagging snapshots that have no lineage to a live production resource.

### Silent Killer Detection

The engine heuristically identifies resources that accrue cost silently:

- **Unattached NAT Gateways**: These incur hourly charges even with zero traffic.
- **Massive Log Groups**: CloudWatch Log Groups exceeding 1GB that have not been accessed or rotated.

---

## 4. Remediation Workflows

CloudSlash enforces a "Safety First" philosophy. By default, it operates in strict Read-Only mode.

### Usage Flow

1.  **Scan**: Execute `cloudslash` to analyze the environment.
    - _Result_: A localized graph graph in memory and a TUI report. No data leaves the machine.
2.  **Review**: Analyze the interactive dashboard and `waste_report.csv`.
3.  **Suppress**: If a resource is valid (e.g., a Disaster Recovery copy), use the generated suppression script.
    - Run `bash cloudslash-out/ignore_resources.sh`.
    - This applies the `cloudslash:ignore=true` tag. Future scans will skip these resources.
4.  **Fix (Code)**: For resources managed by Terraform, execute `bash cloudslash-out/fix_terraform.sh`.
5.  **Fix (Direct)**: For unmanaged resources, use the interactive nuke command.

### Interactive Nuke (`cloudslash nuke`)

This command initiates the cleanup sequence. It iterates through the identified waste list, presenting a final confirmation prompt for each item. This functions as a "dead man's switch," preventing accidental deletion of critical data.

---

## 5. Artifact Reference

Analysis results are persisted to the `cloudslash-out/` directory:

| Artifact              | Detection     | Description                                                   |
| :-------------------- | :------------ | :------------------------------------------------------------ |
| `dashboard.html`      | Visualization | Client-side HTML dashboard with charts and risk heatmaps.     |
| `waste_report.csv`    | Data Export   | Raw CSV export suitable for ingestion into Excel or BI tools. |
| `fix_terraform.sh`    | Remediation   | Shell script to remove waste from Terraform state.            |
| `ignore_resources.sh` | Suppression   | Script to tag identified resources as ignored.                |
| `safe_cleanup.sh`     | Remediation   | Legacy script for snapshot-before-delete workflows.           |

---

## 6. Heuristic Logic Reference

| Resource          | Detection Logic                                                   |
| :---------------- | :---------------------------------------------------------------- |
| **EBS Volumes**   | State is `available` OR attached to `stopped` instance > 30 days. |
| **EC2 Instances** | Max CPU utilization < 5% over 7 days (Right-Sizing).              |
| **NAT Gateways**  | Data processed < 1GB/month AND Active Connections < 5.            |
| **S3 Buckets**    | Contains incomplete multipart uploads older than 7 days.          |
| **RDS Clusters**  | Zero active connections for 7 days OR status is `stopped`.        |
| **Elastic IPs**   | Valid public IP not associated with any network interface.        |
