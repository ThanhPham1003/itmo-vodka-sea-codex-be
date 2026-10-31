# Kết quả kiểm chứng scaffold

**Ngày:** 31/10/2026.

**Phạm vi:** Nền tảng Go/Gin/GORM theo [kế hoạch đã duyệt](source-code-architecture.md). Đây là kết quả cho scaffold, chưa phải nghiệm thu các tính năng nghiệp vụ trong `backend-system-design.md`.

## Kiểm tra source

Đã chạy thành công `make check`, gồm:

```bash
gofmt -l ./cmd ./internal ./pkg
go vet ./...
go test ./...
go build -o bin/api ./cmd/api
```

Trong môi trường thực thi, dùng `GOCACHE=/private/tmp/sea-codex-go-build` để lưu build cache ngoài source. Cấu hình này không bắt buộc đối với máy của người phát triển.

- gofmt không phát hiện file cần định dạng lại.
- Vet, toàn bộ test hiện có và build đều pass.
- Package nghiệp vụ `auth`, `dossier`, `audit` chưa có hành vi để test; chúng chỉ chứa boundary/wiring.
- Test SQLite xác minh UUID, timestamp và soft delete của base model; không đại diện cho behavior PostgreSQL hoặc tenant isolation của repository nghiệp vụ.
- OpenAPI đã được tạo sau khi test/build đạt; JSON và mọi `$ref` nội bộ được kiểm tra hợp lệ. Spec chỉ có `/health`.

## Kiểm thử chạy thực tế

Dùng PostgreSQL 15 đã cài trên máy, tạo cluster tạm riêng trong `/private/tmp`, user/database test và cổng loopback riêng. API được chạy từ binary vừa build với key pair và password thử nghiệm. Không dùng database hoặc credential sẵn có của người dùng.

| Kiểm tra | Kết quả |
| --- | --- |
| API kết nối được PostgreSQL thật và khởi động | Pass |
| `GET /health` trả 200 và `{code:200,message:"Service is running",data:{status:"ok"}}` | Pass |
| `X-Request-ID` UUID gửi vào được giữ trong response | Pass |
| `POST /health` trả 405 và JSON error envelope | Pass |
| `GET /api/v1/dossiers` chưa triển khai trả 404 | Pass |
| Schema `public` của DB test không có bảng được scaffold tự tạo | Pass |
| Gửi SIGTERM: HTTP shutdown, process thoát code 0 | Pass |
| Cấu hình port PostgreSQL không kết nối được: startup thất bại, không phục vụ HTTP | Pass |
| Thiếu private key: startup thất bại với thông báo cấu hình | Pass |
| Log/error của smoke test không chứa password hoặc private key thử nghiệm | Pass |

Các process API/PostgreSQL tạm đã được dừng, thư mục cluster/log tạm đã được xóa sau kiểm thử. Không để lại API server chạy nền.

## Giới hạn còn lại

- Chưa có signup/login, company/user entity, resolver DB hoặc repository nghiệp vụ. Các test middleware dùng resolver giả để kiểm tra fail-closed và nguồn identity; chưa phải kiểm chứng tenant isolation toàn hệ thống.
- Chưa có catalog/checklist, upload, OCR, worker, rule engine, review, PII reveal, audit persistence hoặc dashboard.
- Chưa có migration nghiệp vụ, Docker, pipeline CI hay triển khai production.
- Timeout khi kết nối bị treo và shutdown khi có request nghiệp vụ dài chưa được mô phỏng end-to-end. Đã kiểm tra context kết nối bị hủy bằng test và lỗi kết nối bị từ chối bằng smoke test.

Hướng dẫn cấu hình key/DB và chạy local nằm trong [README](../README.md). Source sẵn sàng làm nền cho module auth tiếp theo; triển khai API nghiệp vụ theo kế hoạch/test case riêng của skill.
