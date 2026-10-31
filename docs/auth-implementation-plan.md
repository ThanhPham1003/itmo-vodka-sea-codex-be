# Kế hoạch triển khai Auth: Signup, Login, Me

**Trạng thái:** Người dùng đã yêu cầu triển khai kế hoạch này và các flow/API còn thiếu theo thiết kế ngày 31/10/2026. Theo dõi phạm vi mở rộng tại [bảng đối chiếu implementation](implementation-coverage.md).

**Ngày:** 31/10/2026.

**Nguồn:** [Skill create-new-apis](../.agents/skills/SKILL.md), [thiết kế nghiệp vụ](backend-system-design.md), source hiện tại của `auth`, `common`, `pkg/token`, `pkg/utils` và bootstrap.

## 1. Hiện trạng đã kiểm tra

- `internal/auth/server.go` có hai hàm wiring rỗng, chưa có endpoint.
- Chưa có entity/migration/repository cho `companies` hoặc `users`.
- Đã có PASETO v4.public, bcrypt, `AuthMiddleware`, `IdentityResolver`, actor/company context, response helper và sentinel errors.
- Chưa có implementation `IdentityResolver` đọc user/company từ DB.
- OpenAPI chỉ có `/health`. Các mô tả “đã triển khai” ở mục 10 của tài liệu nghiệp vụ là blueprint của source tham khảo.

## 2. Phạm vi đề xuất

Module chính: **`auth`**. Thêm audit writer tối thiểu thuộc module **`audit`** để ghi sự kiện đăng ký/đăng nhập, chưa thêm API đọc audit.

| API | Quyền | Input và kết quả |
| --- | --- | --- |
| `POST /api/v1/auth/signup` | Public | Nhận thông tin công ty và tài khoản; tạo company cùng user `company_admin` trong một transaction; trả 201 với dữ liệu an toàn, không tự đăng nhập |
| `POST /api/v1/auth/login` | Public | Nhận `username_or_email`, `password`; kiểm tra user/company còn hoạt động; trả 200 với `access_token`, `token_type: Bearer`, `expires_at` và user |
| `GET /api/v1/auth/me` | PASETO Bearer | Resolve user/company hiện hành và trả user, company, vai trò, quyền hiệu lực; trả 401 khi không xác thực được |

Đợt này chưa có quản trị thành viên, gán role/quyền qua API, refresh token, logout/revocation, reset password, gửi email hoặc SSO. Token dùng thời hạn đang cấu hình, mặc định 24 giờ. Không tạo tài khoản mẫu/mật khẩu mặc định trong source hoặc image.

## 3. Contract và quy tắc

### Signup

Input có `company` gồm tên, tên pháp lý và thông tin đăng ký/liên hệ theo thiết kế; cùng `username`, `email`, `password`, `full_name` của admin đầu tiên. Tên công ty, username, email, password và full name là bắt buộc; các thông tin pháp lý/liên hệ bổ sung là tùy chọn trong đợt này.

- Server sinh UUID/company ID, đặt role `company_admin` và trạng thái active. Không nhận `company_id`, role hoặc quyền do client tự gán.
- Từ chối các trường điều khiển quyền ngoài contract, không âm thầm áp dụng chúng.
- Username chuẩn hóa trim/lowercase, chỉ nhận chữ ASCII, số, `.`, `_`, `-`, dài 3–64 ký tự; không nhận `@` để tránh nhầm username với email.
- Email trim/lowercase, kiểm tra định dạng. Username/email unique toàn DB đúng blueprint, có unique constraint; tài khoản soft-delete không tự giải phóng định danh.
- Đề xuất password tối thiểu 12 ký tự, tối đa 72 byte theo giới hạn bcrypt; không trim hoặc đổi nội dung password.
- Password hash bằng `utils.HashPassword`; entity dùng `json:"-"`, mọi response dùng DTO riêng.
- Không coi email đã được xác minh; đợt này chưa có quy trình xác minh email.

### Login và Me

- Đăng nhập bằng email hoặc username đã chuẩn hóa, không yêu cầu client chọn tenant.
- Password sai, user không tồn tại, user/company inactive hoặc đã xóa đều trả thông báo 401 thống nhất. Dùng kiểm tra bcrypt với dummy hash khi không tìm thấy user để giảm khác biệt thời gian xử lý.
- Lỗi truy cập DB không được trả thành “sai mật khẩu”; dùng 503 và không lộ chi tiết DB.
- Token chỉ chứa identity; `IdentityResolver` đọc trạng thái, company và quyền từ DB cho mỗi request được bảo vệ.
- Mọi user lookup sau xác thực phải gắn user ID và company scope đáng tin cậy. Header/query/body chứa company ID khác không đổi tenant của actor.
- Quyền hiệu lực là quyền mặc định của role cộng quyền bổ sung. `company_admin` giữ `pii:view`, `dashboard:read`; `platform_admin` không có bypass vào tenant và không được tạo qua signup.
- Trong đợt này chỉ `/auth/me` sử dụng thông tin role/quyền; không mở API hồ sơ hoặc quản trị thành viên chưa triển khai.
- Response theo `{code,message,data}`; response có token/identity dùng `Cache-Control: no-store`. Không log body, password, token hoặc nội dung PII.
- Giới hạn request login/signup tại middleware để trả 429 khi vượt ngưỡng. Limiter có giới hạn số key và dọn key hết hạn; bản đầu là per-process, chưa giả định giới hạn toàn cluster.

