{"id":"processing-retry","version":"2026-10-31.v1","aliases":["OCR","FAILED","thử lại","retry","phân loại","classification","processing","healthy"],"pages":["detail","connection"],"sources":["internal/dossier/logic/processing.go"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Phân loại, OCR và thử lại"},"en":{"title":"Classification, OCR and retries"}}}

## vi

Hệ thống xử lý từng trang, phân loại, trích xuất trường cấu hình và có thể tách nhóm giấy tờ, giữ liên kết trang nguồn. Khi không chắc loại/ranh giới, người có quyền cần xác nhận đúng nhóm, loại và vai trò chủ thể. Tệp FAILED có thể thử lại hoặc thay tệp lỗi theo quyền. Thử lại không bảo đảm chữa ảnh mờ/cấu hình thiếu. Backend healthy không chứng minh worker hay OpenAI đang chạy đúng.

## en

Processing classifies pages, extracts configured fields and may split document groups with source-page links. Uncertain types or boundaries need an authorized user to confirm the group, type and subject role. Failed source files may be retried or replaced where permitted. Retrying cannot guarantee a blurred image or missing configuration is fixed. A backend health check does not prove the worker or OpenAI is functioning.
