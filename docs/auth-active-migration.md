# Sửa startup migration: companies.active contains null values

Log `migrate auth: column "active" of relation "companies" contains null values (SQLSTATE 23502)` xảy ra khi database đã có dữ liệu cũ nhưng migration thêm `active NOT NULL` ngay, không cung cấp giá trị cho các hàng đó. Cũng cần xử lý tương tự cho `users.active`. Đây là lỗi migration trước khi HTTP server chạy; không liên quan API key/model OCR.

## Bản sửa source

`internal/auth/migrations/migrate.go` chạy trong một transaction:

1. Bảng cũ chưa có `active`: thêm cột boolean nullable tạm thời.
2. Giữ mọi giá trị `active=true/false` hiện có, kể cả khác với cột cũ `is_active`.
3. Chỉ với hàng `active IS NULL`, chuyển giá trị từ `is_active` boolean nếu có; nếu không xác định được thì đặt **false**. Không tự kích hoạt công ty/tài khoản chưa biết trạng thái.
4. Backfill cả hàng soft-deleted để constraint không thất bại vì dữ liệu đã ẩn khỏi truy vấn ORM.
5. Áp dụng `NOT NULL`, rồi migrate các cột/index/FK còn lại. Nếu bước sau lỗi, transaction rollback cả backfill. Chạy lại không thay đổi trạng thái đã biết.

Không thêm `default:true` vào model bool: GORM có thể thay giá trị false lúc insert bằng default. Signup/create-member vẫn chủ động đặt trạng thái active theo logic hiện hành; insert inactive được giữ nguyên.

Bản sửa xử lý cột active; không tự đoán/migrate các trường legacy khác như username, role, password hash. Nếu schema cũ còn khác ở các constraint khác, cần xem lỗi tiếp theo và migrate dựa trên dữ liệu thực tế.

## Khôi phục container đang dùng image cũ

Chưa có kết nối SSH tới VM trong phiên này, nên các lệnh dưới chạy trên server của bạn. Bản sửa local chưa tự được publish lên Docker Hub.

1. Dừng vòng restart:

   ```bash
   docker stop sea-codex-api
   ```

2. Dùng đúng database của API (`POSTGRES_DBNAME` trong cấu hình API), không mặc định suy ra từ database mặc định của container PostgreSQL. Chép file `scripts/db/20261031_auth_active.sql` của bản sửa lên VM. Thiết lập tên database:

   ```bash
   APP_DB_NAME='<database-cua-api>'
   ```

3. Backup database đó trước khi chạy migration:

   ```bash
   umask 077
   docker exec -e APP_DB_NAME="$APP_DB_NAME" sea-codex-postgres \
     sh -c 'exec pg_dump -U "${POSTGRES_USER:-postgres}" -d "$APP_DB_NAME" -Fc' \
     > "auth-before-migration-$(date +%Y%m%d-%H%M%S).dump"
   ```

   Đảm bảo lệnh trả 0 và backup có dữ liệu trước bước tiếp theo. Các ví dụ dùng tài khoản init của image postgres; thay `-U` nếu bạn dùng tài khoản quản trị khác. Không cần in hoặc gửi password.

4. Kiểm tra database/schema và tên cột, không truy xuất password hash:

   ```bash
   docker exec -i -e APP_DB_NAME="$APP_DB_NAME" sea-codex-postgres \
     sh -c 'exec psql -U "${POSTGRES_USER:-postgres}" -d "$APP_DB_NAME" -v ON_ERROR_STOP=1' <<'SQL'
   SELECT current_database(), current_schema();
   SELECT table_schema, table_name, column_name, data_type, is_nullable
   FROM information_schema.columns
   WHERE table_name IN ('companies', 'users')
     AND column_name IN ('active', 'is_active')
   ORDER BY table_schema, table_name, column_name;
   SQL
   ```

   Script nhắm `current_schema()`; nếu các bảng nằm ở schema khác, cần đặt search_path đúng trước khi thực thi. `is_active`, nếu có, phải là boolean. Sai kiểu thì script dừng và rollback, không tự diễn giải trạng thái.

5. Backfill và đặt constraint nguyên tử:

   ```bash
   docker exec -i -e APP_DB_NAME="$APP_DB_NAME" sea-codex-postgres \
     sh -c 'exec psql -U "${POSTGRES_USER:-postgres}" -d "$APP_DB_NAME" -v ON_ERROR_STOP=1' \
     < scripts/db/20261031_auth_active.sql
   ```

6. Chạy lại API, xem lỗi thực tế:

   ```bash
   docker start sea-codex-api
   docker logs --tail 50 sea-codex-api
   ```

   Nếu không có lỗi migration nào khác, API có thể khởi động. Công ty/user không có trạng thái cũ sẽ inactive; chỉ kích hoạt các ID đã xác nhận. Không chạy UPDATE active=true cho toàn bộ tenant/users. Ví dụ thao tác có phạm vi rõ ràng, chạy trên đúng DB sau khi thay UUID:

   ```sql
   BEGIN;
   UPDATE companies SET active = true WHERE id = '<company-uuid>'::uuid AND deleted_at IS NULL;
   UPDATE users SET active = true WHERE id = '<user-uuid>'::uuid AND company_id = '<company-uuid>'::uuid AND deleted_at IS NULL;
   COMMIT;
   ```

   Script sửa schema không tự kích hoạt dữ liệu. Không xóa database/volume để chữa lỗi này.

## Deploy bản sửa lâu dài

Đẩy source đã sửa qua CI để tạo image mới; pull/recreate container bằng cấu hình deploy hiện hành. `docker restart` không tải image mới. Để một instance chạy migration có kiểm soát, sau khi thành công có thể đặt `POSTGRES_AUTO_MIGRATE=false` và recreate container. Các thay đổi env cần recreate, không chỉ restart.

## Kiểm thử

Test `internal/auth/migrations/migrate_test.go` kiểm tra bảng có dữ liệu nhưng thiếu active, active nullable, nguồn is_active, trạng thái unknown, soft delete, idempotency, giữ explicit false khi insert và rollback khi tạo unique index thất bại. Mặc định dùng SQLite; để kiểm tra PostgreSQL thật, dùng DB test riêng:

```bash
TEST_POSTGRES_DSN='postgres://test_user@127.0.0.1:5432/test_database?sslmode=disable' \
  go test -count=1 -v ./internal/auth/migrations
```

Mỗi test tạo schema UUID riêng và xóa khi xong; tài khoản cần quyền tạo schema. Không dùng production DSN.

Tham khảo: [GORM migration](https://gorm.io/docs/migration.html), [PostgreSQL 16 ALTER TABLE](https://www.postgresql.org/docs/16/ddl-alter.html).
