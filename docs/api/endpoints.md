# API Endpoints Directory

This document catalogs the REST API endpoints exposed by the AEGIS platform.

## 1. Item Bank API (`/api/v1/items`)

* **POST `/api/v1/items`**: Create a new question (draft status). Returns item UUID.
* **GET `/api/v1/items/{id}`**: Retrieve detailed item metadata. Decrypts solutions/answers if requested by authorised users.
* **PUT `/api/v1/items/{id}`**: Modify question stem or options (only valid in Draft state).
* **DELETE `/api/v1/items/{id}`**: Soft delete an item.
* **POST `/api/v1/items/{id}/review`**: Submit reviewer approval/rejection.
* **POST `/api/v1/items/{id}/calibrate`**: Set calibrated IRT parameters (a, b, c).
* **POST `/api/v1/items/{id}/activate`**: Promote item from Pilot to Active status.

---

## 2. Exam API (`/api/v1/exams`)

* **POST `/api/v1/exams`**: Register a new exam code and basic structure.
* **GET `/api/v1/exams/{id}`**: Retrieve exam details and section layout configuration.
* **PUT `/api/v1/exams/{id}`**: Update exam details (only in Configured state).
* **POST `/api/v1/exams/{id}/generate-papers`**: Trigger asynchronous Test Paper assembly via MIP solver.
* **POST `/api/v1/exams/{id}/transition`**: Change exam status (Draft → Configured → Active → Completed).

---

## 3. CSV Export API (`/api/v1/export`)

All export endpoints generate standard CSV files with UTF-8 BOM encoding for compatibility with Excel, R, Power BI, and Tableau.

* **GET `/api/v1/export/items`**: Export all questions with IRT parameters.
* **GET `/api/v1/export/exams/{id}/responses`**: Export candidate × item binary response matrix.
* **GET `/api/v1/export/exams/{id}/statistics`**: Export Classical Test Theory (CTT) item difficulty/discrimination statistics.
* **GET `/api/v1/export/exams/{id}/scores`**: Export raw, scaled, and estimated theta scores for candidates.
* **GET `/api/v1/export/exams/{id}/dif`**: Export Mantel-Haenszel Differential Item Functioning analysis results.
* **GET `/api/v1/export/exams/{id}/person-fit`**: Export candidate $L_z$ fit values for fraud detection.
* **GET `/api/v1/export/exams/{id}/summary`**: Export aggregate exam statistics (mean, SD, Cronbach's alpha).
