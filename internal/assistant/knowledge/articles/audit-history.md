{"id":"audit-history","version":"2026-10-31.v1","aliases":["nhật ký","lịch sử","audit","history"],"pages":["audit","detail"],"sources":["internal/audit"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Nhật ký và lịch sử quyết định"},"en":{"title":"Audit and decision history"}}}

## vi

Lịch sử quyết định trong hồ sơ ghi người duyệt, thời gian, lý do và phiên bản. Nhật ký hoạt động cần quyền audit trong phạm vi công ty. Không sửa/xóa nhật ký qua API thông thường; không bảo đảm chống sửa tuyệt đối bởi quản trị DB. Không dán log chứa token/PII vào chat.

## en

Dossier decision history records the reviewer, time, reason and version. Activity log requires audit access within the company. Normal APIs do not edit or delete audit events; this is not absolute tamper-proofing against a database administrator. Never paste logs containing tokens or PII into chat.
