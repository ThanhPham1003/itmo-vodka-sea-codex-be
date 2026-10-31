# SEA Codex Backend

Backend nền tảng xử lý hồ sơ đa tenant, dùng Go 1.26, Gin, GORM và PostgreSQL. Source được tổ chức theo [skill create-new-apis](.agents/skills/SKILL.md) và [kế hoạch kiến trúc đã duyệt](docs/source-code-architecture.md).

## Hiện trạng

Đã dựng scaffold và endpoint public `GET /health`. Đã có config YAML/env, kết nối PostgreSQL, structured logger, PASETO v4.public, bcrypt, base model UUID, response/error helpers, auth/permission middleware và graceful shutdown.

`auth`, `dossier`, `audit` hiện là khung package và điểm wiring, chưa có API nghiệp vụ hoặc bảng dữ liệu. Chưa có signup/login, upload/OCR, worker, catalog, đối soát, review hoặc dashboard. Auth middleware đã triển khai và kiểm thử bằng resolver giả trong test; resolver đọc user/company từ DB sẽ thuộc module auth. Thiếu resolver thì route được bảo vệ từ chối truy cập.

[Thiết kế nghiệp vụ](docs/backend-system-design.md) chứa blueprint của hệ thống tham khảo, bao gồm mô tả các tính năng từng có ở source khác. Các mô tả đó không thể hiện tính năng đã triển khai trong repository này. [OpenAPI hiện tại](docs/api/api_spec.json) chỉ mô tả endpoint đã được kiểm chứng ở đây.

## Cấu trúc

```text
cmd/
  api/                  HTTP entrypoint
  keygen/               Công cụ tạo key pair chủ động, chạy riêng
config/                 YAML mẫu, không có secret thật
internal/
  bootstrap/            Composition root, router, HTTP lifecycle
  common/               App, actor/tenant context, errors, DTO, base model, middleware
  health/               Luồng service → logic → handler → routes hoàn chỉnh
  auth/                 Company, user, membership và authentication — khung
  dossier/              Catalog, hồ sơ, tài liệu, đối soát, review, dashboard — khung
  audit/                Audit append-only theo tenant — khung
pkg/
  config/ database/ logger/ token/ utils/
docs/
  api/api_spec.json
```

Mỗi module nghiệp vụ có `dto`, `models`, `migrations`, `repositories`, `services`, `logic`, `presentation/handlers`, `routes/v1` và `server.go`. Handler gọi service qua registry; logic điều phối nghiệp vụ/transaction; repository chịu trách nhiệm persistence. Registry chỉ đăng ký ở startup, không được thay khi đang phục vụ request.

## Chạy local

Cần Go 1.26+, PostgreSQL đang chạy và C toolchain để chạy test SQLite. Liveness không query DB mỗi request nhưng startup yêu cầu DB kết nối được. Chưa cần Docker, Redis, Poppler hoặc AI key cho scaffold.

1. Tải dependency:

   ```bash
   go mod download
   ```

2. Chuẩn bị database/user riêng. Ví dụ dưới đây dùng PostgreSQL chạy local; `createuser --pwprompt` yêu cầu bạn đặt password, không ghi password vào command history:

   ```bash
   createuser --pwprompt sea_codex
   createdb --owner=sea_codex sea_codex
   ```

   Các lệnh provisioning cần tài khoản PostgreSQL có quyền tạo user/database. Có thể dùng một database riêng đã được quản trị viên chuẩn bị thay thế.

3. Tạo PASETO key pair một lần và nạp vào shell:

   ```bash
   umask 077
   go run ./cmd/keygen > .env.keys
   set -a
   . ./.env.keys
   set +a
   ```

   `.env.keys` bị gitignore. Công cụ này chỉ xuất key khi bạn chủ động chạy; API không sinh key lúc startup. Dùng lại file này qua các lần restart; không tạo lại key nếu muốn giữ token hợp lệ.

4. Copy config và điền password của database vào file local đã bị gitignore:

   ```bash
   cp config/app.development.yaml config/app.local.yaml
   ```

   Sửa `postgres.password`, cùng host/port/user/dbname nếu cần. Bạn cũng có thể export `POSTGRES_PASSWORD` từ cơ chế secret của môi trường. Key đang được nạp từ shell ở bước 3, nên không cần chép private key vào YAML.

5. Chạy server:

   ```bash
   make run CONFIG=config/app.local.yaml
   ```

