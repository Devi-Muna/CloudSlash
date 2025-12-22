# CloudSlash Product Roadmap

> **Strategic Pivot**: Focusing on the "Single Account / Small Team" user profile. $29/mo price point (Anti-Enterprise).

## Phase 1: Container Intelligence (v1.2.x Series)

_Goal: Complete EKS/ECS coverage by v1.2.8_

### v1.2.3 - v1.2.5: The Kubernetes Layer

- [x] **Zombie Control Planes**: Identify EKS clusters ($72/mo base) that have detected **Zero Worker Nodes** for > 7 days.
- [ ] **Ghost Node Groups**: EKS Managed Node Groups that are `ACTIVE` but have **0% Pod Allocation** (just running distinct Daemons).
- [ ] **Abandoned Fargate Profiles**: Profiles active in namespaces that have had no pod launches in 30 days.

### v1.2.6 - v1.2.8: The ECS & Fargate Layer

- [ ] **Empty Services**: ECS Services with `desiredCount > 0` but `runningCount == 0` (failing to launch loop).
- [ ] **Idle Clusters**: ECS Clusters with registered container instances but **0 Tasks** running.
- [ ] **Reverse-Terraform V2**: Full Module Support (Fix "The Caveat"). Parse module paths to generate correct `terraform state rm module.x.type.name` commands.

## Phase 2: "Extended Plans" Coverage (v1.2.9 -> v1.3.0)

_Goal: Complete Database & Network forensics by v1.3.0 Launch_

### v1.2.9: Databases & Caching

- [ ] **Elasticache (Redis/Memcached)**: Clusters with **0 Connections** or `< 2% CPU` over 7 days.
- [ ] **Redshift Warehouses**: Clusters that have not executed a query in 24 hours (Pause them!).
- [ ] **DynamoDB Provisioned Bleed**: Tables set to "Provisioned Mode" with usage consistently below 5% of capacity (Move to On-Demand).

### v1.3.0: The Network "Vampires"

- [ ] **Unallocated EIPs (Enhanced)**: Verify EIPs attached to **Stopped Instances** (still billing hourly) vs truly unattached.
- [ ] **Idle Load Balancers**: ALBs/NLBs with **0 Request Count** for 7 days (often left over after Blue/Green deployments).

## Future: The "Anti-Enterprise" UX (v2.0)

- [ ] **One-Click HTML Report**: A single static HTML file you can email to your boss showing "I saved us $400/mo".
- [ ] **"Just Fix It" Button**: Interactive TUI mode to release EIPs and Snapshot+Delete Volumes immediately.