## 4. File tạo/sửa

Các file dưới đây là phạm vi dự kiến; chưa viết implementation hoặc OpenAPI trong bước lập kế hoạch.

### Auth: theo đúng luồng layer của skill

```text
internal/auth/dto/auth_request.go
internal/auth/dto/auth_response.go
internal/auth/models/company.go
internal/auth/models/user.go
internal/auth/migrations/migrate.go
internal/auth/repositories/company_repository.go
internal/auth/repositories/user_repository.go
internal/auth/services/auth_service.go
internal/auth/logic/auth_logic.go
internal/auth/logic/identity_resolver.go
internal/auth/logic/permissions.go
internal/auth/presentation/handlers/auth_handler.go
internal/auth/routes/routes.go
internal/auth/routes/v1/routes.go
internal/auth/server.go
```

Models company/user embed `common/models.BaseModel`. User có `company_id NOT NULL`, normalized username/email, hash, tên, role, quyền bổ sung và trạng thái active; FK company phải hợp lệ. Repository dùng `ICompanyRepository`, `IUserRepository`, context đầu tiên và transaction được inject. Service registry có `IAuthService`, `RegisterAuthService()` và `AuthService()`; implementation ở `logic`, handler gọi interface qua registry.

### Audit writer và shared dependencies

```text
internal/common/audit.go
internal/common/app.go
internal/common/middlewares/rate_limit.go
internal/audit/models/audit_log.go
internal/audit/migrations/migrate.go
internal/audit/repositories/audit_repository.go
internal/audit/services/audit_service.go
internal/audit/logic/audit_logic.go
internal/audit/server.go
```

`common.AuditWriter` là contract hẹp được inject qua App; auth không import concrete audit logic/repository. Audit writer nhận context và transaction khi ghi cùng mutation. Audit entity có UUID/CreatedAt riêng, không có soft delete hoặc API update/delete. Hooks ngăn update/delete qua GORM không được coi là cơ chế chống quản trị viên DB can thiệp.

### Wiring, config và tài liệu

```text
internal/bootstrap/app.go
internal/bootstrap/router.go
pkg/config/config.go
pkg/config/load.go
config/app.development.yaml
config/app.docker.yaml
config/docker.env.example
.env.example
docs/api/api_spec.json
README.md
docs/docker-and-ci.md
```

- Wiring audit trước auth; đăng ký auth service và gán resolver thật trước khi mount route được bảo vệ.
- Thêm `POSTGRES_AUTO_MIGRATE`, mặc định false; bật rõ ràng mới chạy GORM AutoMigrate cho audit/company/user theo thứ tự dependency. Khi tắt mà thiếu schema auth, startup báo thiếu migration thay vì phục vụ API không dùng được.
- Thêm cấu hình ngưỡng/window limiter cho auth, không mở trusted proxy toàn bộ để lấy IP.
- Giữ nguyên key pair ổn định, không sinh signing key tự động lúc startup.
- Cập nhật hướng dẫn local/container để khởi tạo schema trước signup; không đưa DB password hoặc signing key vào CI build.
- Chỉ cập nhật OpenAPI sau khi build và test đạt; giữ `/health`, thêm ba API cùng schemas và Bearer security cho `/auth/me`.

## 5. Transaction và xử lý lỗi

Signup hash/validate trước transaction, sau đó tạo company → admin → audit `COMPANY_REGISTERED` bằng cùng `tx`. Bất kỳ bước nào lỗi đều rollback; không để company hoặc admin mồ côi. Unique constraint quyết định khi hai request tạo cùng username/email cạnh tranh; map duplicate thành 409, không chỉ dựa vào check trước insert.

Login chỉ cấp token sau khi kiểm tra credential và ghi audit thành công. Không ghi password/token vào audit. Audit thất bại thì không trả đăng nhập thành công không có sự kiện; trả 503. Failed-login audit không lưu nguyên văn username/email do người dùng gửi và không dùng tenant ID chưa xác minh; phân biệt với log bảo mật chung có request ID.

