# CloudSlash Operator Manual (v1.3.0)

Reference guide for execution, interpretation, and remediation.

---

## 1. Prerequisites

Before initiating the forensic engine, ensure the following environment configurations are met:

1.  **AWS Identity:** Valid credentials must be present in `~/.aws/credentials` or via the `AWS_PROFILE` environment variable.
2.  **IAM Scope:** The executing principal requires `ViewOnlyAccess` at minimum.
3.  **Terraform Bridge (Optional):** To use "The State Doctor" capabilities, Terraform CLI (v0.12+) must be installed and accessible in `$PATH`.

---

## 2. Chapter 1: The Scan Strategy

CloudSlash supports three distinct execution modes depending on the operational context.

### Mode A: Standard Interactive Scan

Used for manual auditing and deep-dive investigations. Launches the TUI.

```bash
cloudslash scan
```

### Mode B: CI/CD Headless

Used for automated pipelines (GitHub Actions, Jenkins). Returns Exit Code 0 (Clean) or 1 (Waste Detected).

```bash
cloudslash scan --headless
```

### Mode C: Cost-Savings Scan

Disables high-cost CloudWatch metric queries. Relies solely on metadata heuristics.

```bash
cloudslash scan --no-metrics
```

---

## 3. Chapter 2: Interpreting the Forensic Graph (The TUI)

The Terminal User Interface (TUI) visualizes the dependency graph of your infrastructure. It highlights "Hollow" resources—assets that exist but serve no functional purpose.

### The Live Dashboard

```text
 CLOUDSLASH PROTOCOL
 [ PRO MODE ACCESS ]

 [ POTENTIAL SAVINGS: $4,250.00 ]

 [FAIL] NAT Gateway: nat-0abc123
   ├─ Cost: $32.40/mo (Fixed) + $0.00 (Data)
   ├─ Traffic: 0 Bytes (Last 7 Days)
   ├─ Connected Subnet: subnet-xyz (3 Instances / All STOPPED)
   └─ Recommendation: DELETE (Hollow Gateway)

 [WARN] EIP: 54.200.100.10 (eipalloc-xyz)
   ├─ Cost: $3.60/mo
   └─ Reason: Unattached but Referenced in Route53 (DNS Danger)
```

- **Financial Ticker:** Updates in real-time as heuristics complete. Represents the monthly burn rate of identified waste.
- **Tree Structure:** Visualizes parent/child relationships (e.g., A NAT Gateway and its dependent Subnets).

### Key Bindings

| Key          | Action                                                                                   |
| :----------- | :--------------------------------------------------------------------------------------- |
| `Use Arrows` | Navigate the resource tree.                                                              |
| `Enter`      | Expand forensic details pane.                                                            |
| `i`          | **Ignore Resource.** Adds ID to `.cloudslash/ignore.yaml` and removes it from the graph. |
| `q`          | Quit application.                                                                        |

---

## 4. Chapter 3: The Terraform Bridge (Advanced)

CloudSlash v1.3.0 introduces "The State Doctor"—a heuristic engine that bridges the gap between AWS reality and your Terraform state.

**[WARN] Safety Notice**: CloudSlash does **not** parse raw `.tfstate` files, as this is prone to corruption. It utilizes `terraform show -json` to generate a safe, read-only representation of the state.

### The Remediation Workflow

1.  **Navigate** to the root of your Terraform project (where `.tf` files reside).
2.  **Execute** `cloudslash scan`.
3.  **Analyze** the output directory `cloudslash-out/`.

If "Zombie" resources (exists in AWS but not in State) or "Shadow" resources are found, CloudSlash generates `fix_terraform.sh`.

### Managing State Drift

The generated script surgically removes waste from your state file to preventing "Resource Not Found" errors during your next apply.

```bash
# 1. Review the generated script
cat cloudslash-out/fix_terraform.sh

# 2. Execute state surgery
./cloudslash-out/fix_terraform.sh
# Output: terraform state rm module.vpc.aws_nat_gateway.main

# 3. Confirm alignment
terraform plan
```

---

## 5. Chapter 4: Specific Forensics (The "Why")

This section details the logic behind v1.3.0's advanced heuristics.

### The Network Surgeon

**Detection:** "Hollow" NAT Gateways and "Safe-Release" EIPs.

- **Logic:** A NAT Gateway is considered "Hollow" if it has processed < 1GB of traffic in 30 days OR if all instances in its connected subnets are in a `STOPPED` state.
- **DNS Safety:** Elastic IPs are checked against Route53. If an unattached IP is still referenced in a DNS A-Record, CloudSlash marks it **[DANGER] Do Not Release**, preventing potential subdomain takeover attacks.

### The Storage Janitor

**Detection:** S3 Multipart Uploads and EBS Modernization.

- **S3 Invisible Waste:** Incomplete multipart uploads occupy storage but are invisible in the standard S3 Console file browser. CloudSlash queries the specific API to find these fragments.
- **EBS Modernization:** Detects legacy `gp2` volumes.
  - **Savings:** `gp3` is 20% cheaper per GB.
  - **Performance:** `gp2` IOPS are tied to size (3 IOPS/GB). `gp3` provides a flat 3000 IOPS baseline. CloudSlash calculates the exact performance gain (e.g., "Speed Boost: 2.5x").

---

## 6. Troubleshooting

**Error: "Permission Denied" (AWS)**

- **Cause:** The IAM User/Role lacks `ce:GetCostAndUsage` or specific `Describe*` permissions.
- **Fix:** Attach `ViewOnlyAccess` and `AWSBillingReadOnlyAccess` managed policies.

**Error: "Terraform State Not Found"**

- **Cause:** CloudSlash is running outside the directory containing the `.tfstate` file.
- **Fix:** `cd` into the infrastructure root before running the scan, or use the `--tfstate` flag to specify the path.
