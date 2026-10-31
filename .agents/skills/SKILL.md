---
name: create-new-apis
description: Use this skill when implementing new APIs or endpoints in sea-codex-server. Enforces strict context scoping, upfront plan & testcase approval, precise Clean Layered Architecture paths (dto -> models -> repositories -> services & logic -> presentation/handlers -> routes -> server.go/cmd), testify/Go testing, and automatic API documentation updates after verification.
---

# Create New APIs Workflow

Use this skill when adding or updating APIs in `sea-codex-server`. Directory paths are organized around the variable `xxxx` (representing the specific domain service or module name, e.g., `auth`, `problem`, `submission`, `contest`).

---

## 1. Directory Structure Conventions

`sea-codex-server` follows a Clean Layered Modular Architecture. Each layer and artifact must reside in its designated path:

- **DTO Layer:** `internal/xxxx/dto/` (Request & Response structs, Gin binding tags, e.g., `binding:"required"`)
- **Model & Migration Layer:** `internal/xxxx/models/` (GORM entities embedding `BaseModel` with UUID primary key) and `internal/xxxx/migrations/` (GORM `AutoMigrate`)
- **Repository Layer:** `internal/xxxx/repositories/` (Data access layer using GORM, interfaces `I<Entity>Repository` and concrete structs)
- **Service Layer (Interfaces & Registry):** `internal/xxxx/services/` (Defines `I<Entity>Service`, `Register<Entity>Service()`, and singleton accessor `<Entity>Service()`)
- **Logic Layer (Business Logic):** `internal/xxxx/logic/` (Implements `services.I<Entity>Service`, business rules, validation, domain processing)
- **Handler Layer (Presentation / HTTP):** `internal/xxxx/presentation/handlers/` (Gin HTTP handlers parsing inputs, calling service registry, responding via standard JSON helpers)
- **Routes Layer:** `internal/xxxx/routes/` and `internal/xxxx/routes/v1/routes.go` (Gin route group definitions, applying middlewares)
- **Module Server & Wiring:** `internal/xxxx/server.go` (`RegisterServices(ctx *common.App)`, `NewServer(ctx *common.App)`)
- **Entrypoint:** `cmd/api/main.go` (Loads config, initializes DB, logger, token manager, runs server)
- **Shared App Context & Middlewares:** `internal/common/` (`common.App`, `middlewares.AuthMiddleware`, `middlewares.RequestIDMiddleware`, `middlewares.LoggerMiddleware`)
- **Core Packages (`pkg/`):**
  - `pkg/token`: PASETO v4 Asymmetric Token Manager (`ITokenManager`, `TokenClaims`)
  - `pkg/database`: PostgreSQL connection via GORM (`InitPostgresDB`)
  - `pkg/logger`: Structured logging with Zerolog (`NewLogger`)
  - `pkg/config`: YAML + Viper configuration loader (`LoadConfig`)
  - `pkg/utils`: Bcrypt password helpers (`HashPassword`, `CheckPasswordHash`)
- **API Documentation File:** `docs/api/api_spec.json`

---

## 2. Core Constraints

- **Strict Context Scoping (Zero Unnecessary Scanning):**
  - Rely exclusively on files, structs, DTOs, and specs provided directly in the prompt or relevant module.
  - DO NOT scan unrelated modules or out-of-scope files to avoid burning time and context tokens.
  - If a shared dependency is missing (e.g., standard response wrapper, middleware, DB client instance), inspect `internal/common` or ask concisely.
- **Upfront Plan & Test Case Approval:**
  - Always outline the execution plan and unit test cases before touching any code.
  - **STOP and wait for explicit user approval** before writing any implementation code.
- **Consistent Response Format:**
  - Always use `dto.SuccessResponse(c, statusCode, message, data)` for successful responses.
  - Always use `dto.ErrorResponse(c, statusCode, message)` for error responses.
- **Authentication & Security:**
  - Protected endpoints MUST be secured with `middlewares.AuthMiddleware(app)` which validates PASETO v4 Bearer tokens and injects `user_id`, `user_email`, `user_role`, `is_admin`, and claims into `gin.Context`.
  - Passwords must be hashed with `pkg/utils.HashPassword` (bcrypt) and never exposed in JSON responses (`json:"-"`).

---

## 3. Step-by-Step Workflow

### Step 1: Implementation Plan & Test Cases (Wait for Approval)

Before writing any code, output a concise plan covering:

1. **Target Module (`xxxx`):** Confirm the active module/service name (e.g., `problem`, `submission`).
2. **File List & Layer Flow:** List the exact files to create or modify across all layers:
   - `internal/xxxx/dto/...`
   - `internal/xxxx/models/...` and `internal/xxxx/migrations/...`
   - `internal/xxxx/repositories/...`
   - `internal/xxxx/services/...`
   - `internal/xxxx/logic/...`
   - `internal/xxxx/presentation/handlers/...`
   - `internal/xxxx/routes/v1/...`
   - `internal/xxxx/server.go`
   - `docs/api/api_spec.json`
