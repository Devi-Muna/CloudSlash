# CloudSlash Technical Documentation (v1.2.5)

This document serves as the primary technical reference for the CloudSlash forensic engine. It details the system architecture, operational workflows, detection logic, and remediation protocols. It is intended for Site Reliability Engineers (SREs), DevOps practitioners, and FinOps stakeholders.

## 1. System Architecture

CloudSlash operates as a local-first, zero-trust forensic scanner. It diverges from traditional SaaS cost management platforms by executing entirely within the user's secure environment, requiring no external data exfiltration or long-term IAM role assumption by third parties.

### Core Components

1.  **Forensic Graph Engine**: CloudSlash constructs an in-memory Directed Acyclic Graph (DAG) of the target AWS environment. Nodes represent infrastructure assets (EC2 Instances, EBS Volumes, EKS Clusters), and edges represent functional dependencies (Attachment, Hosting, Security Group Association).
2.  **Telemetry Ingest**: The system queries AWS CloudWatch metrics directly to determine resource utilization (CPU, Network I/O, Connection Count) over a 30-day window, ensuring analysis is based on historical evidence rather than instantaneous state.
3.  **Heuristic Analyzers**: A suite of specialized logic modules runs against the graph to identify specific patterns of waste ("Zombie", "Ghost", "Orphan").

### Security Model

- **Execution scope**: Local CLI binary.
- **Authentication**: Uses the standard AWS credential provider chain (`~/.aws/credentials`, `AWS_PROFILE`, IAM Instance Profiles).
- **Permissions**: Read-only access (`ViewOnlyAccess`) is sufficient for scanning. Remediation features require specific `Delete` permissions.

## 2. CLI Command Reference

### Scan Operation

The primary operation mode. Scans the target AWS region, builds the graph, and outputs a summary to the terminal.

```bash
cloudslash scan [flags]
```

**Flags:**

- `--region`: Specify the target AWS region (e.g., `us-east-1`). Defaults to the environment variable configuration.
- `--headless`: Runs without the interactive Terminal User Interface (TUI), suitable for CI/CD logs.

### Export Operation

Generates detailed forensic artifacts for offline analysis or auditing.

```bash
cloudslash export
```

**Generated Artifacts (in `./cloudslash-out`):**

- `dashboard.html`: A self-contained, interactive HTML report visualizing cost distribution and specific waste items.
- `waste_report.csv`: A tabular dataset containing Resource IDs, Risk Scores, and Estimated Monthly Costs.
- `waste_report.json`: A hierarchical JSON export of the waste graph for programmatic integration.

### Remediation Operation (Safety Brake)

Interactive cleanup protocol.

```bash
cloudslash nuke
```

**Safety Protocol:**

1.  **Re-Verify**: The system performs a fresh scan to ensure the state has not drifted since the last report.
2.  **Interactive Confirmation**: The user is presented with each identified waste resource sequentially.
3.  **Explicit Consent**: Deletion API calls require affirmative user input (`y/N`) for every individual resource.

### Update Operation

Updates the local binary to the latest version, ensuring access to the newest heuristics and pricing data.

```bash
cloudslash update
```

## 3. Heuristic Detection Logic

CloudSlash v1.2.5 includes advanced heuristics designed to detect subtle forms of infrastructure waste that escape simple tag-based filtering.

### Trap Door Analysis (Fargate Profiles)

**Introduced in v1.2.5**

This heuristic targets "Abandoned" Fargate Profiles in Amazon EKS. A Fargate Profile acts as a configuration listener; if no Pods match its selectors, it represents configuration debt and a risk of accidental billing.

**Detection Layers:**

1.  **Broken Link Check**: Verifies if the Kubernetes Namespace defined in the profile selector exists. If the namespace is missing, the profile is flagged as broken/abandoned.
2.  **Utilization Pulse**: Queries for active Pods in the target namespace that match the profile's labels. If valid pods exist, the profile is active.
3.  **Ghost Town Forensics**: If no pods exist, the system analyzes Controllers (Deployments, StatefulSets). If no controllers match the profile selectors, or if all matching controllers have been scaled to zero for an extended period, the profile is deemed abandoned.
4.  **Allow-List**: Profiles acting on the `kube-system` namespace or named `fp-default` are automatically excluded to prevent disruption of cluster-critical infrastructure.

### Ghost Detector (EKS Node Groups)

**Introduced in v1.2.4**

