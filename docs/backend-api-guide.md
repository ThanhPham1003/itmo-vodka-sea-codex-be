# Hướng dẫn tích hợp backend

Contract đầy đủ: [api_spec.json](api/api_spec.json), import vào Swagger Editor/Postman. Base `/api/v1`, Bearer PASETO v4.public. Mọi response JSON `{code,message,data}`; lỗi có `data:null`. Tenant lấy từ user trong DB, không truyền `company_id` để chọn công ty.

## Đăng ký, đăng nhập và thành viên

```bash
curl -X POST http://localhost:8080/api/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"company":{"name":"Synthetic Demo Company"},"username":"demo-admin","email":"demo-admin@example.test","password":"replace-with-a-long-password","full_name":"Demo Admin"}'

curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username_or_email":"demo-admin","password":"replace-with-a-long-password"}'
```

Signup trả user/company; login là request riêng trả `access_token`, `expires_at`, `user`. Dùng `Authorization: Bearer <access_token>` cho các API còn lại. Chỉ company_admin gọi `GET/POST /auth/users` và `PATCH /auth/users/{id}/access`; không cho tự nâng role hoặc chọn tenant lúc signup. Role/quyền/activation được kiểm tra lại mỗi request, token cũ không giữ quyền đã thu hồi. Quyền bổ sung cộng với quyền role, chưa có deny.

| Role | Quyền mặc định |
| --- | --- |
| broker | Tạo, đọc hồ sơ của mình, upload/bổ sung checklist; không xem PII/ảnh gốc hoặc sửa trường/duyệt |
| legal_specialist | Tạo, đọc toàn công ty, upload, sửa, review/reopen, ảnh/PII và audit; duyệt cần được phân công |
| notary | Đọc hồ sơ được giao, ảnh/PII; không mặc định có review |
| manager | Đọc toàn công ty, phân công, dashboard và audit |
| company_admin | Quản trị thành viên/catalog, xử lý/review, dashboard/PII/ảnh trong công ty |

Assignee là thành viên active có dossier:read, không phải broker. Cấp dossier:review bổ sung khi cần người được giao có quyền duyệt. Không có quản trị xuyên tenant/platform_admin API. Không được vô hiệu hóa/hạ role admin active cuối cùng.

## Cấu hình catalog và snapshot

1. Admin `POST /document-types`, body phẳng:

   ```json
   {"name":"Synthetic identity","description":"Synthetic test document, never real identity","fields":[{"label":"Subject code","type":"text","required":true,"pii":true},{"label":"Birth date","type":"date","required":true,"pii":true}],"special_requirements":[]}
   ```

2. Server trả `code`, `current_version` và `definition.fields[].key`. Tạo các giấy tờ khác tương tự. Không tự đặt key/code mới.
3. `POST /dossier-types` với `name`, `description`, `documents`, `rules` ở top level. Ví dụ `documents:[{"document_type":"<returned-code>","required":true,"min_count":1,"schema_version":1}]`. Rule tham chiếu code/key đã trả về; operator gồm `TEXT_EQUALS`, `DIGITS_EQUAL`, `DATE_EQUAL`, `NUMBER_EQUAL`. So số cần cùng kiểu (number/money/area). Văn bản chuẩn hóa chữ thường/khoảng trắng; chưa bỏ dấu hoặc ánh xạ địa chỉ hành chính.
4. `PUT /document-types/{code}` hoặc `/dossier-types/{code}` gửi toàn bộ definition phẳng và `expected_version=current_version`. Giữ key/code cũ khi sửa, để rỗng khi thêm mới. `active:false`/DELETE ngừng dùng, không phá snapshot hồ sơ cũ.
5. `POST .../suggestions` trả `{draft:true,definition:...}` hoặc rules nháp. Admin rà soát và POST/PUT rõ ràng để lưu. Không tự kích hoạt. Không có API lưu nháp lâu dài; client giữ nháp trả về đến khi admin lưu.

Khi tạo loại bộ hồ sơ mới, có thể gọi `POST /dossier-types/comparison-rules/suggestions` trước khi lưu:

```json
{"description":"Đối chiếu thông tin chủ thể","definition":{"name":"Bộ hồ sơ mới","description":"Mô tả nghiệp vụ","documents":[{"document_type":"<returned-code>","schema_version":1,"role":"bên mua","required":true,"min_count":1}],"rules":[]}}
```

