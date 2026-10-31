{"id":"processing-retry","version":"2026-10-31.v3","aliases":["OCR","FAILED","thử lại","retry","phân loại","classification","processing","healthy"],"pages":["detail","connection"],"sources":["internal/dossier/logic/processing.go"],"revision":"dae1d41e4618300b4912556aecbac4b42e92753a","translations":{"vi":{"title":"Phân loại, OCR và thử lại"},"en":{"title":"Classification, OCR and retries"}}}

## vi

Hệ thống đọc từng trang, nhận diện loại giấy tờ và lấy những thông tin đã được cấu hình. Một tệp chứa nhiều giấy tờ có thể được tách thành các nhóm và vẫn xem được trang gốc. Khi nhận diện chưa chắc chắn, người được phép cần kiểm tra lại loại giấy tờ và đúng người liên quan. Với tài liệu báo Xử lý lỗi, bạn có thể thử lại hoặc thay tệp nếu có quyền. Thử lại không bảo đảm khắc phục được ảnh mờ hay thiếu thiết lập. Màn Kết nối backend báo kết nối thành công chỉ cho biết ứng dụng kết nối được với hệ thống, chưa chứng minh việc đọc giấy tờ đang hoạt động bình thường.

## en

The system reads each page, identifies the document type and collects the configured information. A file containing several documents may be split into groups while retaining links to the original pages. When identification is uncertain, an authorized person should confirm the document type and the correct person it concerns. A file marked Processing failed may be retried or replaced where permitted. Retrying may not fix a blurred image or missing settings. A successful connection check only shows the application can reach the system; it does not prove document reading is working.
