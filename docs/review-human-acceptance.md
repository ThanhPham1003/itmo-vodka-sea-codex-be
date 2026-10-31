# Phê duyệt sau khi người kiểm tra chấp nhận các mục

Quy tắc nghiệp vụ được người dùng xác nhận: khi các giấy tờ/kiểm tra đều đạt và người có quyền đã xác nhận chấp nhận từng cảnh báo còn lại, hồ sơ được phép phê duyệt cuối cùng. Không bắt nhập lại dữ liệu trích xuất chỉ để mở nút duyệt.

## Điều kiện cụ thể

- Thông tin bắt buộc hợp lệ được chấp nhận như trước.
- Thông tin bắt buộc thiếu hoặc chưa hợp lệ có thể được chấp nhận qua cảnh báo của **đúng thông tin đó**, sau khi người kiểm tra xác nhận với lý do. Cảnh báo phải có người xác nhận hợp lệ; xác nhận mục khác không thay thế.
- Trước quyết định, BE đối soát lại. Xác nhận chỉ được giữ khi bằng chứng chưa thay đổi; nếu dữ liệu thay đổi, cảnh báo quay lại trạng thái cần kiểm tra.
- Vẫn cần đủ giấy tờ/nguồn, hoàn tất xử lý, không có cảnh báo chưa xử lý hoặc lỗi bắt buộc phải khắc phục. Quyền kiểm duyệt, phân công và kiểm tra phiên bản giữ nguyên.

## Truy vết

Không đổi giá trị hoặc kết quả kiểm tra OCR thành hợp lệ giả tạo. Snapshot quyết định lưu dữ liệu gốc và các cảnh báo đã chấp nhận, gồm `acknowledged_by` và `acknowledgement`. Audit phê duyệt bổ sung danh sách ID `accepted_required_fields` nếu có các mục được chấp nhận theo cách này. Không thêm bảng/cột hoặc sửa quyết định lịch sử.

## FE

Danh sách còn phải kiểm tra và bộ đếm loại các thông tin đã được chấp nhận hợp lệ. Khi mọi điều kiện đạt, hiện thông báo có thể phê duyệt và bật nút. Mục chưa xác nhận vẫn có lối mở tài liệu để kiểm tra/sửa hoặc thao tác xác nhận cảnh báo kèm lý do.

Cần deploy cả BE và FE để đồng bộ quy tắc. Các xác nhận cũ có đủ người/lý do và bằng chứng còn hiệu lực được sử dụng ngay; không cần nhập lại.

## Kiểm chứng

- Go unit: đúng mục, đúng bằng chứng, cảnh báo đã xác nhận, người/lý do bắt buộc; không chấp nhận cảnh báo khác hoặc lỗi bắt buộc phải khắc phục.
- API integration: chưa xác nhận/thiếu một mục còn bị chặn; chấp nhận đủ thì duyệt được; snapshot giữ nguyên dữ liệu và audit có ID mục đã chấp nhận; mở lại và thay dữ liệu yêu cầu xác nhận mới.
- FE unit/browser desktop-mobile: 5 thông tin còn thiếu nhưng cả 5 được xác nhận có thể phê duyệt; không gửi PATCH sửa dữ liệu; trường hợp chưa xác nhận hoặc dữ liệu thay đổi vẫn bị chặn.
