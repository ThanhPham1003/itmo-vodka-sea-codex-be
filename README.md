# SEA Codex Backend

Backend đa tenant tiếp nhận hồ sơ theo nhiều đợt, phân loại/trích xuất tài liệu theo cấu hình công ty, đối soát và kiểm duyệt pháp lý. Dùng Go 1.26, Gin, GORM, PostgreSQL, PASETO v4.public và bcrypt; tổ chức module theo [skill create-new-apis](.agents/skills/SKILL.md).

Đã triển khai auth/thành viên, catalog có version, hồ sơ/checklist, upload ảnh/PDF, worker AI, đối soát, correction/review/reopen, PII/audit và dashboard. Xem [đối chiếu từng flow và phạm vi kiểm chứng](docs/implementation-coverage.md), [OpenAPI](docs/api/api_spec.json) và [hướng dẫn gọi API](docs/backend-api-guide.md). File [backend-system-design](docs/backend-system-design.md) được giữ nguyên làm yêu cầu đầu vào; các mô tả source tham khảo trong mục 10 không thay thế báo cáo kiểm chứng của repo này.

## Cấu trúc

```text
cmd/api/                HTTP entrypoint
cmd/keygen/             Tạo PASETO key pair chủ động
internal/bootstrap/     Config, DB, migrations, router, worker lifecycle
internal/common/        Actor/tenant, permission, audit contract, DTO, middleware
internal/auth/          Company, user, signup/login, member/access management
internal/dossier/       Catalog, intake, processing, rules, review, privacy, dashboard
internal/audit/         Audit append-only API theo tenant
internal/health/        Liveness
pkg/                    config, database, logger, token, utils, storage, ai, document
scripts/openapi/        Sinh API contract từ DTO và danh sách route
scripts/evaluation/     Synthetic PDF generator và script đánh giá
```

Mỗi module có `dto → models/migrations → repositories → services/logic → presentation/handlers → routes/v1 → server.go`. Service registry được đăng ký khi startup và binding vào handler; transaction nghiệp vụ ghi cùng audit/status history. Repository kiểm tra tenant từ actor đã xác thực. Đây là tenant isolation ở ứng dụng; chưa có PostgreSQL RLS.

## Chạy local

Cần Go 1.26+, PostgreSQL và C toolchain cho SQLite tests. Cài Poppler (`pdfinfo`, `pdftoppm`, `pdfseparate`, `pdfunite`) để xử lý PDF: `brew install poppler` trên macOS hoặc `apt-get install poppler-utils` trên Debian/Ubuntu. Chỉ dùng API auth/catalog không cần AI key; xử lý tài liệu cần provider vision thật.

1. Cài dependency và chuẩn bị database riêng:

   ```bash
   go mod download
   createuser --pwprompt sea_codex
   createdb --owner=sea_codex sea_codex
   ```

   Provisioning cần tài khoản PostgreSQL có quyền tạo user/database; cũng có thể dùng database do quản trị viên chuẩn bị.

2. Sinh key **một lần**, nạp vào shell:

   ```bash
   umask 077
   go run ./cmd/keygen > .env.keys
   set -a
   . ./.env.keys
   set +a
   ```

   Dùng lại key qua restart để token còn hợp lệ. `.env.keys` bị gitignore; không đưa vào image hoặc Git.

3. Copy config, điền database/password vào file local:

   ```bash
   cp config/app.development.yaml config/app.local.yaml
   ```

4. Khởi tạo schema trong database riêng và chạy:

   ```bash
   POSTGRES_AUTO_MIGRATE=true make run CONFIG=config/app.local.yaml
   ```

   Lần đầu tạo 17 bảng nghiệp vụ. Startup mặc định `auto_migrate=false`; các lần sau chạy `make run CONFIG=config/app.local.yaml`. Khi nâng cấp schema, chạy migration có kiểm soát trước các replica phục vụ traffic.

5. Kiểm tra:

   ```bash
   curl -i http://127.0.0.1:8080/health
   ```

   JSON dùng envelope `{code,message,data}`, kèm `X-Request-ID`. Ctrl+C/SIGTERM dừng worker và đóng DB pool.

## Bật AI processing

Trong YAML local hoặc env, đặt provider/model/key thật rồi bật worker. Ví dụ cấu hình biến, thay placeholder bằng cấu hình môi trường của bạn:

```bash
export PROCESSING_ENABLED=true
export PROCESSING_PROVIDER=anthropic
export PROCESSING_BASE_URL=https://api.anthropic.com
export PROCESSING_MODEL='<vision-model-id>'
# Nạp PROCESSING_API_KEY qua secret của môi trường.
make run CONFIG=config/app.local.yaml
```

Adapter `openai-compatible` dùng endpoint `/v1/chat/completions`, JSON output và ảnh base64; endpoint phải hỗ trợ `max_completion_tokens`, `response_format` và `store=false`. Base URL có thể kết thúc bằng `/v1`. Anthropic dùng `/v1/messages`. Không có model/key mặc định và không có kết quả OCR giả khi provider lỗi. Nội dung trang tài liệu được gửi tới provider cấu hình khi xử lý; gợi ý schema/rule chỉ gửi cấu hình.