6. Kiểm tra:

   ```bash
   curl -i http://127.0.0.1:8080/health
   ```

   ```json
   {"code":200,"message":"Service is running","data":{"status":"ok"}}
   ```

   Response có `X-Request-ID`. `POST /health` trả 405; path chưa triển khai trả 404, cùng JSON envelope. Dùng Ctrl+C hoặc SIGTERM để shutdown và đóng DB pool.

## Cấu hình

Viper đọc file `--config` và biến môi trường với tên viết hoa, dấu chấm đổi thành `_`. `.env.example` chỉ là danh sách tham khảo; API không tự đọc `.env`. Env được ưu tiên hơn YAML, kể cả env rỗng, nên env secret rỗng làm validation thất bại.

| Nhóm | Biến chính |
| --- | --- |
| HTTP | `HTTP_SERVER_HOST`, `HTTP_SERVER_PORT`, `HTTP_SERVER_MODE`, `HTTP_SERVER_LOG_LEVEL` |
| Timeout | `HTTP_SERVER_READ_TIMEOUT`, `HTTP_SERVER_READ_HEADER_TIMEOUT`, `HTTP_SERVER_WRITE_TIMEOUT`, `HTTP_SERVER_IDLE_TIMEOUT`, `HTTP_SERVER_SHUTDOWN_TIMEOUT` |
| PostgreSQL | `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DBNAME`, `POSTGRES_SSLMODE` |
| Pool | `POSTGRES_CONNECT_TIMEOUT`, `POSTGRES_MAX_OPEN_CONNS`, `POSTGRES_MAX_IDLE_CONNS`, `POSTGRES_CONN_MAX_LIFETIME` |
| Auth | `AUTH_PRIVATE_KEY_HEX`, `AUTH_PUBLIC_KEY_HEX`, `AUTH_TOKEN_DURATION` |

Duration dùng định dạng như `5s`, `30m`, `24h`. PASETO private key là 64 byte encoded hex, public key là 32 byte encoded hex; startup kiểm tra cặp key khớp nhau. Token chứa identity và mốc thời gian; company, role và quyền phải được resolve lại từ nguồn tin cậy mỗi request.

YAML mẫu bind `127.0.0.1` và dùng `sslmode: disable` cho DB local. Khi triển khai từ xa, cấu hình host/network và TLS theo hạ tầng; chưa có Docker/CI/deployment trong đợt scaffold. Không có AutoMigrate hoặc schema nghiệp vụ tự chạy lúc startup.

## Kiểm tra và build

```bash
make check
```

Lệnh này kiểm tra gofmt, chạy `go vet ./...`, `go test ./...` và build `bin/api`. Có thể dùng riêng `make fmt`, `make test`, `make vet`, `make build`.

Test unit/HTTP chạy không cần PostgreSQL thật. Test base model dùng SQLite in-memory; không dùng kết quả đó để khẳng định tenant isolation hoặc behavior PostgreSQL. Các nhóm được kiểm tra gồm config/env/secret-safe errors, token/key/expiry/signature, bcrypt, UUID/soft delete, response envelope, auth/permission, request ID/log/recovery, health và route wiring.

Xem [kết quả kiểm chứng scaffold](docs/scaffold-verification.md) để biết các lệnh đã chạy và phạm vi còn chưa triển khai.

## Quy ước mở rộng

- Triển khai API mới theo kế hoạch/test case được duyệt và đúng layer của skill.
- Mọi repository nghiệp vụ phải nhận context và scope `company_id`; middleware permission không thay thế kiểm tra tenant/assignment tại service và DB.
- Quyền mặc định `company_admin` gồm `pii:view` và `dashboard:read`; không có bypass tenant. Các quyền role còn lại sẽ được triển khai ở resolver của module auth.
- Model user phải dùng `json:"-"` cho password hash; không log token, key, body/PII hoặc DSN. Audit không kế thừa soft delete của base model mutable.
- Mutation nhiều entity cùng audit dùng một transaction, không gọi AI/file/network khi giữ khóa DB.
- Chỉ cập nhật OpenAPI sau khi endpoint build/test đạt. Không khai báo API dự kiến như tính năng đã hoạt động.

Tham khảo thư viện: [PASETO Go](https://github.com/aidantwoods/go-paseto), [GORM PostgreSQL](https://gorm.io/docs/connecting_to_the_database.html), [Viper](https://github.com/spf13/viper).
