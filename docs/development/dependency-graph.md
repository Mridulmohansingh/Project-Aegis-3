# Dependency & Package Flow Graph

This document details the package dependency hierarchy. It enforces a strict unidirectional dependency rule: **inner layers must never import outer layers**.

## Package Layout Layers

```
  ┌─────────────────────────────────────────────────────────┐
  │ 1. API LAYER (Handlers, DTOs, HTTP Server Mux)          │
  └───────────────────────────┬─────────────────────────────┘
                              │ imports
                              ▼
  ┌─────────────────────────────────────────────────────────┐
  │ 2. CORE SERVICES (ItemService, DeliveryService)         │
  └───────────────────────────┬─────────────────────────────┘
                              │ imports
                              ▼
  ┌─────────────────────────────────────────────────────────┐
  │ 3. DOMAIN & ENGINES (Item/Exam models, IRT/MIP engines) │
  └───────────────────────────┬─────────────────────────────┘
                              │ imports
                              ▼
  ┌─────────────────────────────────────────────────────────┐
  │ 4. BASE PACKAGES (pkg/crypto, pkg/apperrors)            │
  └─────────────────────────────────────────────────────────┘
```

## Dependency Rules

* **Domain** (`internal/domain/...`) holds purely model interfaces and value objects. It has zero external dependencies (except UUID library and standard math/time tools).
* **Engines** (`internal/irt/`, `internal/papergen/`) contain mathematical rules. They depend only on standard packages and domain models.
* **Services** (`internal/service/`, `internal/delivery/`) orchestrate application rules. They depend on domains, engines, and repositories (via interface).
* **Infrastructure** (`internal/infrastructure/`) implements repository interfaces. It can import `domain` models but must not import `service` business logic.
* **API Handler** (`internal/api/handler/`) acts as the entry boundary. It connects HTTP request payloads to Service methods.