Me và resolver không tin quyền cũ trong token; user/company inactive khiến request bị từ chối ngay. Không giữ transaction trong lúc thực hiện bcrypt. Không mở transaction mới bên trong audit writer khi caller đã truyền `tx`.

| Tình huống | Sentinel/HTTP |
| --- | --- |
| Input sai hoặc trường bị cấm | `ErrInvalidArgument` / 400 |
| Login sai, token không hợp lệ, account không hoạt động | `ErrUnauthenticated` / 401 |
| Không có quyền | `ErrForbidden` / 403 |
| Trùng username/email | `ErrConflict` / 409 |
| Quá ngưỡng request | 429 |
| DB/audit/token dependency lỗi | `ErrUnavailable` / 503 |
| Lỗi chưa dự kiến | 500, thông báo an toàn |

Wrap nguyên nhân bằng `%w`, kiểm tra bằng `errors.Is`; dùng response helpers hiện có, không đưa SQL/stack/credential vào response.

## 6. Test case cần duyệt

Dùng table-driven `testing` + testify, SQLite in-memory cho repository/model theo skill; `httptest` cho HTTP. PostgreSQL thật được dùng thêm để kiểm tra transaction/constraint của driver thực, không suy ra từ SQLite.

| Nhóm | File test dự kiến | Tình huống |
| --- | --- | --- |
| Repository — happy/edge | `internal/auth/repositories/user_repository_test.go` | Create/find, normalize/unique username-email, hash không serialize, lookup theo company không lộ user tenant khác |
| Repository — transaction/failure | `internal/auth/repositories/company_repository_test.go` | FK, rollback company khi tạo user lỗi; hai request trùng không để dữ liệu dở |
| Logic — signup | `internal/auth/logic/auth_logic_test.go` | Tạo đúng company/admin, hash khác plaintext, audit cùng transaction; validation sai; duplicate; DB/audit lỗi |
| Logic — login | `internal/auth/logic/auth_logic_test.go` | Login bằng username/email; sai password; user không tồn tại; inactive/deleted user/company; token/audit lỗi; cùng thông báo 401 |
| Resolver — unauthorized/tenant | `internal/auth/logic/identity_resolver_test.go` | User/current company từ DB, cập nhật role có hiệu lực với token cũ, thiếu company/inactive bị chặn, DB lỗi là 503 |
| Quyền mặc định | `internal/auth/logic/permissions_test.go` | Role defaults + additional permissions; company admin luôn có PII/dashboard; platform admin không bypass |
| HTTP — happy/validation | `internal/auth/presentation/handlers/auth_handler_test.go` | Signup 201, login 200, me 200; JSON hỏng/missing/wrong type; role/company injection; hash/secret không có trong response |
| HTTP — unauthorized | `internal/auth/presentation/handlers/auth_handler_test.go` | Me không token/token sai/hết hạn; giả company header/body không đổi tenant; 401 và 503 phân biệt đúng |
| Audit — atomicity | `internal/audit/repositories/audit_repository_test.go` | Audit theo company, không chứa secret, cùng tx signup; failure rollback; mutation audit qua GORM bị chặn |
| Limiter — edge | `internal/common/middlewares/rate_limit_test.go` | Vượt ngưỡng 429, hết window được thử lại, key cleanup/capacity, không tin X-Forwarded-For từ proxy không được cấu hình |
| Config — validation | `pkg/config/load_test.go` | AutoMigrate mặc định tắt, env override, limiter window/ngưỡng sai |
| Wiring | `internal/bootstrap/router_test.go` | Health còn hoạt động, ba auth route được mount, Me thực sự được bảo vệ; API nghiệp vụ khác vẫn chưa mount |

Kiểm thử PostgreSQL tạm: bật migration tạo schema; signup → login → me; signup trùng rollback nguyên tử; user/company bị khóa thì token cũ bị từ chối; startup thiếu schema khi không cho migrate báo lỗi rõ ràng.

Lệnh nghiệm thu dự kiến:

```bash
go test -v ./internal/auth/... ./internal/audit/... ./internal/common/...
make check
```

## 7. Trình tự triển khai sau khi duyệt

1. DTO → entity/migration → repository và constraints.
2. Audit writer tối thiểu và transaction contract.
3. Service/registry → auth logic/resolver → handler → route/middleware.
4. Wiring startup, migration opt-in, cấu hình và test.
5. Build/test và smoke PostgreSQL; sửa lỗi phát hiện trong phạm vi auth.
6. Sau khi kiểm tra đạt mới cập nhật OpenAPI và hướng dẫn curl/container.

**Nội dung cần duyệt:** Ba API ở mục 2, phạm vi file ở mục 4, quy tắc/transaction và test case ở mục 3, 5, 6. Skill yêu cầu phê duyệt rõ ràng trước khi viết implementation; bước hiện tại chỉ tạo tài liệu kế hoạch.
