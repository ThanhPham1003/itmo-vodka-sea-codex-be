# Đối chiếu implementation với backend-system-design

Rà soát ngày 31/10/2026 theo yêu cầu kiểm tra và triển khai flow/API còn thiếu. Trước đợt này repository chỉ có `/health` và khung module. File yêu cầu [backend-system-design.md](backend-system-design.md) được giữ nguyên; mục 10 của file đó mô tả source tham khảo, không phải kết quả kiểm thử của repository này.

## Kết quả triển khai

**38 operations / 30 paths**, gồm `/health` và 37 operations nghiệp vụ dưới `/api/v1`. Contract: [OpenAPI](api/api_spec.json). Cách chạy/gọi API: [README](../README.md), [API guide](backend-api-guide.md).

| Flow | Đã triển khai | Kiểm chứng chính |
| --- | --- | --- |
| A — Tenant/Auth | Signup company/admin nguyên tử, login PASETO, me, list/create members, update role/permissions/active; resolver đọc DB mỗi request; giữ admin cuối cùng; rate limit signup/login | Auth logic tests, rollback audit failure, HTTP end-to-end, PostgreSQL và binary smoke |
| H — Catalog | CRUD/archive/reactivate doc/dossier types, code/key tự sinh, version bất biến, pin schema/rules/checklist; validate type/reference/role/condition cycle/tenant; AI suggestions trả draft chờ admin lưu | Snapshot v1 sau catalog v2, stale expected_version, forbidden admin operations, cross-tenant lookup; pure rule validation/normalization |
| B — Tiếp nhận | Create/list/detail, scope own/assigned/read_all, priority, multi-batch/multi-file upload, checksum/dedupe, MIME/size/count limits, replacement source linkage | Nhiều đợt upload, duplicate, same-company unassigned broker và cross-company deny, private storage tests |
| C — Processing | DB claim/lease/recovery, giới hạn concurrency, worker cancellation; AI Anthropic/OpenAI-compatible; PDF render/segmentation/derived files; schema extraction/evidence/model version; kiểm tra chất lượng ảnh cơ bản | Provider HTTP fixtures/retry, PDF render/split thật trong Linux; test mixed PDF qua worker; claim concurrent/recovery trên SQLite và PostgreSQL |
| D — Bổ sung/sửa | Activate/waive conditional requirement có lý do, retry FAILED, confirm classification/group, sửa trường có mã lý do, giữ correction khi rerun; đối soát lại | Correction làm hết BLOCKER, stale version reject, duplicate mandatory requirement reject |
| E — Review | Assignment, queue VALID theo priority, warning acknowledge, approve/reject/request changes; khóa/version; private decision snapshot và audit/status history cùng transaction | Chặn approve thiếu giấy tờ/BLOCKER/warning/version cũ/chưa phân công; request changes không bị assignment xóa; ack rồi approve |
| F — PII | Default masking, raw excerpt/evidence redaction, explicit field reveal, file streaming cần quyền và audit; no-store | Broker reveal/image bị từ chối; default detail/audit không chứa fixture PII/path; admin reveal/stream có audit |
| G — Reopen | Chỉ mở approved, bắt buộc lý do/version, giữ quyết định trước, kiểm tra lại và yêu cầu duyệt lại | End-to-end approve → khóa upload → reopen → VALID, giữ decision history |
| I — Dashboard | Status history, date/timezone, transitions/unique/current, daily series; drill-down giữ lịch sử thay vì ép current status | Approved transition trong dashboard; historical VALID drill-down vẫn thấy hồ sơ đã approved; broker không có quyền dashboard |
| Audit | Append-only model/API, tenant list, filters action/actor/resource/dossier/time, pagination | Tenant tests, VIEW_PII/VIEW_FILE, per-dossier filter, rollback khi audit lỗi |
| F60–F61 — Đánh giá | Generator PDF có watermark test, nhiễu, lỗi có chủ đích và đáp án; script tính classification/field accuracy, precision/recall, latency/cost từ prediction đã ghi | Generator chạy thành công; unit test bảo đảm prediction thiếu không bị bỏ qua và metric thiếu trả null |

Gợi ý AI được kiểm tra schema/reference trước khi trả về; chưa gọi model thương mại trong đợt này. Nháp trả về cho client, chưa có persistence API riêng cho nháp. Admin xác nhận bằng POST/PUT tạo version. Không gửi hồ sơ thật trong luồng gợi ý cấu hình.

## Kiến trúc và transaction

- Auth, audit, dossier giữ đúng lớp DTO → model/migration → repository → service/logic → handler → route → module wiring.
- Shared actor lấy tenant từ identity DB sau xác thực. Signup/membership không nhận company_id do client chọn. Không có bypass platform_admin xuyên tenant.
- Mutation khóa dossier, kiểm tra expected_version, cập nhật dữ liệu/checklist/issues/status history/audit trong transaction. AI, render và private file write chạy ngoài khóa DB. Resolve assignee chạy trước transaction để không làm cạn pool/chờ kết nối của chính transaction.
- Worker claim PostgreSQL dùng `FOR UPDATE SKIP LOCKED`, claim UUID và lease. Kết quả chỉ commit nếu claim vẫn hiện hành; lease cũ không thể renew sau recovery. Human correction được giữ riêng để OCR không ghi đè ngoài ý muốn.
- Snapshot cấu hình và quyết định giữ dữ liệu lịch sử; original upload được giữ khi tạo derived/replacement. Không có xóa file gốc hoặc retention tự động.
- Tenant isolation hiện ở repository/service, chưa có RLS hay composite foreign key cho toàn bộ quan hệ. Audit bất biến ở model/API, chưa phải cơ chế chống sửa bởi DB owner.

