{"id":"common-errors","version":"2026-10-31.v1","aliases":["lỗi","kết nối","401","403","5xx","connection","errors"],"pages":["connection"],"sources":["internal/common/errors.go"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Kết nối và lỗi thường gặp"},"en":{"title":"Connection and common errors"}}}

## vi

401: đăng nhập lại nếu phiên hết hạn. 403: nhờ quản trị kiểm tra quyền. 409: tải lại và kiểm tra dữ liệu/trạng thái thay đổi. 5xx/kết nối: xem Kết nối backend, ghi thời điểm/request ID rồi báo phụ trách. Không xóa volume, chạy migration hay đổi token để vượt lỗi UI. Vận hành VM ngoài phạm vi bot.

## en

401: sign in again if the session expired. 403: ask an administrator to check permissions. 409: reload and inspect changed data/state. Connection/5xx: check Backend connection and report the time and request ID. Do not delete volumes, run migrations or change tokens to bypass UI errors. VM administration is outside assistant scope.
