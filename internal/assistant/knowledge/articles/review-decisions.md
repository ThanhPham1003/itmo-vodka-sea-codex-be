{"id":"review-decisions","version":"2026-10-31.v1","aliases":["bổ sung","từ chối","mở lại","409","reopen","reject","request changes"],"pages":["detail","review"],"sources":["internal/dossier/logic/review.go"],"revision":"1a3ee150f43d1f01f60d16d0484e79b66dd92021","translations":{"vi":{"title":"Bổ sung, từ chối và mở lại"},"en":{"title":"Request changes, reject and reopen"}}}

## vi

Yêu cầu bổ sung cần lý do và đưa hồ sơ về luồng chỉnh/bổ sung. Từ chối cần lý do và không thực hiện khi đang PROCESSING. Người có quyền mở lại hồ sơ đã APPROVED kèm lý do, cần kiểm tra/duyệt lại và giữ quyết định cũ. REJECTED không thuộc luồng mở lại hiện tại. Gặp 409 cần tải lại và kiểm tra phiên bản/trạng thái trước khi thử quyết định, không gửi lại mù quáng.

## en

Request changes requires a reason and returns the dossier for correction/supplementation. Reject requires a reason and is not allowed while PROCESSING. Authorized users reopen APPROVED dossiers with a reason; old decisions remain and review is required again. REJECTED is not supported by the current reopen flow. On 409 reload and inspect the latest state/version before retrying a decision.
