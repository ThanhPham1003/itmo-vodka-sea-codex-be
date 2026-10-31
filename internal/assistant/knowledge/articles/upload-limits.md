{"id":"upload-limits","version":"2026-10-31.v1","aliases":["upload","tải lên","Excel","PDF","JPEG","PNG","giới hạn","limits","size"],"pages":["detail"],"sources":["pkg/config/load.go"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Định dạng và giới hạn tệp"},"en":{"title":"Formats and upload limits"}}}

## vi

Nhận PDF, JPEG và PNG. Giới hạn theo cấu hình máy chủ; mặc định hiện tại 20 MiB/tệp, 10 tệp/request, 100 trang/PDF, production có thể khác. Xem kết quả từng tệp. Đổi tên chưa chắc vượt kiểm tra trùng. Không xóa volume/database để chữa lỗi upload.

## en

Supported formats are PDF, JPEG and PNG. Limits depend on server configuration; current defaults are 20 MiB per file, 10 files per request and 100 pages per PDF. Production may differ. Inspect each file result. Renaming may not bypass duplicate detection. Never delete database volumes to fix an upload error.