3. **Error Handling & Transaction Strategy:** Detail sentinel errors and transaction orchestration if mutations span multiple entities.
4. **Unit Test Plan:**
   - Categorize cases: _Happy Path_, _Edge Cases_, _Validation Errors_, _Unauthorized / Forbidden_, _DB Failures_.
   - Identify which layer tests each specific scenario.

> **STOP:** Await user confirmation ("OK", "Approved") before moving to Step 2.

---

### Step 2: Layer-by-Layer Implementation

Implement sequentially from bottom to top, adhering to idiomatic Go 1.26 practices:

#### 1. DTO Layer (`internal/xxxx/dto/`)
- Define request and response structs with Gin tags (`binding:"required,min=..."`, `json:"..."`).

#### 2. Models & Migrations (`internal/xxxx/models/` & `internal/xxxx/migrations/`)
- Embed `models.BaseModel` for UUID primary keys, `CreatedAt`, `UpdatedAt`, and `DeletedAt`.
- Register new models in `internal/xxxx/migrations/migrate.go` via `db.AutoMigrate(...)`.

#### 3. Repository Layer (`internal/xxxx/repositories/`)
- Accept `ctx context.Context` as the first argument in all methods.
- Define `I<Entity>Repository` interface and GORM implementation.
- Return sentinel errors or wrap errors using `%w` (`fmt.Errorf("find record: %w", err)`).

#### 4. Service Layer (`internal/xxxx/services/`) & Logic Layer (`internal/xxxx/logic/`)
- **Service Interface & Registry:** Declare `I<Entity>Service` in `internal/xxxx/services/` with `Register<Entity>Service()` and `<Entity>Service()`.
- **Business Logic:** Implement in `internal/xxxx/logic/`. Enforce domain rules, validation, and DTO-to-entity mappings.
- Write unit tests for logic (`internal/xxxx/logic/*_test.go`).

#### 5. Handler Layer (`internal/xxxx/presentation/handlers/`)
- Parse and validate incoming transport data (`c.ShouldBindJSON`, `c.Param`, `c.Query`).
- Call service via `services.<Entity>Service()`.
- Use `dto.SuccessResponse` and `dto.ErrorResponse` to return standardized JSON.
- Write integration tests using `httptest` and Gin test mode (`internal/xxxx/presentation/handlers/*_test.go`).

#### 6. Routing & Wiring (`internal/xxxx/routes/` & `internal/xxxx/server.go`)
- Register endpoints in `internal/xxxx/routes/v1/routes.go`.
- Apply `middlewares.AuthMiddleware(app)` to protected routes.
- Wire service in `RegisterServices(ctx *common.App)` within `internal/xxxx/server.go`.

---

### Step 3: Verification & API Documentation Update

Once the code implementation across all layers is complete:

1. **Run Verifications:**
   - Ensure the project builds cleanly without compile errors:
     ```bash
     go build -o bin/api ./cmd/api
     ```
   - Run unit and integration tests and confirm 100% pass:
     ```bash
     go test -v ./internal/xxxx/...
     ```
2. **Update API Spec:**
   - **ONLY after all tests and builds have passed**, update or create the API specification in:
     `docs/api/api_spec.json`
   - Append the new endpoint definition (HTTP method, path, request headers, parameters, request body schemas, and response status codes/schemas).
   - Ensure the JSON format remains valid, clean, and properly formatted.

---

## 4. Testing Standards

1. **Table-Driven Test Pattern:**
   - Structure unit tests using table-driven definitions (`tests := []struct{ ... }`).
   - Test both happy paths and failure branches (validation error, conflict, not found, unauthorized).

2. **In-Memory SQLite for DB & Repositories:**
   - Use SQLite in-memory (`sqlite.Open(":memory:")`) with `migrations.MigrateXxxModels(db)` to test GORM repositories and models without external DB dependencies.

3. **HTTP Integration Tests:**
   - Use `httptest.NewRecorder()` and `router.ServeHTTP(w, req)` to test full request/response cycle including middlewares and JSON serialization.

---

## 5. Completion Checklist

- [ ] Directory layout matches: `dto` $\rightarrow$ `models` $\rightarrow$ `repositories` $\rightarrow$ `services` / `logic` $\rightarrow$ `presentation/handlers` $\rightarrow$ `routes` $\rightarrow$ `server.go`.
- [ ] GORM models embed `BaseModel` and UUID primary key is generated properly.
- [ ] `ctx context.Context` is passed across all functions and layers.
- [ ] Errors are wrapped with actionable context via `%w` (no swallowed errors).
- [ ] Standard JSON response format (`dto.SuccessResponse`, `dto.ErrorResponse`) is used.
- [ ] Protected endpoints use `middlewares.AuthMiddleware(app)` with PASETO v4 tokens.
- [ ] All unit and integration tests pass cleanly (`go test -v ./internal/xxxx/...`).
- [ ] Binary compiles successfully (`go build -o bin/api ./cmd/api`).
- [ ] `docs/api/api_spec.json` is updated with valid JSON for the new endpoints.
