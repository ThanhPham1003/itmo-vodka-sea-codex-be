{"id":"review-readiness","version":"2026-10-31.v1","aliases":["phê duyệt","duyệt","approve","approval","kiểm duyệt","review"],"pages":["detail","review"],"sources":["internal/dossier/logic/review.go"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Điều kiện kiểm duyệt"},"en":{"title":"Review readiness"}}}

## vi

Vào chi tiết hồ sơ → Đối soát & kiểm duyệt. Phê duyệt cần quyền dossier:review, đúng người phụ trách trừ company_admin, trạng thái VALID_DOCUMENT, trường bắt buộc hợp lệ và không còn WARNING/BLOCKER mở hoặc quy tắc chờ nguồn. VALID_DOCUMENT là Chờ kiểm duyệt, không tự phê duyệt. Cần con người ghi quyết định. Bot không đọc hồ sơ thật, không xác nhận hồ sơ cụ thể đủ điều kiện và không duyệt hộ.

## en

Open dossier details → Reconciliation & review. Approval requires dossier:review, assignment to the caller except for a company administrator, VALID_DOCUMENT status, valid required fields and no open WARNING/BLOCKER or rule pending source documents. VALID_DOCUMENT means awaiting review and does not approve automatically. A human must record the decision. The assistant cannot inspect live dossiers, confirm a particular dossier is ready or approve it.
