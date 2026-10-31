# Rà soát GET và mã định danh hiển thị

Phạm vi: toàn bộ 15 route GET khai báo trong source và các màn hình frontend dùng chúng. Kiểm tra bằng source, test API với SQLite và test trình duyệt dùng response giả lập; không phải kiểm tra trực tiếp bản production đã deploy.

UUID, code, key vẫn được trả về để liên kết dữ liệu, gọi mutation, phân trang và giữ snapshot. Không đổi ID thành tên, không tạo mã hồ sơ giả bằng cách cắt UUID. Tên/nhãn dùng riêng cho hiển thị.

| GET | Kết quả rà soát / xử lý |
| --- | --- |
| `/health` | Không có UUID nghiệp vụ; Request ID dùng chẩn đoán kết nối. |
| `/api/v1/auth/me` | Có `full_name`, `username`, `company.name`; ID chỉ dùng session và quyền. |
| `/api/v1/auth/users` | Có tên thành viên; giữ giới hạn quyền đọc danh sách thành viên. |
| `/api/v1/document-types` | Dùng `definition.name`, `fields[].label`; bỏ dòng code cấu hình và field key khỏi trang. |
| `/api/v1/document-types/:code` | Cùng cấu trúc và quy tắc hiển thị với danh sách. |
| `/api/v1/dossier-types` | Bổ sung `document_schemas` theo đúng `schema_version` trong yêu cầu; frontend dùng tên giấy tờ và nhãn trường khi hiển thị nguồn quy tắc. |
| `/api/v1/dossier-types/:code` | Cùng dữ liệu schema đã pin với danh sách; vẫn đọc được nhãn của loại giấy tờ đã ngừng sử dụng. |
| `/api/v1/dossiers` | Bổ sung `creator_name`, `assignee_name`; bỏ việc lấy UUID làm mã hồ sơ trên bảng và dashboard. |
| `/api/v1/dossiers/review-queue` | Cùng tên người phụ trách với danh sách; bỏ UUID trên thẻ hồ sơ. |
| `/api/v1/dossiers/:id` | Có tên người tạo/phụ trách và `decisions[].actor_name`; nhóm tài liệu, checklist, trường và đối soát dùng snapshot/nhãn sẵn có. Tiêu đề và modal dùng tên hồ sơ. |
| `/api/v1/dossiers/:id/files/:file_id/content` | Trả nội dung tệp và tên tệp; ID nằm trong URL truy cập được kiểm quyền. |
| `/api/v1/dashboard/statuses` | Chỉ dữ liệu tổng hợp, trạng thái và ngày; không hiển thị UUID. |
| `/api/v1/audit-logs` | Bổ sung `resource_name`, `dossier_title`, giữ `actor_name`; không dùng resource UUID hoặc dossier UUID làm nội dung nhật ký. Thiếu liên kết thì hiển thị tên nhóm tài nguyên. |
| `/api/v1/assistant/context` | Bài hướng dẫn có `title`; article ID chỉ dùng liên kết. |
| `/api/v1/assistant/articles/:id` | Hiển thị tiêu đề/nội dung; ID dùng tải bài. |

Các tên bổ sung được resolve khi đọc, không tạo cột database, không sửa audit bất biến. Tra cứu theo công ty và theo lô; thiếu hoặc sai tenant trả tên rỗng, giao diện dùng nhãn thay thế, không hiện ID. Không thay đổi che PII, phân quyền, version hoặc payload ghi.

ID vẫn tồn tại trong URL, DOM `value`/`data-*`, request payload, bộ lọc kỹ thuật nhập ID và Request ID chẩn đoán. Đây không phải tên tài nguyên để trình bày trên trang nghiệp vụ.

Kiểm thử chính: `internal/bootstrap/display_labels_test.go`, `tests/display-labels.test.mjs`, `tests/browser/display-labels.spec.js`; hồi quy catalog, audit và reconciliation. Bao gồm tên schema cũ sau đổi tên/ngừng dùng, tên người thực hiện, tham chiếu chéo tenant, giữ nguyên ID khi lưu và điều hướng.
