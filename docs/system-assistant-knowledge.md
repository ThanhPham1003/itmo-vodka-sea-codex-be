# Bảo trì kho hướng dẫn DocSense

Corpus v1 gồm 18 bài VI/EN tại `internal/assistant/knowledge/articles/`, version `2026-10-31.v1`. Mỗi bài ghi source và revision kiểm chứng. Chỉ đưa kiến thức chung đã đối chiếu source; không thêm `.env`, token, log, company/dossier data hoặc tài liệu upload.

| ID | Chủ đề |
| --- | --- |
| getting-started | Luồng cấu hình → tiếp nhận → xử lý → đối soát → người duyệt |
| login-session | Đăng nhập, đăng ký công ty, phiên bộ nhớ, demo |
| roles-access | Vai trò, quyền bổ sung, phạm vi hồ sơ |
| members-admin | Thành viên, quản trị cuối cùng, không hứa gửi email |
| catalog-versions | Mã ổn định, version/snapshot, AI draft cần lưu |
| create-intake | Tạo hồ sơ và tải giấy tờ nhiều đợt |
| upload-limits | PDF/JPEG/PNG, giới hạn phụ thuộc config |
| processing-retry | OCR, nhóm/trang nguồn, xác nhận, retry FAILED |
| checklist-conditions | Đúng loại/vai trò/số lượng, giấy tờ có điều kiện |
| dossier-statuses | Sáu trạng thái; VALID khác APPROVED |
| reconciliation-issues | Đủ nguồn/đúng chủ thể, WARNING khác BLOCKER |
| field-corrections | Sửa có căn cứ/lý do, confidence không xác nhận pháp lý |
| review-readiness | Quyền, phân công và các điều kiện phê duyệt |
| review-decisions | Bổ sung, từ chối, chỉ mở lại APPROVED, version conflict |
| privacy-evidence | Mask/reveal/ảnh gốc, không vượt quyền bằng bot |
| dashboard-metrics | Current, transitions, unique và lọc thời gian |
| audit-history | Lịch sử quyết định và audit theo tenant |
| common-errors | 401/403/409/5xx và kết nối |

## Sửa bài

1. Kiểm tra source đang thực hiện hành vi nào, không chỉ dựa vào thiết kế cũ hoặc demo.
2. Sửa **nội dung Markdown dưới `## vi` / `## en`** và metadata `aliases`, `pages`, `sources`, `revision` khi phù hợp. Giữ ID để citation cũ vẫn có ý nghĩa; không thêm company-specific values.
3. Nếu phát hành corpus mới, cập nhật version đồng bộ metadata của mọi bài và `models.KnowledgeVersion`. Binary sẽ từ chối corpus sai phiên bản, thiếu bản dịch hoặc trùng ID.
4. Thêm câu hỏi VI, không dấu, EN vào retrieval tests và trường hợp nghiệm thu cần thiết. Chạy regression/model evaluation trước khi bật image mới.

Không tự động nạp source tùy ý vào prompt, không cần embedding job hay migration DB. Nội dung demo tại FE là FAQ mẫu nhỏ hơn corpus thật, phải giữ nhãn demo và cập nhật khi quy trình thay đổi.