API dùng giấy tờ/vai trò đang chọn và phiên bản schema chỉ định, trả `{draft:true,rules:[...]}` không có `configuration_version`. Key yêu cầu và code quy tắc mới để rỗng. Chỉ admin công ty được gọi; giấy tờ khác công ty hoặc đã ngừng dùng bị từ chối. API không tạo loại hồ sơ hoặc phiên bản; frontend thêm quy tắc vào bản đang soạn, giữ các quy tắc thủ công và chỉ gọi `POST /dossier-types` khi người dùng bấm lưu. API cho loại đã lưu `/dossier-types/{code}/comparison-rules/suggestions` vẫn trả `configuration_version` để kiểm tra phiên bản khi chỉnh sửa.

Một document type dùng nhiều lần cần role hoặc subject_field để ghép đúng đối tượng. Engine chỉ đối soát các cặp một-một có subject tin cậy; mơ hồ/thiếu/không parse được tạo WARNING thay vì chọn trường confidence cao bất kỳ. Nhiều đối tượng ghép đúng được kiểm tra từng cặp.

`rules[].left/right.role` là bộ lọc vai trò của giấy tờ, không phải tên vai trò suy ra từ nhãn trường. Vai trò khác rỗng phải tồn tại trong `documents[]` của đúng `document_type`; rỗng nghĩa là không lọc theo vai trò. Ví dụ giấy tờ chỉ có `role:""` thì quy tắc không được tự đặt `role:"bên mua"` dù trường có tên “Họ tên bên mua”. API gợi ý quy tắc giới hạn code/field/role theo cấu hình và schema đã pin, rồi dùng cùng bộ kiểm tra với PUT trước khi trả bản nháp. Gợi ý không thay đổi yêu cầu giấy tờ và không tự lưu.

Conditional requirement hỗ trợ điều kiện một trường `equals`/`exists` có kiểu, cùng tenant và có tài liệu nguồn trong cấu hình. Dependency vòng bị từ chối. Không phải DSL biểu thức tổng quát; chưa có AND/OR/nested expression. Không có condition thì checklist chờ xác nhận thủ công; thêm/bỏ phát sinh phải có lý do.

## Tạo hồ sơ, upload nhiều đợt và xử lý

Các GET vẫn giữ UUID/code/key cho thao tác API, nhưng giao diện dùng tên riêng: danh sách/chi tiết hồ sơ có `creator_name`, `assignee_name`; quyết định trong chi tiết có `actor_name`; nhật ký có `actor_name`, `resource_name`, `dossier_title`. GET loại bộ hồ sơ có `document_schemas` chứa tên và nhãn trường theo đúng phiên bản giấy tờ đã pin. Đây là dữ liệu chỉ resolve khi đọc, không cần migration. Chi tiết từng endpoint: [Rà soát GET và nhãn hiển thị](get-display-label-audit.md).

1. `POST /dossiers` với `{"title":"Synthetic case","dossier_type":"<code>","priority":0}`. Server pin toàn bộ cấu hình/version và tạo checklist.
2. Upload `POST /dossiers/{id}/files` multipart:

   ```bash
   curl -X POST http://localhost:8080/api/v1/dossiers/ID/files \
     -H "Authorization: Bearer $ACCESS_TOKEN" \
     -F 'files=@/path/to/synthetic.pdf' \
     -F 'document_types=<document-code>'
   ```

   `document_types` là hint tùy chọn; bỏ để phân loại tự động. Với nhiều file, lặp `files` và metadata theo cùng thứ tự; metadata nếu có phải đủ số lượng. `roles`, `replaces_file_ids` cũng là array multipart. Mỗi lần upload có `batch_id`, mỗi file có status/error riêng. Checksum trùng trả DUPLICATE và ID cũ. Upload đợt tiếp theo vào cùng dossier.
3. Poll `GET /dossiers/{id}`: `files`, `document_groups`, `checklist`, `fields`, `issues`, `decisions` và `dossier.version`. Không nhận storage key hoặc đường dẫn nội bộ. Worker phải bật/config AI; PDF cần Poppler.
4. Nhiều tài liệu trong PDF được phân đoạn theo trang/đối tượng, tạo derived files có `source_file_id`/`source_pages`. Field trỏ file gốc và trang gốc; bounding_box dùng tọa độ chuẩn hóa 0..1. Nếu xem derived PDF, đổi trang bằng vị trí trong source_pages. Nhãn phân loại/ranh giới không chắc cần người xác nhận.
5. File FAILED: `POST .../files/{file_id}/retry` kèm `expected_version`. Sửa nhãn: `PATCH .../classification` với type/role và tùy chọn document_group_id. Re-extract bất đồng bộ, tiếp tục dùng pinned schema. Khi chủ động reclassify group, correction của group đó bị hủy để không gán giá trị cũ vào tài liệu mới; correction ở group khác được giữ.

