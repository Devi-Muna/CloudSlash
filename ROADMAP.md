# CloudSlash Roadmap

> **Philosophy**: We are the "Swiss Army Knife" for cloud cost cleanup. We build for the solo developer, the consultant, and the agile team lead—not for the Enterprise Procurement Department. We prioritize speed, safety, and local-first execution.

---

## 📅 Upcoming Releases

### v1.4: Container Intelligence (The "Black Hole" Update)

_Targets: EKS, ECS, Fargate_
Kubernetes is often the biggest source of hidden waste. This update focuses on identifying "Zombie Clusters".

- [ ] **Empty EKS Control Planes**: Detect clusters costing ~$72/mo that have 0 worker nodes attached.
- [ ] **Ghost Node Groups**: Identify Auto Scaling Groups for EKS that have launched instances but have 0% Pod allocation.
- [ ] **Abandoned ECS Clusters**: Clusters with running services but 0 tasks or traffic.

### v1.5: Serverless & Database Forensics

_Targets: Lambda, DynamoDB, RDS_

- [ ] **Lambda Crypts**: Functions with 0 invocations over 90 days (Code rot).
- [ ] **DynamoDB Over-Provisioning**: Tables using "Provisioned" capacity mode but consuming < 5% of RCUs/WCUs.
- [ ] **RDS Right-Sizing**: identifying "xlarge" instances running at 2% CPU use.

---

## 🔮 Future Concepts (Under Consideration)

- **"Quick-Audit" PDF**: One-click generation of a branded PDF report. Perfect for consultants to send to clients.
- **Cost Anomalies**: "Did this bill just spike 20% yesterday?" (Simple anomaly detection).
- **Multi-Cloud**: Azure/GCP adapters (Long-term goal).

---

## ✅ Completed Milestones

### v1.3: Cost Intelligence (Released)

- [x] Real-Time AWS Pricing integration.
- [x] Daily Burn Rate & Annual Projection.

### v1.2: Core & Forensics (Released)

- [x] **Robust Installer**: Progress bars, auto-arch detection.
- [x] **Owner Identification**: Integrating CloudTrail for "The Blame Game".
- [x] **Reverse-Terraform**: `flux_terraform.sh` generation.
- [x] **Fossil Snapshots**: Graph-based snapshot cleaner.
