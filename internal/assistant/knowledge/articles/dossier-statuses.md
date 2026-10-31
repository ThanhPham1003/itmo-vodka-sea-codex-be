{"id":"dossier-statuses","version":"2026-10-31.v1","aliases":["trạng thái","chờ kiểm duyệt","status","statuses","VALID_DOCUMENT","APPROVED_DOCUMENT"],"pages":["detail","review","dossiers"],"sources":["internal/dossier/models/dossier.go"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Ý nghĩa trạng thái hồ sơ"},"en":{"title":"Dossier statuses"}}}

## vi

PROCESSING: còn tài liệu chờ xử lý. MISSING_DOCUMENT: chưa thỏa giấy tờ/điều kiện. INVALID_DOCUMENT: lỗi, blocker hoặc yêu cầu chỉnh/bổ sung. VALID_DOCUMENT: Chờ kiểm duyệt, chưa phải Đã phê duyệt. APPROVED_DOCUMENT: đã ghi phê duyệt. REJECTED: đã ghi từ chối. Không phải chuỗi một chiều bắt buộc. Hồ sơ chưa kết thúc ưu tiên xử lý còn chờ, rồi lỗi/blocker, thiếu giấy tờ, cuối cùng sẵn sàng duyệt. Xem checklist và vấn đề, không đoán từ số file.

## en

PROCESSING: work is pending. MISSING_DOCUMENT: requirements/conditions are not satisfied. INVALID_DOCUMENT: failures, blockers or a request for changes. VALID_DOCUMENT: awaiting human review, not approved. APPROVED_DOCUMENT: approval recorded. REJECTED: rejection recorded. This is not a mandatory one-way sequence. Editable dossiers prioritize pending processing, errors/blockers, missing requirements, then readiness. Inspect the checklist and issues, not just file counts.
