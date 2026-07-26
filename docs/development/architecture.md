# Architecture & System Design

This document details the architectural principles and core components of Project AEGIS.

## Core Architectural Principles

1. **Zero Trust Isolation**: Row-level security (RLS) restricts database access based on organization scopes. Separation of duties prevents single-point administrative compromise.
2. **Horizontal Scalability**: Heavy-write tables (e.g., candidate exam responses) are hash-partitioned into 64 partitions to support millions of concurrent submissions.
3. **Immutability & Integrity**: Item history and audit logs are append-only. Auditing uses a SHA-256 Merkle chain with digital signatures to guarantee tamper evidence.
4. **Deterministic Fairness**: MIP optimization ensures parallel test form assembly meets equivalent psychometric blueprints, avoiding selection bias.

---

## Bounded Contexts

The backend is organized into distinct packages representing domain bounded contexts:

### 1. Item Bank Bounded Context (`internal/domain/item/`, `internal/service/`)
Manages the lifecycle of test questions (Items).
* **State Machine**: Draft → Review → Calibration → Pilot → Active → Retired.
* **Separation of Duties**: Author, Reviewer, Psychometrician, and Approver must be distinct users.
* **Encryption**: Answers and solutions are encrypted at rest using AES-256-GCM envelope encryption.

### 2. Paper Assembly Bounded Context (`internal/papergen/`)
Assembles test forms from blueprints.
* **MIP Engine**: Solves a Mixed Integer Programming problem to pick items satisfying target distributions for chapters, difficulty, cognitive levels, time budgets, and option balances.

### 3. Exam Delivery Bounded Context (`internal/delivery/`, `internal/domain/exam/`)
Handles candidate testing sessions.
* **State Management**: Authenticated → In Progress → Completed/Timed Out.
* **Timing**: Enforces server-authoritative timing with NTP-synchronized locks.
* **Capture**: Records response changes and times spent, hashing submissions using per-session HMACs to prevent replay attacks.

### 4. Psychometrics & Scoring Context (`internal/irt/`, `internal/analysis/`)
Scores exams and refines item metadata.
* **IRT 3PL Model**: Employs Newtonian MLE and Gauss-Hermite EAP to estimate latent ability.
* **Equating**: Converts raw scores across forms to a shared scale.
* **DIF**: Checks Mantel-Haenszel indices to detect subgroup bias.
* **Person-Fit**: Uses $L_z$ stats to flag suspicious behavior.
