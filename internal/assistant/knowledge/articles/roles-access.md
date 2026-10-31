{"id":"roles-access","version":"2026-10-31.v1","aliases":["vai trò","phân quyền","broker","manager","legal","notary","permissions","access"],"pages":["members"],"sources":["internal/common/permissions.go"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Vai trò và quyền truy cập"},"en":{"title":"Roles and access"}}}

## vi

Broker mặc định tạo/đọc/tải giấy tờ trong phạm vi được phép; chuyên viên pháp lý kiểm tra, sửa và kiểm duyệt; notary đọc hồ sơ trong phạm vi, xem ảnh/PII theo quyền; manager theo dõi và phân công; company_admin quản trị thành viên, cấu hình và nghiệp vụ trong công ty. Quyền bổ sung có thể khác mặc định. Nút thao tác còn phụ thuộc trạng thái và phân công. Bot không cấp quyền.

## en

Default roles: brokers create/read/upload accessible dossiers; legal specialists inspect, correct and review; notaries read accessible dossiers with image/PII permissions; managers monitor and assign; company administrators manage company members, configuration and workflows. Additional permissions may change defaults. Buttons also depend on state and assignment. The assistant cannot grant permissions.