Worker mặc định **tắt**. Upload vẫn nhận và giữ trạng thái `UPLOADED`/`PROCESSING` khi chờ worker; bật processing và restart để xử lý. Poll mặc định 2s, concurrency 3 file/process, tối đa 10; tổng concurrency tăng theo số replica. Claim/lease nằm trong PostgreSQL, worker cũ bị từ chối khi claim đổi; đây là hàng đợi trong tiến trình API, chưa phải service queue độc lập.

## Cấu hình

API đọc YAML `--config` và env, không tự đọc `.env`. Env ưu tiên hơn YAML, kể cả env rỗng. [`.env.example`](.env.example) và [Docker env mẫu](config/docker.env.example) liệt kê biến.

| Nhóm | Biến chính |
| --- | --- |
| HTTP | `HTTP_SERVER_HOST`, `HTTP_SERVER_PORT`, `HTTP_SERVER_MODE`, `HTTP_SERVER_LOG_LEVEL`, `HTTP_SERVER_*_TIMEOUT` |
| PostgreSQL | `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DBNAME`, `POSTGRES_SSLMODE`, `POSTGRES_AUTO_MIGRATE` |
| Pool | `POSTGRES_CONNECT_TIMEOUT`, `POSTGRES_MAX_OPEN_CONNS`, `POSTGRES_MAX_IDLE_CONNS`, `POSTGRES_CONN_MAX_LIFETIME` |
| Auth | `AUTH_PRIVATE_KEY_HEX`, `AUTH_PUBLIC_KEY_HEX`, `AUTH_TOKEN_DURATION`, `AUTH_REQUEST_LIMIT`, `AUTH_RATE_WINDOW` |
| Files | `UPLOADS_DIRECTORY`, `UPLOADS_MAX_FILE_SIZE_MB`, `UPLOADS_MAX_FILES_PER_REQUEST`, `UPLOADS_MAX_PDF_PAGES` |
| AI/worker | `PROCESSING_ENABLED`, `PROCESSING_PROVIDER`, `PROCESSING_BASE_URL`, `PROCESSING_API_KEY`, `PROCESSING_MODEL`, `PROCESSING_MAX_CONCURRENT_FILES`, `PROCESSING_POLL_INTERVAL`, `PROCESSING_TIMEOUT`, `PROCESSING_LEASE_DURATION`, `PROCESSING_MAX_PAGE_DIMENSION` |

Defaults: 20 MiB/file, 10 files/upload, 100 PDF pages, 1600px cạnh dài; timeout AI 90s, lease 5m. Duration như `5s`, `1m`, `24h`. `identifier` là định danh số giữ số 0 đầu; mã chữ/số dùng `text` kèm regex. PII được cấu hình theo trường; cần đánh dấu đúng trong schema.

Uploads là private local storage, cần persistent volume và backup cùng database. Các replica phải cùng nhìn thấy storage; triển khai nhiều máy cần shared storage hoặc thay adapter. Không có API xóa file gốc/retention tự động. Audit chặn sửa/xóa qua model và API; DB owner vẫn có quyền sửa dữ liệu, chưa có immutable storage ngoài DB.

## Docker và CI

[Dockerfile](Dockerfile) multi-stage, Poppler runtime, UID `10001:10001`. [CI](.github/workflows/cicd.yaml) chạy format/vet/test/build, evaluation tests và container smoke trước khi push Docker Hub `amd64`/`arm64`. [Hướng dẫn đầy đủ](docs/docker-and-ci.md) có volume uploads, migration và secrets.

Secrets build/push: `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN` có quyền push; tùy chọn variable `DOCKERHUB_REPOSITORY`. Push `main`: `latest` và `sha-<commit>`; tag `v*`: release và SHA. Pull request không publish. CI không deploy ứng dụng.

## Kiểm tra

```bash
make check
python3 -m unittest discover -s scripts/evaluation
```

Chạy test Linux có đầy đủ Poppler:

```bash
docker build --target test -t sea-codex-server:flow-tests .
```

Test gồm auth/rollback/tenant, version snapshot, nhiều đợt upload, cảnh báo/BLOCKER, correction, assignment, review/reopen, PII/file audit, dashboard, claim/lease, provider HTTP contract/retry, PDF render/split và storage bounds. AI được mock bằng HTTP fixtures; không dùng test pass để suy ra độ chính xác trên hồ sơ thật. [Synthetic/evaluation](scripts/evaluation/README.md) cung cấp dữ liệu giả và cách chấm prediction thật.

Bootstrap integration tests mặc định dùng SQLite. Có thể chạy cùng bộ test trên PostgreSQL riêng; mỗi app test tạo schema UUID rồi xóa schema đó:

```bash
TEST_POSTGRES_DSN='postgres://test_user@127.0.0.1:5432/test_database?sslmode=disable' \
  go test -count=1 -v ./internal/bootstrap
```

Tài khoản test cần quyền tạo schema trong database test. Không dùng production DSN. Sau build/test pass, cập nhật contract bằng `go run ./scripts/openapi`; test route parity kiểm tra spec khớp router.

Chưa triển khai frontend, ảnh preview làm mờ, bảng địa chỉ hành chính 2025 hoặc UI benchmark (P2). Không có webhook/export/API key cho hệ thống ngoài vì thiết kế loại khỏi phạm vi.
