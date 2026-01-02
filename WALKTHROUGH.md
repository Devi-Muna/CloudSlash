# CloudSlash Guide (v1.3.0)

This manual outlines the operational workflows for CloudSlash: executing scans, interpreting the dependency graph, and performing safe remediation of Terraform state.

## 1. Prerequisites

Ensure the following are configured before execution:

- **AWS Credentials:** A valid session via `~/.aws/credentials` or `AWS_PROFILE`.
- **IAM Permissions:** The `ViewOnlyAccess` managed policy is sufficient for analysis.
- **Terraform CLI:** (Optional) Required locally (v0.12+) only if generating state remediation scripts.

## 2. Scan Modes

Select the execution mode based on your environment.

### Standard Scan (Interactive)

The default mode launches a Terminal UI (TUI) for interactive exploration of resource dependencies.

```bash
cloudslash scan
```

### Headless Mode (CI/CD)

Designed for automated pipelines (GitHub Actions, Jenkins). It outputs raw logs and exit codes (Exit 0: Clean, Exit 1: Waste Detected).

```bash
cloudslash scan --headless
```

### Metadata-Only Scan (Fast)

Bypasses CloudWatch metrics API calls to reduce execution time. Checks are limited to metadata (tags, status, configuration) rather than utilization.

```bash
cloudslash scan --no-metrics
```

## 3. The Resource Dependency Graph (TUI)

CloudSlash visualizes resources as a **Dependency Tree**. This allows you to evaluate the "blast radius" of a resource before removal by seeing its parent/child relationships.

**Dashboard Overview**

```plaintext
 [ ESTIMATED MONTHLY WASTE: $4,250.00 ]

 [FAIL] NAT Gateway: nat-0abc123
  ├─ Cost: $32.40/mo (Fixed Rate)
  ├─ Traffic: 0 Bytes (Last 7 Days)
  ├─ Connected Subnet: subnet-xyz (3 Instances / State: STOPPED)
  └─ Recommendation: DELETE (Idle Gateway)
```

- **Cost Estimate:** Real-time calculation of potential monthly savings.
- **Dependency Context:** In the example above, the NAT Gateway is flagged because the instances in its connected subnet are stopped, rendering the gateway redundant.

**Controls**

- **Navigation:** Arrow Keys (Up/Down)
- **Drill Down:** Enter (View IDs, ARNs, Tags)
- **Suppress:** `i` (Adds resource to local `.cloudslash/ignore.yaml`)
- **Exit:** `q`

## 4. Terraform State Reconciliation

CloudSlash maps identified AWS resources back to your local Terraform state to facilitate cleanup.

> **Safety Mechanism:** CloudSlash never modifies `.tfstate` files directly. It utilizes `terraform show -json` to safely inspect the current state.

**Remediation Workflow**

1. **Navigate to Project Root:**

   ```bash
   cd /path/to/my-infra
   ```

2. **Execute Scan:**

   ```bash
   cloudslash scan
   ```

3. **Review Output:** If drift is detected, a script is generated in the `cloudslash-out/` directory.

   ```bash
   # 1. Audit the generated script
   cat cloudslash-out/fix_terraform.sh

   # 2. Execute Remediation
   # This executes 'terraform state rm' for identified resources
   ./cloudslash-out/fix_terraform.sh

   # 3. Verify Consistency
   terraform plan
   ```

## 5. Detection Logic & Heuristics

Detailed logic for specific high-value checks.

### Network Optimization

**1. Idle NAT Gateways**

- **Cost Impact:** ~$32/month base cost + data processing fees.
- **Logic:** Flagged if traffic < 1GB (30d) OR if all backend instances in connected subnets are in a terminated/stopped state.

**2. Elastic IP DNS Safety Check**

- **Risk:** "Subdomain Takeover." If an EIP is released to AWS but remains in your Route53 DNS records, a third party could claim the IP and serve traffic on your domain.
- **Logic:** Cross-references EIPs against Route53 A-records. If a match is found, the EIP is marked **[DANGER]**. DNS records must be updated before release.

### Storage Hygiene

**1. Incomplete S3 Multipart Uploads**

- **Issue:** Failed large file uploads leave hidden data chunks in S3 that incur storage costs but do not appear in standard file listings.
- **Detection:** Queries the `ListMultipartUploads` API for incomplete segments older than 7 days.

**2. EBS Volume Optimization (gp2 vs gp3)**

- **Benefit:** gp3 volumes offer a 20% lower cost per GB and decouple IOPS from storage size compared to gp2.
- **Logic:** Identifies gp2 volumes and calculates the specific cost savings and IOPS performance gain of migrating to gp3.

## 6. Troubleshooting

- **"Permission Denied" errors:** Ensure your IAM user possesses the `ce:GetCostAndUsage` permission in addition to `ViewOnlyAccess`.
- **"Terraform State Not Found":** Ensure you are executing the CLI from the directory containing `.tfstate` (or the `.terraform` directory). For remote backends (S3), run `terraform init` to refresh the local cache.

---

# Appendix: Heuristics Catalog

A reference guide to the detection engines available in v1.3.0.

### 1. Compute & Serverless

| Detection                  | Condition                                                                      | Severity |
| :------------------------- | :----------------------------------------------------------------------------- | :------- |
| **Lambda Code Stagnation** | 0 Invocations (90d) AND Last Modified > 90d                                    | [FAIL]   |
| **Lambda Version Bloat**   | >5 Versions exist (excluding $LATEST or Aliased versions)                      | [WARN]   |
| **ECS Idle Cluster**       | Cluster contains active EC2 Instances (>1h uptime) but 0 Tasks running         | [FAIL]   |
| **ECS Crash Loop**         | Service Desired Count > 0 but Running Count == 0 (Checks for OOM/Image errors) | [FAIL]   |

### 2. Data & Storage

| Detection                      | Condition                                                      | Severity |
| :----------------------------- | :------------------------------------------------------------- | :------- |
| **Redshift Inactivity**        | 0 Queries executed in last 24h                                 | [WARN]   |
| **ElastiCache Idle**           | 0 Hits/Misses (7d) AND CPU < 2% AND Network < 5MB/week         | [FAIL]   |
| **DynamoDB Over-provisioning** | Provisioned Mode with utilization < 15% (Recommends On-Demand) | [WARN]   |
| **EBS Legacy Type**            | Volume Type is `gp2` (Recommends `gp3` upgrade)                | [WARN]   |
| **S3 Multipart Abandonment**   | Incomplete Upload parts > 7 days old                           | [WARN]   |

### 3. Network & Infrastructure

| Detection                  | Condition                                                         | Severity |
| :------------------------- | :---------------------------------------------------------------- | :------- |
| **Idle NAT Gateway**       | Traffic < 1GB (30d) OR Connected Subnets have 0 Running Instances | [FAIL]   |
| **Dangling EIP**           | Unattached EIP. (Includes Route53 cross-reference check)          | [FAIL]   |
| **Orphaned Load Balancer** | ELB exists but has no registered targets/instances                | [FAIL]   |

### 4. Containers & Operations

| Detection                 | Condition                                                             | Severity |
| :------------------------ | :-------------------------------------------------------------------- | :------- |
| **ECR Lifecycle Missing** | Repo has No Lifecycle Policy AND Images are Untagged + Unpulled > 90d | [WARN]   |
| **Log Retention Missing** | Retention = "Never" AND Group Size > 1GB                              | [WARN]   |
| **Abandoned Log Group**   | Stored Bytes > 0 BUT Incoming Bytes == 0 (30d)                        | [FAIL]   |
