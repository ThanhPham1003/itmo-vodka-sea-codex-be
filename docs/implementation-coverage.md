# Đối chiếu flow/API với backend-system-design

Ngày rà soát: 31/10/2026. Người dùng yêu cầu kiểm tra và triển khai các flow/API còn thiếu; kế hoạch Auth đã được gửi trước yêu cầu này. Đây là kế hoạch thực thi và bảng theo dõi trong cùng đợt, không phải tuyên bố tính năng đã hoàn tất.

## Hiện trạng trước thay đổi

Chỉ `GET /health` có implementation. `auth`, `audit`, `dossier` mới có package boundaries. Mục 10 trong thiết kế là blueprint tham khảo, không phản ánh implementation của repository này.

## Phạm vi triển khai theo phụ thuộc

| Flow | API/implementation cần bổ sung | Trạng thái ban đầu |
| --- | --- | --- |
| A — Tenant/Auth | Signup, login, me, list/create company users, update access; resolver từ DB; audit; migration | Thiếu |
| H — Catalog | CRUD document types/dossier types, schema/version, code/key tự sinh, validate rule references/types; AI suggestions chỉ nháp | Thiếu |
| B — Tiếp nhận | Create/list/detail dossiers, checklist snapshot, multi-file upload, checksum/dedupe, limits, access scope | Thiếu |
| C — Processing | Worker DB claim/lease/concurrency/recovery; private local storage; PDF pages/groups/derived files; AI extraction/schema/validation | Thiếu |
| D — Sửa/bổ sung | Conditional requirements, retry, manual classification, field corrections, re-reconciliation, evidence | Thiếu |
| E — Review | Assignment, queue, acknowledge warnings, approve/reject/request changes; version/concurrency guards | Thiếu |
| F — PII | Default masking, explicit reveal and audit, protected source file streaming | Thiếu |
| G — Reopen | Require reason, unlock approved dossier, retain decision/audit history and rerun checks | Thiếu |
| I — Dashboard | Status history, date/timezone filters, transition/unique/current metrics, daily series and drill-down | Thiếu |
| Audit | Tenant-scoped audit list, filtering and pagination | Thiếu |

Không bổ sung webhook/export/API keys vì thiết kế ghi rõ ngoài phạm vi. AI thực tế cần provider/model/key do môi trường cung cấp; thiếu cấu hình phải trả trạng thái/lỗi rõ ràng, không tạo dữ liệu giả. Những khả năng UI, đo chất lượng trên tài liệu thật và tích hợp nguồn pháp lý bên ngoài không được coi là đã có chỉ bằng cách tạo API.

## Các nhóm file

Theo skill: DTO → models/migrations → repositories → services/logic → presentation/handlers → routes/v1 → server.go. Interface registry được binding vào handler khi wiring, tránh dùng singleton của app khác trong request/test.

- Auth: các file trong `docs/auth-implementation-plan.md`, mở rộng `member_request.go`, `user_repository.go`, `member_logic.go`, `auth_handler.go` và route cho member management.
- Audit: `internal/audit/{dto/audit_request.go,models/audit_log.go,migrations/migrate.go,repositories/audit_repository.go,services/audit_service.go,logic/audit_logic.go,presentation/handlers/audit_handler.go,routes/routes.go,routes/v1/routes.go,server.go}`.
- Dossier DTO/model: `internal/dossier/dto/{catalog.go,dossier.go,processing.go}`, `internal/dossier/models/{catalog.go,dossier.go,document.go,review.go}`, `internal/dossier/migrations/migrate.go`.
- Persistence/contracts: `internal/dossier/repositories/dossier_repository.go`, `internal/dossier/services/{catalog_service.go,dossier_service.go,processing.go}`.
- Logic: `internal/dossier/logic/{catalog.go,dossier.go,intake.go,processing.go,reconciliation.go,review.go,dashboard.go,privacy.go}`.
- HTTP: `internal/dossier/presentation/handlers/{catalog.go,dossier.go,files.go}`, `internal/dossier/routes/{routes.go,v1/routes.go}`, `internal/dossier/server.go`.
- Shared/infrastructure: `internal/common/{audit.go,validation.go,pagination.go}`, auth/rate limit middleware, `pkg/storage`, `pkg/ai`, config, bootstrap startup/migrations/worker shutdown, Docker Poppler runtime.
- Contract/docs: cập nhật `docs/api/api_spec.json` sau build/test, README và hướng dẫn chạy/thử flow.

## Transaction và sentinel errors

Signup company+admin+audit, version config+snapshot+audit, correction/review/status history+audit dùng cùng transaction. Dossier mutation khóa hàng hoặc kiểm tra revision để không duyệt dữ liệu cũ. Repo luôn scope company/parent; thiếu scope fail closed. Worker dùng claim nguyên tử và lease, lưu token claim để callback cũ không ghi đè lần retry mới. Không giữ khóa DB khi gọi AI hoặc xử lý file. Input/unauthenticated/forbidden/not-found/conflict/unavailable dùng các sentinel errors có sẵn; wrap `%w`.

## Test case trước implementation

| Nhóm | Trường hợp bắt buộc |
| --- | --- |
| Auth/member | Signup/login/me happy path, duplicate/race rollback, bad password/token, inactive account/company, forbidden role escalation, cross-tenant user ID |
| Catalog | Auto codes/keys stable, immutable versions, pinned snapshots, invalid field/operator/reference, cross-company relations |
| Intake | Multi-batch convergence, real file validation, size/count limits, duplicate hash, independent failures, wrong dossier scope |
| Processing | Provider fixtures via httptest, unknown/low-confidence classification, PDF grouping by instance, bounded concurrency, atomic claims/lease recovery, stale callback |
| Rules | Parse dates/numbers/identifiers, missing data/ambiguous subject never false PASS/BLOCKER, same-person pair only, pending document sources |
| Review | Block approve missing/processing/blocker/unacknowledged warning, mandatory reasons, corrections invalidate approval, reopen preserves history |
| Privacy | Mask raw/value/source in default responses, reveal/image permission+audit, no storage path/PII leak, tenant fail closed |
| Dashboard | Dates/timezone, status history rather than updated_at, transitions vs unique/current, tenant and assignment scope |
| Failure | DB/audit rollback, safe HTTP error envelope, provider timeout/429/5xx, unsafe file path, cancelled context |

Table-driven Go/testify tests, SQLite in-memory for repositories and HTTP fixtures; PostgreSQL smoke cho transaction/constraint/claim của driver thực. Chạy `make check`, rồi cập nhật OpenAPI theo endpoint thực tế. Kiểm thử AI dùng fixture có kiểm soát; không suy ra độ chính xác trên hồ sơ thật.
