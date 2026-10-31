{"id":"reconciliation-issues","version":"2026-10-31.v1","aliases":["đối soát","cảnh báo","blocker","warning","cross check","reconciliation","acknowledge"],"pages":["detail"],"sources":["internal/dossier/logic/reconciliation.go"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Đối soát, cảnh báo và blocker"},"en":{"title":"Cross-checks, warnings and blockers"}}}

## vi

Mở Đối soát & kiểm duyệt để xem quy tắc, hai nguồn, trạng thái và mức độ. Cần đủ nguồn và ghép đúng chủ thể; thiếu/không chắc không phải khớp. Người có quyền có thể xác nhận WARNING phù hợp với lý do. BLOCKER không được bỏ qua bằng xác nhận, cần giải quyết dữ liệu/tài liệu gốc. Kiểm tra cần nguồn ngoài không đồng nghĩa đã tra cứu cơ sở dữ liệu ngoài.

## en

Open Reconciliation & review to inspect rules, both sources, status and severity. Comparisons need sufficient evidence and an unambiguous subject match; missing or uncertain evidence is not a match. Authorized reviewers can acknowledge eligible WARNINGs with a reason. BLOCKERs cannot be bypassed by acknowledgement; fix the underlying evidence. Checks needing external evidence do not imply an external registry was queried.