Khi còn file chờ/chạy: PROCESSING. Khi xong: lỗi file/BLOCKER ưu tiên INVALID_DOCUMENT; thiếu giấy tờ là MISSING_DOCUMENT; đủ nguồn và không BLOCKER là VALID_DOCUMENT. WARNING còn mở vẫn ngăn approve. Ảnh trắng/tương phản rất thấp/mất chi tiết nghiêm trọng trả lỗi có trang; heuristic không thay thế đánh giá chất lượng scan thực tế. Tiền xử lý xoay theo EXIF, chưa có deskew góc nghiêng tự do.

## Sửa, duyệt và PII

Mutation gửi `expected_version` từ detail mới nhất. 409 nghĩa version cũ hoặc trạng thái không cho phép; tải lại detail trước khi thao tác. Upload tự tăng version theo transaction từng file nên không gửi expected_version. Công việc worker cũng tăng version khi lưu kết quả.

- Phân công: `PATCH /dossiers/{id}/assignment`, `assignee_id` hoặc null; tùy chọn priority 0..5. Review queue sắp ưu tiên rồi thời gian tạo.
- Sửa: `PATCH .../fields/{field_id}` với value, reason (`OCR_ERROR`, `SOURCE_ERROR`, `ADMIN_UNIT_CHANGE`, `OTHER`), note bắt buộc cho OTHER. Lưu before/after riêng và đối soát lại; API audit không lộ giá trị này.
- WARNING: `POST .../issues/{issue_id}/acknowledge`, reason bắt buộc. Không xác nhận thay cho sửa BLOCKER. Dữ liệu thay đổi làm mất xác nhận cũ nếu evidence đổi.
- Review: `POST .../decisions`, action APPROVE/REJECT/REQUEST_CHANGES. Approve cần đủ nguồn, không pending/open WARNING/BLOCKER. Trường bắt buộc phải hợp lệ **hoặc cảnh báo của chính trường đó đã được người kiểm tra xác nhận chấp nhận kèm lý do**. Xác nhận phải giữ nguyên hiệu lực sau khi đối soát lại dữ liệu hiện tại; xác nhận mục khác không thay thế. Không sửa giá trị/validation OCR để giả lập đạt kiểm tra. Quyết định lưu snapshot người xác nhận/lý do và audit `accepted_required_fields` khi có ngoại lệ được chấp nhận. Reject/request changes bắt buộc reason. Reviewer phải được phân công, ngoại trừ admin công ty.
- Hồ sơ approved/rejected bị khóa. `POST .../reopen` chỉ mở hồ sơ approved, reason bắt buộc; giữ quyết định cũ và phải duyệt lại.
- Detail luôn mask PII, kể cả admin. `POST .../pii/reveal` với field_ids, quyền pii:view + read, audit VIEW_PII; broker bị từ chối.
- `GET .../files/{file_id}/content` stream ảnh/PDF, cần image_view + read, audit VIEW_FILE. Chưa có ảnh làm mờ dành cho broker.

## Dashboard và audit

`GET /dashboard/statuses?from=2026-10-01&to=2026-10-31&timezone=Asia/Ho_Chi_Minh&metric=transitions`. Ngày kết thúc bao gồm cả ngày; timestamp kết thúc là exclusive. Mặc định hôm nay, khoảng tối đa 366 ngày.

`transitions` đếm mọi lần chuyển vào trạng thái; `unique` đếm dossier/status duy nhất trong kỳ và dossier/status/day trong từng ngày; `current` là snapshot hiện tại, trả range_applies=false. Drill-down qua `GET /dossiers` giữ status/from/to/timezone/dossier_type; lọc thời gian dựa vào status history, không ép trạng thái hiện tại vẫn giống quá khứ.

`GET /audit-logs` hỗ trợ action, actor_id, resource_id, dossier_id, from/to RFC3339, limit/offset. Audit không có update/delete API. Các event chứa ID/version/status, không chứa ảnh hoặc giá trị PII gốc. Không có API xem raw decision snapshot.

Mỗi event có `actor_name` để hiển thị người thực hiện: họ tên hiện tại, fallback username nếu họ tên trống. Chỉ resolve user thuộc cùng công ty của event, bao gồm tài khoản inactive/xóa mềm để giữ thông tin truy vết. User không còn bản ghi hoặc tác vụ hệ thống (`actor_id` là UUID zero) có `actor_name` rỗng; UI hiển thị nhãn phù hợp. `actor_id` giữ nguyên để lọc/tham chiếu. Đây là trường đọc tính từ user, không thêm cột DB, không sửa event cũ và không phải snapshot tên tại thời điểm xảy ra sự kiện.
