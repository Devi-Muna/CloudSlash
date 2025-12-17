# CloudSlash User Walkthrough 🗡️☁️

Welcome to **CloudSlash**, the forensic accountant for your AWS infrastructure. This guide will walk you through how to use the software to identify waste, estimate costs, and safely remediate "Shadow IT" across your entire organization.

---

## 1. Quick Start

### Installation

#### Option A: Quick Install (Developers)

If you have Go installed, you can build and install it directly from source in one command:

```bash
go install github.com/DrSkyle/CloudSlash/cmd/cloudslash@latest
```

Then just run `cloudslash` from anywhere!

#### Option B: Download Binary (No Dependencies)

If you don't want to install Go, just run these commands to download and run the binary.

**macOS (Apple Silicon / M1 / M2)**

```bash
curl -L -o cloudslash https://github.com/DrSkyle/CloudSlash/releases/latest/download/cloudslash-darwin-arm64
chmod +x cloudslash
./cloudslash
```

**macOS (Intel)**

```bash
curl -L -o cloudslash https://github.com/DrSkyle/CloudSlash/releases/latest/download/cloudslash-darwin-amd64
chmod +x cloudslash
./cloudslash
```

**Linux**

```bash
curl -L -o cloudslash https://github.com/DrSkyle/CloudSlash/releases/latest/download/cloudslash-linux-amd64
chmod +x cloudslash
./cloudslash
```

**Windows** (PowerShell)

```powershell
Invoke-WebRequest -Uri https://github.com/DrSkyle/CloudSlash/releases/latest/download/cloudslash-windows-amd64.exe -OutFile cloudslash.exe
.\cloudslash.exe
```

### Authentication

CloudSlash uses your standard AWS credentials. It looks for profiles in `~/.aws/credentials` and `~/.aws/config`.

- Ensure you have credentials set up via `aws configure` or `aws sso login`.
- **Security Note**: CloudSlash runs locally and requires only `ReadOnlyAccess` + `ViewOnlyAccess`. It does _not_ send data to the cloud.

---

## 2. Core Workflows

### 🔍 The "God Mode" Scan (Enterprise)

To scan **every AWS account** defined in your local configuration at once:

```bash
./cloudslash -license PRO-YOUR-KEY -all-profiles
```

- **What happens?**
  - CloudSlash iterates through every profile in `~/.aws/config`.
  - It builds a massive in-memory graph of your entire infrastructure.
  - It identifies cross-account waste and zombie resources.
  - **New**: A real-time TUI shows scanning progress across threads.

### 📉 Right-Sizing Analysis

CloudSlash doesn't just find trash; it finds "Fat" resources.

- **Feature**: `UnderutilizedInstanceHeuristic`
- **Logic**: Detects EC2 instances with **Max CPU < 5%** over the last 7 days.
- **Output**: Marks these as waste and calculates the "Inefficient Spend" based on standard AWS pricing.

### 💰 Real-Time Cost Estimation

- CloudSlash queries the AWS Price List API in real-time.
- It calculates the **exact monthly cost** of every waste item found (e.g., "This 50GB gp2 volume is costing you $5.00/mo").

---

## 3. The Outputs

After a scan completes, CloudSlash generates a `cloudslash-out/` directory with three key artifacts:

### 1. The Executive Dashboard (`dashboard.html`) 📊

A beautiful, self-contained HTML report designed for CTOs and Finance teams.

- **Features/Usage**:
  - Open `cloudslash-out/dashboard.html` in any browser.
  - **Charts**: View "Cost by Resource Type" and "Waste Utilization" visualized.
  - **Table**: A detailed, sortable list of every waste item, risk score, and reason.

### 2. The Safe Remediation Check (`safe_cleanup.sh`) 🛡️

Your safety net. We never want you to lose data.

- **Usage**:
  ```bash
  cd cloudslash-out
  ./safe_cleanup.sh
  ```
- **What it does**:
  - **Snapshot First**: Automatically creates a backup snapshot for every EBS volume and RDS instance _before_ touching it.
  - **Safe Deletion**: Only deletes the resource after the snapshot is confirmed.

### 3. The Terraform Export (`waste.tf`) 🏗️

For Infrastructure-as-Code teams who want to import waste to manage it properly before destruction.

- **Usage**: Run `terraform init` and `terraform apply` to import the discovered waste resources into a state file.

---

## 4. Deep Dive: Heuristics

CloudSlash uses "Forensic Heuristics" to find what other tools miss:

| Heuristic        | What it Hunting         | Logic                                                                               |
| :--------------- | :---------------------- | :---------------------------------------------------------------------------------- |
| **Zombie EBS**   | Unattached Volumes      | Volumes not attached to any EC2. Checks if attached to instances stopped > 30 days. |
| **Right-Sizing** | Over-provisioned EC2    | **CPU Max < 5%** (7-day window). Calculates cost of waste.                          |
| **NAT Vampires** | Unused NAT Gateways     | Traffic < 1GB/mo & Connections < 5. High cost items!                                |
| **S3 Ghosts**    | Stale Multipart Uploads | Incomplete uploads hanging in limbo > 7 days.                                       |
| **RDS Zombies**  | Idle Databases          | DBs with **0 connections** in 7 days or status "stopped".                           |
| **Elastic IP**   | Unattached IPs          | EIPs not attached to any NIC, costing hourly fees.                                  |

---

## 5. Command Reference

| Flag            | Description                                             | Default             |
| :-------------- | :------------------------------------------------------ | :------------------ |
| `-license`      | Pro License Key                                         | `""` (Trial Mode)   |
| `-region`       | AWS Region to scan                                      | `us-east-1`         |
| `-all-profiles` | **Multi-Account**: Scan ALL profiles in `~/.aws/config` | `false`             |
| `-mock`         | Run with simulated data (Demo Mode)                     | `false`             |
| `-tfstate`      | Path to terraform state for drift detection             | `terraform.tfstate` |

---

**Happy Hunting!** 🏹
_For Pro Licenses, visit: [cloudslash-web.pages.dev](https://cloudslash-web.pages.dev/#download)_

---

## 6. Serverless License Server (Admin Only) ☁️

To enable the subscription-based licensing system, you must deploy the Cloudflare Worker.

### Prerequisites

- [Node.js](https://nodejs.org/) & `npm` installed.
- Cloudflare Account (Free Tier).

### Deployment Steps

1.  **Login to Cloudflare**:

    ```bash
    npx wrangler login
    ```

    - (This will open your browser to authorize).

2.  **Create KV Namespace**:

    ```bash
    npx wrangler kv:namespace create "LICENSES"
    ```

    - Copy the output `id` (e.g., `889e09...`) and update `license-server/wrangler.toml` with it.

3.  **Deploy**:

    ```bash
    cd license-server
    npm install
    npx wrangler deploy
    ```

4.  **Connect App**:
    - Copy your worker URL (e.g., `https://cloudslash-license-server.yourname.workers.dev`).
    - Update `internal/license/check.go`:
      ```go
      const LicenseServerURL = "https://your-worker.workers.dev/verify"
      ```
    - Rebuild the CLI: `go build -o cloudslash ./cmd/cloudslash`.

### Managing Subscriptions

- **Issue Key**: Use the Cloudflare Dashboard -> KV, or implement a simple `POST /webhook` script to call your worker.
- **Revoke**: Delete the key from KV in the Cloudflare Dashboard. The CLI will stop working immediately.
