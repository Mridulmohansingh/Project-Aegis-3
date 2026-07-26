# Project AEGIS

**National Digital Assessment Platform (NDAP)**

> A secure, scalable, psychometrically rigorous digital examination platform designed for national-scale computer-based testing.

---

## What is this?

Project AEGIS is a complete architecture and implementation for conducting **millions of secure, fair, computer-based examinations**. It is designed with the engineering quality of systems like Aadhaar, UPI, and DigiLocker.

This is **not** a toy exam app. It implements:

- **IRT-based psychometrics** (3-Parameter Logistic model for item calibration, ability estimation, and score equating)
- **Blueprint-constrained paper generation** using Mixed Integer Programming (MIP) — mathematically guarantees equal difficulty, coverage, and fairness across test forms
- **Tamper-evident audit logging** with SHA-256 Merkle chain integrity and Ed25519 signed checkpoints
- **AES-256-GCM envelope encryption** with HSM-abstracted key management
- **Zero Trust security architecture** with ABAC policy enforcement
- **Data-driven item improvement** — every exam administration feeds statistics back to make the question bank smarter

## Current Status

🚀 **Fully Functional Reference Architecture** — All core engines, backend services, database migrations, security policies, deployments config, and frontend components are fully implemented.

| Component | Status |
|---|---|
| Architecture Blueprint | ✅ Complete |
| Domain Models (Item, Blueprint, Paper, Exam) | ✅ Complete |
| Database Migrations (8 migrations with RLS & Partitions) | ✅ Complete |
| IRT Scoring Engine (3PL, MLE, EAP) | ✅ Complete |
| Paper Generation Engine (MIP Solver) | ✅ Complete |
| DIF Detection (Mantel-Haenszel) | ✅ Complete |
| Person-Fit Analysis (Lz statistic) | ✅ Complete |
| Score Equating (IRT True-Score) | ✅ Complete |
| Audit Service (Merkle chain) | ✅ Complete |
| Cryptography Service (AES-256-GCM, Ed25519) | ✅ Complete |
| HTTP Middleware Stack | ✅ Complete |
| Question Bank & Exam Handlers | ✅ Complete |
| PostgreSQL Repository Layer | ✅ Complete |
| Exam Delivery Service | ✅ Complete |
| React Frontend | ✅ Complete |
| Docker Compose (local dev infrastructure) | ✅ Complete |
| Kubernetes Manifests & Terraform Modules | ✅ Complete |
| CSV Exporter Engine | ✅ Complete |
| CI/CD Pipelines | ✅ Complete |

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                    Edge Layer                        │
│            CDN · WAF · DDoS Protection              │
├─────────────────────────────────────────────────────┤
│                   Access Layer                       │
│         API Gateway · Rate Limiter · Auth            │
├─────────────────────────────────────────────────────┤
│                Application Layer                     │
│  Question Bank · Paper Engine · Exam Delivery        │
│  Candidate Mgmt · Scoring · Proctoring              │
├─────────────────────────────────────────────────────┤
│                 Domain Services                      │
│     IRT Engine · Crypto Service · Audit Service      │
├─────────────────────────────────────────────────────┤
│                   Data Layer                         │
│   PostgreSQL · Redis · Kafka · Object Storage        │
├─────────────────────────────────────────────────────┤
│              Security Infrastructure                 │
│          HSM · Vault · SIEM · OPA                    │
└─────────────────────────────────────────────────────┘
```

## Key Technical Highlights

### Paper Generation Engine
Uses **Mixed Integer Programming** to assemble test papers that satisfy all constraints simultaneously:
- Chapter/topic coverage
- Difficulty distribution (IRT-b targets)
- Cognitive level balance (Bloom's taxonomy)
- Time budget
- Exposure control (Sympson-Hetter)
- Enemy item exclusion
- Answer key balance
- Test Information Function optimization

### IRT Scoring Engine
Implements the **3-Parameter Logistic (3PL) model**:

```
P(θ) = c + (1 - c) / (1 + exp(-a(θ - b)))
```

- **MLE** ability estimation via Newton-Raphson iteration
- **EAP** estimation with Gauss-Hermite quadrature
- **IRT True-Score Equating** across parallel forms
- **Mantel-Haenszel DIF detection** with ETS delta classification
- **Person-fit statistics** (standardized log-likelihood Lz)

### Data Feedback Loop
Every exam administration improves the question bank:
1. Responses collected → classical item statistics computed
2. IRT calibration → precise difficulty/discrimination parameters
3. DIF analysis → detect unfair items across demographic groups
4. Person-fit → flag suspicious response patterns
5. Items with poor statistics → flagged for revision or retirement

### Security
- AES-256-GCM encryption with envelope encryption (DEK/KEK hierarchy)
- Ed25519 digital signatures for approval chain and audit integrity
- SHA-256 Merkle chain audit log (append-only, tamper-evident)
- Separation of duties enforced (author ≠ reviewer ≠ psychometrician ≠ approver)
- Hash-partitioned response tables (64 partitions) for scalability

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.22 |
| Database | PostgreSQL (partitioned, RLS) |
| Cache | Redis |
| Messaging | Apache Kafka |
| Search | OpenSearch |
| Crypto | AES-256-GCM, Ed25519, SHA-256 |
| Auth | Keycloak + FIDO2 + OPA |
| Frontend | React + TypeScript + Vite |
| Infrastructure | Docker, Kubernetes, Terraform |
| Observability | Prometheus, Grafana, Loki, Jaeger |

## Project Structure

```
project-aegis/
├── cmd/                        # Service entry points
│   ├── question-bank/          # Question Bank Service
│   └── aegis-simulator/        # Standalone Psychometric simulator
├── internal/
│   ├── api/handler/            # HTTP handlers (REST, Export)
│   ├── audit/                  # Merkle chain audit service
│   ├── cache/                  # Redis cache & Rate-limiting lock service
│   ├── domain/
│   │   ├── item/               # Item aggregate (question bank)
│   │   ├── blueprint/          # Test assembly blueprints
│   │   ├── paper/              # Generated test forms
│   │   └── exam/               # Exams, Sessions, & Responses
│   ├── events/                 # Kafka event publishing module
│   ├── export/                 # CSV exporters for Excel/R/Tableau/Power BI
│   ├── infrastructure/
│   │   └── postgres/           # PostgreSQL pool & repositories
│   ├── irt/                    # IRT scoring engine (Estimation, Equating, DIF, Person-fit)
│   └── papergen/               # Paper generation MIP solver
├── migrations/                 # PostgreSQL migrations (001-008)
├── pkg/                        # Core shared packages (crypto, apperrors, middleware, logging)
├── deployments/                # Docker compose configs & Kubernetes manifests
├── terraform/                  # Infrastructure as Code modules (EKS, RDS, VPC)
├── policies/                   # OPA ABAC policies
├── docs/                       # Technical & onboarding documentation
├── frontend/                   # React + TypeScript + Vite dashboard client
├── Makefile
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

## Getting Started (Local Development)

Please refer to the detailed developer guides inside `/docs/development` for complete environment configuration and local setup instructions.

```bash
# Clone
git clone https://github.com/YOUR_USERNAME/project-aegis.git
cd project-aegis

# Install dependencies
go mod tidy

# Build all services
make build

# Run unit tests
go test ./...

# Run the psychometric pipeline simulation
go run cmd/aegis-simulator/main.go
```

## License

MIT License (see [LICENSE](LICENSE) for details).

## Author

Mridul Mohan Singh
