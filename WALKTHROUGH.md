# CloudSlash Operator's Guide (v1.3.0)

This is the manual for CloudSlash. It covers how to run the scan, read the output, and fix the problems safely.

## 1. Prerequisites

You need three things to get started:

1.  **AWS Credentials:** Standard `~/.aws/credentials` or `AWS_PROFILE` set in your terminal.
2.  **Permissions:** A user/role with `ViewOnlyAccess`. We don't need Write access just to look.
3.  **Terraform CLI:** (Optional) If you want to fix state drift, you need terraform (v0.12+) installed locally.

## 2. Scan Strategy

Pick the mode that fits your current workflow.

### Standard Scan (Interactive)

This is the default. It launches the Terminal UI (TUI) so you can explore the graph manually.

```bash
cloudslash scan
```

### Headless Mode (CI/CD)

Use this for pipelines (GitHub Actions, Jenkins). It prints no UI, just logs. Returns exit 0 if clean, exit 1 if waste is found.

```bash
cloudslash scan --headless
```

### Cost-Save Mode (Fast)

Skips the expensive CloudWatch API calls (metrics). It only checks metadata (tags, status). Much faster, but less "forensic."

```bash
cloudslash scan --no-metrics
```

## 3. The Forensic Graph (TUI)

The TUI isn't just a list; it's a **Dependency Tree**. We map the "Waste" to its "Parent" so you can see the blast radius before you delete anything.

### The Dashboard

```text
 [ POTENTIAL SAVINGS: $4,250.00 ]

 [FAIL] NAT Gateway: nat-0abc123
  ├─ Cost: $32.40/mo (Fixed)
  ├─ Traffic: 0 Bytes (Last 7 Days)
  ├─ Connected Subnet: subnet-xyz (3 Instances / All STOPPED)
  └─ Recommendation: DELETE (Hollow Gateway)
```

- **The Ticker:** That green number at the top is your monthly burn rate. It updates live.
- **The Tree:** Notice how `subnet-xyz` is nested under the NAT Gateway? That tells you why we flagged it.

### Controls

- **Arrows:** Move up/down.
- **Enter:** Drill down for details (IDs, ARNs, Tags).
- **i:** Ignore. Adds the resource to your local allowlist (`.cloudslash/ignore.yaml`).
- **q:** Quit.

## 4. The Terraform Bridge

This is the v1.3.0 killer feature. We map the "Zombie" AWS resources back to your local Terraform state.

**Safety First:** We do **not** touch your `.tfstate` file directly. That’s dangerous. Instead, we run `terraform show -json` to get a clean, safe readout of what Terraform thinks exists.

### The Fix Workflow

1.  **Go to your project root:** `cd /path/to/my-infra`
2.  **Run the scan:** `cloudslash scan`
3.  **Check the output:** Look in the `cloudslash-out/` folder.

If we find drift, we generate a script called `fix_terraform.sh`.

```bash
# 1. Review the script (Always review auto-generated code)
cat cloudslash-out/fix_terraform.sh

# 2. Run the surgery
# This runs 'terraform state rm' commands for you
./cloudslash-out/fix_terraform.sh

# 3. Verify
terraform plan
```

## 5. Specific Forensics

Here is the logic regarding why we flag certain things.

### The Network Surgeon

**1. The "Hollow" NAT Gateway**

A NAT Gateway costs ~$32/month just to exist.

- **Our Logic:** If traffic < 1GB OR the subnets behind it are empty/stopped, it's a zombie. You are paying for a door that nobody is walking through.

**2. The "Safe-Release" EIP**

- **The Risk:** If you release an Elastic IP (EIP) back to AWS, but your DNS (Route53) still points `api.company.com` to it, a hacker can claim that IP and hijack your traffic.
- **Our Check:** We cross-reference Route53. If the IP is in a Record Set, we mark it **[DANGER]**. Do not release until you fix DNS.

### The Storage Janitor

**1. S3 Multipart Debris**

- **The Trap:** When a huge upload fails, S3 keeps the chunks "just in case" you want to resume. These chunks cost money but are invisible in the file browser.
- **The Fix:** We query the low-level API to find these ghosts.

**2. EBS Modernization (gp2 -> gp3)**

- **The Math:** `gp2` is legacy. `gp3` is strictly better.
- **The Upsell:** Switching saves 20% on storage costs AND unlinks IOPS from size. We calculate the exact savings and performance boost for you.

## 6. Troubleshooting

- **"Permission Denied" errors?** You are likely missing `ce:GetCostAndUsage`. Ensure your IAM user has the `ViewOnlyAccess` managed policy.
- **"Terraform State Not Found"?** You need to be in the folder that has the `.tfstate` (or the `.terraform` folder). If you use remote state (S3 backend), make sure you have run `terraform init` recently so your local cache is fresh.

---

## Appendix: Heuristics Catalog (The Encyclopedia)

Reference guide to all 7 Forensic Engines in CloudSlash v1.3.0.

### 1. Compute & Serverless

| Detection                | Condition                                                                         | Status   |
| :----------------------- | :-------------------------------------------------------------------------------- | :------- |
| **Lambda Code Rot**      | Invocations (90d) == 0 **AND** Last Modified > 90d                                | `[FAIL]` |
| **Lambda Version Bloat** | >5 Versions exist that are NOT `$LATEST` or Aliased.                              | `[WARN]` |
| **ECS Idle Cluster**     | Cluster has EC2 Instances (>1h uptime) but **0 Tasks** running.                   | `[FAIL]` |
| **ECS Crash Loop**       | Service Desired Count > 0 but Running Count == 0. (Checks for OOM/Image Missing). | `[FAIL]` |

### 2. Data & Storage

| Detection                     | Condition                                                                  | Status   |
| :---------------------------- | :------------------------------------------------------------------------- | :------- |
| **Redshift Pause**            | 0 Queries in last 24h.                                                     | `[WARN]` |
| **Ghost Cache (ElastiCache)** | 0 Hits/Misses (7d) **AND** CPU < 2% **AND** Network < 5MB/week.            | `[FAIL]` |
| **DynamoDB Bleed**            | Provisioned Mode with utilization < 15%. Recommends On-Demand switch.      | `[WARN]` |
| **EBS Legacy**                | Volume Type is `gp2`. Recommends `gp3` (20% Cheaper + 3000 IOPS baseline). | `[WARN]` |
| **S3 Multipart**              | Incomplete Upload parts > 7 days old.                                      | `[WARN]` |

### 3. Network & Infrastructure

| Detection                | Condition                                                              | Status   |
| :----------------------- | :--------------------------------------------------------------------- | :------- |
| **Hollow NAT**           | Traffic < 1GB (30d) **OR** Connected Subnets have 0 Running Instances. | `[FAIL]` |
| **Zombie EIP**           | Unattached EIP. Checks Route53 for DNS hazards before flagging Safe.   | `[FAIL]` |
| **Orphan Load Balancer** | ELB exists but target instances/targets don't exist.                   | `[FAIL]` |

### 4. Containers & Ops

| Detection               | Condition                                                                  | Status   |
| :---------------------- | :------------------------------------------------------------------------- | :------- |
| **ECR Digital Janitor** | Repo has **No Lifecycle Policy** AND Images are Untagged + Unpulled > 90d. | `[WARN]` |
| **Log Hoarder**         | Retention = "Never" AND Size > 1GB.                                        | `[WARN]` |
| **Abandoned Log**       | Stored Bytes > 0 BUT Incoming Bytes == 0 (30d).                            | `[FAIL]` |