## Quy tắc nghiệp vụ đã áp dụng

1. PROCESSING khi còn source file chờ/chạy; sau đó INVALID nếu file lỗi/BLOCKER/request changes, MISSING nếu còn thiếu nguồn; đủ nguồn và không BLOCKER mới VALID. WARNING chưa xác nhận vẫn chặn approve.
2. Không so chéo tài liệu khi chưa đủ nguồn. Dữ liệu thiếu/sai định dạng/low-confidence/ghép cặp mơ hồ thành WARNING hoặc PENDING, không suy diễn BLOCKER. Ngưỡng tin cậy hiện cố định 0.8.
3. Ghép subject một-một và kiểm tra mọi cặp. Subject trùng/không rõ không tự chọn cặp có confidence cao. Chuẩn hóa định danh số giữ số 0 đầu; ngày strict; số/tiền/diện tích dùng số hữu tỉ, không round float ngầm.
4. Conditional requirement hỗ trợ một điều kiện `equals`/`exists` có kiểu hoặc xác nhận thủ công. Chưa có biểu thức AND/OR/nested DSL. Bỏ yêu cầu chỉ áp dụng conditional và phải có lý do.
5. Special requirement cần nguồn ngoài luôn yêu cầu con người xác minh, không coi prompt/model là bằng chứng đã kiểm chứng bên ngoài.
6. PII được khai báo theo schema. Detail che PII cho mọi role, reveal từng field cần quyền và audit; broker bị chặn ảnh gốc/PII. Không trả storage key, machine values hoặc raw decision snapshots.
7. File lỗi xử lý độc lập; không làm ngừng worker khác. Upload response có kết quả từng file, không đồng nghĩa OCR đã xong. Worker tắt thì file nằm chờ; không sinh OCR giả.

## Các lệnh kiểm chứng

- Build binary API thật bằng `go build -o bin/api ./cmd/api`: pass.
- `go vet ./...` và `go test ./...` trong Docker Linux có Poppler: pass. Bao gồm document preparation/page limit/split, auth, intake/review, rules và các package nền.
- `go test -race -timeout 60s ./internal/bootstrap ./internal/auth/logic ./internal/dossier/logic ./pkg/ai`: pass.
- Bootstrap integration suite trên **PostgreSQL 15 tạm riêng**, tạo schema riêng từng app test và tự dọn: pass. Kiểm tra SQL/driver, transaction, claim/lease và toàn bộ HTTP lifecycle tương ứng.
- Binary API thật với cluster tạm: startup, opt-in migration 17 bảng, health, signup, login, SIGTERM shutdown: pass. Không dùng DB hoặc secrets người dùng.
- Rootless image `linux/amd64`: build, `--help` exit 0, UID 10001, quyền config/uploads và Poppler render/tách/ghép PDF: pass.
- `git diff --check`, gofmt và actionlint: pass.
- Python evaluation unit test và generator 4 synthetic PDFs + ground truth: pass. Chưa có benchmark model thật.
- API parity test: 38 route thực tế khớp spec, operation IDs duy nhất, schema references đầy đủ và mọi route protected trả 401 khi thiếu token.

Poppler được đóng gói sẵn trong môi trường host macOS của agent bị timeout khi render; không coi native PDF test là pass. PDF được kiểm chứng bằng Poppler đầy đủ trong Docker Linux/CI, gồm cả runtime UID không phải root. CI cài `poppler-utils`; local cần cài Poppler hoạt động được. Không bỏ qua lỗi PDF có công cụ hiện diện trong Go tests.

## Phần chưa triển khai hoặc chưa xác minh

| Phần | Giới hạn |
| --- | --- |
| F43/F26/F62 (P2) | Chưa có preview ảnh làm mờ, bảng ánh xạ hành chính 2025 hoặc UI chỉ số đánh giá. Broker bị từ chối xem original thay vì nhận preview. |
| Frontend F31–F33/F38/F45 | Repo cung cấp dữ liệu trường/source/page/box, issues, queue và audit; giao diện xem song song/highlight nằm ở frontend riêng. |
| AI thực tế | Cần model vision/key/base URL và PROCESSING_ENABLED=true; chưa xác minh accuracy, latency, chi phí hoặc chất lượng pháp lý trên hồ sơ thật. Fixture tests chỉ xác nhận xử lý hợp đồng và domain behavior. |
| Chất lượng ảnh | Phát hiện trắng/tương phản rất thấp/mất chi tiết nghiêm trọng bằng heuristic; EXIF rotation/resize/compression có, chưa có deskew góc tự do hoặc mô hình đánh giá scan. |
| Mẫu catalog F13 | Có request mẫu trong API guide và template synthetic để khởi tạo; không seed catalog pháp lý cố định cho mọi tenant. |
| Vận hành | DB polling trong API process; nhiều replica phải dùng chung storage, tổng concurrency nhân theo replica. Chưa có queue service độc lập, distributed global rate limit, object storage, retention scheduler hoặc quy trình versioned SQL migrations production. |

Webhook, export JSON/CSV, client API key và CRM sync được thiết kế ghi rõ ngoài phạm vi; không bổ sung các API đó. Chưa push image/deploy hoặc chạy workflow từ máy local.
