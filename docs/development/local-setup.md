# Local Setup & Developer Onboarding

Welcome to the AEGIS developer onboarding guide. This document helps you set up a local development environment for the National Digital Assessment Platform (NDAP).

## Prerequisites

Ensure you have the following installed on your machine:
* **Go 1.22+**
* **Docker & Docker Compose** (for running database, cache, message broker, and auth servers)
* **Node.js v20+ & npm** (for running the React frontend)
* **PostgreSQL Client** (psql, optional for direct DB queries)

---

## 🚀 Step 1: Clone and Start Infrastructure

Start all required third-party services in the background using Docker Compose:

```bash
# Start DB, Redis, Kafka, OPA, Keycloak, Prometheus, Grafana, OpenSearch
docker compose -f deployments/docker/docker-compose.yml up -d
```

Verify that all containers are healthy:
```bash
docker compose -f deployments/docker/docker-compose.yml ps
```

---

## 🏛️ Step 2: Database Setup & Migrations

Run database migrations to initialize schemas, tables, partition rules, and triggers:

```bash
# Apply SQL migrations up to date
make migrate-up
```

This creates:
1. Core schemas (orgs, users)
2. Item bank schema (taxonomy, items, versions)
3. Exam delivery schema (sessions, partitioned responses, scoring, results)
4. Merkle-chained audit log

---

## ⚙️ Step 3: Run the Backend

Configure settings using the default configuration file and start the Question Bank API:

```bash
# Build the binary
make build

# Run the Question Bank Service
./bin/question-bank --config=config.dev.yaml
```

The service will start on `http://localhost:8080`.
* Health check: `http://localhost:8080/health`
* Prometheus scraping metrics: `http://localhost:8080/metrics`

---

## 🖥️ Step 4: Run the React Frontend

Install dependencies and start the Vite dev server:

```bash
cd frontend
npm install
npm run dev
```

The Vite dev server will start on `http://localhost:3000`. It is configured to automatically proxy `/api/*` requests to the Go backend on `http://localhost:8080`.

---

## 🧪 Step 5: Running Unit Tests & Simulations

To run Go unit tests:
```bash
go test ./... -v
```

To execute the full psychometric pipeline simulator (which does not require a running database and runs entirely in memory):
```bash
go run cmd/aegis-simulator/main.go
```
This generates analysis CSVs in the `./simulation_exports` directory.
