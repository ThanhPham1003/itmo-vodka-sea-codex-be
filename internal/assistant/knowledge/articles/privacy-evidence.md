{"id":"privacy-evidence","version":"2026-10-31.v1","aliases":["CCCD","PII","che dữ liệu","bản gốc","masked","privacy","original"],"pages":["detail"],"sources":["internal/dossier/logic/privacy.go"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Dữ liệu che và bản gốc"},"en":{"title":"Masked data and originals"}}}

## vi

Chi tiết hồ sơ che dữ liệu nhạy cảm mặc định. Xem rõ trường hoặc ảnh gốc cần quyền riêng và ghi nhật ký. Có quyền đọc hồ sơ không tự có quyền mọi PII. Broker không được dùng chatbot vượt che dữ liệu. Không nhập CCCD hay nội dung giấy tờ vào chat. Bot chỉ hướng dẫn, không tra cứu hoặc reveal thay người dùng.

## en

Dossier detail masks sensitive fields by default. Explicit field reveal and original-image access need their respective permissions and are audited. Read permission alone does not expose all PII. Brokers cannot bypass masking via chat. Do not paste identity numbers or document contents into chat. The assistant only explains and cannot retrieve or reveal dossier data.