Identifies EKS Node Groups that incur compute costs but serve zero generic user workloads. This addresses "Hollow Clusters" where scaling logic has failed to scale down empty groups.

**Detection Logic:**

1.  **Node Enumeration**: Lists all nodes associated with a specific Node Group.
2.  **Pod Filtration**: Examines all pods running on these nodes, filtering out:
    - DaemonSet pods (infrastructure overhead).
    - System pods (CoreDNS, VPC CNI).
    - Completed Jobs/Pods.
3.  **Verdict**: If a Node Group contains active EC2 instances but runs zero qualifying application workloads, it is flagged as a Ghost Node Group.

### Zombie Control Planes (EKS Clusters)

**Introduced in v1.2.3**

Detects EKS Clusters that consist only of a Control Plane (costing ~$72/month) with no attached compute capacity.

**Detection Logic:**

1.  **Age Threshold**: Ignores clusters created within the last 7 days (provisioning window).
2.  **Capacity Triangulation**: Checks for the absence of:
    - Managed Node Groups.
    - Fargate Profiles.
    - Self-Managed EC2 instances marked with the cluster's ownership tags.
3.  **Verdict**: If all capacity checks return negative, the cluster is flagged as a Zombie.

### Vampire NAT Gateways

Identifies NAT Gateways that incur high hourly charges but process negligible traffic.

**Thresholds:**

- **Traffic Volume**: Less than 1 GB of data processed over a 30-day window.
- **Verdict**: High fixed cost with low utilization suggests an architectural inefficiency (e.g., a NAT Gateway for a subnet that has no active outbound traffic requirements).

### Orphaned Load Balancers

Detects Elastic Load Balancers (ALB/NLB) that persist after their parent EKS cluster has been deleted.

**Logic:**

1.  **Tag Inspection**: Identifies Resources tagged with `kubernetes.io/cluster/<name>`.
2.  **Parent Verification**: Queries EKS Service to verify if cluster `<name>` exists.
3.  **Verdict**: If the parent cluster is not found, the Load Balancer is confirmed as orphaned waste.

## 4. Advanced Suppression System

## 4. Advanced Suppression System

CloudSlash allows you to "Safelist" resources so they are ignored during scans. This is done by applying a standard AWS Tag to the resource itself (via Console, CLI, or Terraform). CloudSlash reads these tags during its discovery phase.

### How to Ignore a Resource

**Option 1: AWS Console**

1.  Navigate to the resource (e.g., EC2 Dashboard -> Instances).
2.  Select the resource and click **Tags** -> **Manage Tags**.
3.  Add New Tag:
    - **Key**: `cloudslash:ignore`
    - **Value**: `true` (or see options below)
4.  Save. The next `cloudslash scan` will automatically skip this resource.

**Option 2: Terraform / IaC**
Simply add the tag to your resource definition:

```hcl
resource "aws_instance" "bastion" {
  # ... other config ...
  tags = {
    "cloudslash:ignore" = "true"
  }
}
```

### Tag Value Options

Apply the tag `cloudslash:ignore` with one of the following values to control behavior:

| Value Format | Behavior                                                                             | Use Case                                                                     |
| :----------- | :----------------------------------------------------------------------------------- | :--------------------------------------------------------------------------- |
| `true`       | **Permanent Ignore**<br>Resource is completely hidden from the CLI and all reports.  | Critical infrastructure with irregular usage patterns (e.g., Bastion Hosts). |
| `YYYY-MM-DD` | **Date Expiry**<br>Resource is ignored until the specified date.                     | Temporary projects or proof-of-concept resources.                            |
| `30d`        | **Relative Grace Period**<br>Resource is ignored if created within the last 30 days. | New deployments that are still being configured.                             |
| `cost<15.00` | **Cost Threshold**<br>Resource is ignored if monthly cost is below the value.        | Ignoring low-value noise to focus on high-impact savings.                    |

## 5. Remediation Scripts

In addition to the interactive `nuke` command, CloudSlash generates remediation artifacts for Infrastructure-as-Code workflows.

- **Terraform Remediation**: The `fix_terraform.sh` script parses the local `terraform.tfstate` and generates `terraform state rm` commands. This allows engineers to safely decouple waste resources from state management before physical deletion, preventing state-drift errors.
- **Owner Traceability**: Reports include the IAM Principal responsible for the resource creation (extracted from CloudTrail `RunInstances` or equivalent events), facilitating accountability.

---

_Generated by CloudSlash Documentation Engine._
