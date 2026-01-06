# CloudSlash Operator's Manual (v1.3.3)

This manual outlines the operational workflows for CloudSlash: executing scans, interpreting the dependency graph, and performing safe remediation of Terraform state.

## 1. Prerequisites

Ensure the following are configured before execution:

- **AWS Credentials:** A valid session via `~/.aws/credentials` or `AWS_PROFILE`.
- **IAM Permissions:** `ViewOnlyAccess` is sufficient for analysis. `ce:GetCostAndUsage` is recommended for accurate pricing.
- **Terraform CLI:** (Optional) Required locally (v0.12+) only if generating state remediation scripts.

## 2. Execution Modes

CloudSlash is designed for both interactive exploration and automated pipelines.

### **Interactive Dashboard (TUI)**

The default mode. Best for investigating complex dependencies and exploring your account's waste topology.

```bash
cloudslash
```

### **Headless Mode (CI/CD)**

Run cloudslash in your GitHub Actions or Jenkins pipelines. It bypasses the UI and generates exit codes based on findings.

- **Exit 0**: Clean System.
- **Exit 1**: Waste Detected.

```bash
cloudslash scan --headless
```

### **Metadata-Only Scan (Fast)**

Bypasses CloudWatch metrics API calls. Useful for rapid iteration when you only care about configuration state (e.g., "Is this EIP attached?") rather than utilization (e.g., "Is this CPU idle?").

```bash
cloudslash scan --no-metrics
```

## 3. The Dashboard (TUI) Explained

CloudSlash visualizes resources as a **Dependency Tree**. This allows you to evaluate the "blast radius" of a resource before removal.

**Dashboard Layout**

```plaintext
 [ CLOUDSLASH v1.3.3 ] [ TASKS: 12/40 ] [ WASTE: $4,250.00/mo ] [ RISK: HIGH ]

 [FAIL] NAT Gateway: nat-0abc123
  ├─ Cost: $32.40/mo (Fixed Rate)
  ├─ Traffic: 0 Bytes (Last 7 Days)
  ├─ Connected Subnet: subnet-xyz (3 Instances / State: STOPPED)
  └─ Recommendation: DELETE (Idle Gateway)
```

- **Cost Estimate:** Real-time calculation of potential monthly savings.
- **Risk Score:** Heuristic assessment of how "dangerous" the current state is (e.g., Dangling DNS = Critical).
- **Dependency Context:** Shows _why_ a resource is flagged. In the example above, the NAT Gateway is flagged not just because it has low traffic, but because its connected instances are stopped.

**Keyboard Controls**

| Key       | Action       | Description                                        |
| :-------- | :----------- | :------------------------------------------------- |
| `↑` / `↓` | **Navigate** | Scroll through the waste list.                     |
| `Enter`   | **Details**  | Open the Detail View (Sparklines, Tags, JSON).     |
| `i`       | **Ignore**   | Suppress this resource (Adds to `.ignore.yaml`).   |
| `m`       | **Mark**     | Soft-delete mark (Simulated).                      |
| `y`       | **Copy ID**  | Copy Resource ID (e.g., `i-0123...`) to clipboard. |
| `o`       | **Open**     | Open resource in AWS Console (Browser).            |
| `q`       | **Quit**     | Exit CloudSlash.                                   |

## 4. Remediation Workflows

CloudSlash does not strictly _enforce_ deletion it empowers you to make the decision.

### **Workflow A: The "State Doctor" (Terraform Users)**

If your infrastructure is managed by Terraform, deleting resources manually in the AWS Console will break your state file. CloudSlash solves this.

1.  **Run Scan in Repo Root:**
    ```bash
    cd /path/to/my-infra
    cloudslash
    ```
2.  **Inspect Output:**
    CloudSlash detects your `.tfstate` and cross-references it with the waste it found.
3.  **Execute Fix Script:**
    A script is generated at `cloudslash-out/fix_terraform.sh`.

    ```bash
    # Review changes
    cat cloudslash-out/fix_terraform.sh

    # Apply changes (Removes "zombies" from state)
    ./cloudslash-out/fix_terraform.sh
    ```

4.  **Verify:**
    ```bash
    terraform plan
    ```
    Your plan should now show that Terraform _agrees_ the resources are gone, rather than trying to recreate them.

### **Workflow B: The "Janitor" (Console/ClickOps Users)**

For resources not managed by Terraform (or click-ops accounts).

1.  **Generate Scripts:**
    CloudSlash creates `cloudslash-out/safe_cleanup.sh`.
2.  **Run Cleanup:**
    ```bash
    ./cloudslash-out/safe_cleanup.sh
    ```
    _Note: This script uses the AWS CLI to delete resources. Ensure you have permissions._

## 5. Heuristics Catalog

A reference guide to the detection engines utilized by CloudSlash.

### **Compute & Serverless**

| Detection                  | Logic                                                           | Remediation                               |
| :------------------------- | :-------------------------------------------------------------- | :---------------------------------------- |
| **Lambda Code Stagnation** | 0 Invocations (90d) AND Last Modified > 90d.                    | Delete function or archive code to S3.    |
| **ECS Idle Cluster**       | EC2 instances running for >1h but Cluster has 0 Tasks/Services. | Scale ASG to 0 or delete Cluster.         |
| **ECS Crash Loop**         | Service Desired Count > 0 but Running Count == 0.               | Check Task Definitions / ECR Image pulls. |

### **Storage & Database**

| Detection            | Logic                                                   | Remediation                                    |
| :------------------- | :------------------------------------------------------ | :--------------------------------------------- |
| **Zombie EBS**       | Volume state is `available` (unattached) for > 14 days. | Snapshot (optional) then Delete.               |
| **Legacy EBS (gp2)** | Volume is `gp2`. `gp3` is 20% cheaper and decoupled.    | Modify Volume to `gp3` (No downtime).          |
| **Fossil Snapshots** | RDS/EBS Snapshot > 90 days old, not attached to AMI.    | Delete old snapshots.                          |
| **RDS Idle**         | 0 Connections (7d) AND CPU < 5%.                        | Stop instance or take final snapshot & delete. |

### **Network & Security**

| Detection                   | Logic                                                              | Remediation                                     |
| :-------------------------- | :----------------------------------------------------------------- | :---------------------------------------------- |
| **Hollow NAT Gateway**      | Traffic < 1GB (30d) OR Connected Subnets have 0 Running Instances. | Delete NAT Gateway.                             |
| **Dangling EIP (Critical)** | EIP unattached but matches an A-Record in Route53.                 | **URGENT**: Update DNS first, then release EIP. |
| **Orphaned ELB**            | Load Balancer has 0 registered/healthy targets.                    | Delete ELB.                                     |

### **Containers**

| Detection                 | Logic                                               | Remediation                                     |
| :------------------------ | :-------------------------------------------------- | :---------------------------------------------- |
| **ECR Lifecycle Missing** | Repo has images > 90d old but no expiration policy. | Add Lifecycle Policy to expire untagged images. |
| **Log Retention Missing** | CloudWatch Group set to "Never Expire" (>1GB size). | Set retention to 30d/90d.                       |

## 6. Troubleshooting

- **"Permission Denied"**: Ensure your IAM User/Role has `ViewOnlyAccess`. for Cost data, `ce:GetCostAndUsage` is required.
- **"Terraform State Not Found"**: Run CloudSlash from the directory containing your `terraform.tfstate` or `.terraform/` folder.
- **"TUI Distortion"**: If the dashboard looks broken, try resizing your terminal or ensuring a Nerd Font is installed (though standard fonts should work).
