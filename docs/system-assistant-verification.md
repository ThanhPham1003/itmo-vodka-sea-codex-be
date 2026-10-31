# Kết quả kiểm chứng chatbot — 31/10/2026

Branch BE và FE: `codex/system-assistant`. Đây là kiểm chứng local; chưa push image hoặc deploy VM.

| Kiểm tra | Kết quả |
| --- | --- |
| Go build `./cmd/api` | Pass |
| Go vet toàn repo | Pass |
| Tests assistant/config/AI/bootstrap trên host | Pass |
| OpenAPI generation và route parity | Pass: 41 operations, 33 paths |
| Docker test stage: `go vet ./... && go test ./...` | Pass, gồm PDF/Poppler và contract mới |
| FE syntax check, unit tests, static build | Pass: 58 unit tests |
| Playwright assistant + live API fixture | Pass: 30 checks trên desktop/mobile |
| Playwright assistant + auth ở lượt trước | Pass: 12 checks; sau đó bổ sung các case assistant live |
| Image BE/FE runtime | Build pass: `sea-codex-server:assistant-check`, `sea-codex-frontend:assistant-check` |
| Smoke runtime với PostgreSQL 16 tạm | Pass |
| Compose assistant overlay | `docker compose config --quiet` pass với base service `api` tối thiểu |

Smoke runtime tạo network/container/database tổng hợp riêng và tự dọn sau test. API chạy từ image, không mount source; FE dùng port 80 và proxy tới API trong network Docker. Đã xác minh signup/login, bảo vệ context bằng token, đủ 18 bài embedded, bài EN, từ chối query company giả, POST chat trả 503 khi AI tắt và assets widget có trong FE image.

Các browser test dùng API fixture hoặc demo local, kiểm tra nguồn tham chiếu, disabled library, payload không tự thêm dữ liệu trang, Bearer token, HTML an toàn, đổi ngôn ngữ, logout/401, session stale và layout mobile.

## Giới hạn của kết quả

Không có cuộc gọi tới OpenAI thật trong các test này. Provider giả kiểm tra contract, lỗi, timeout, citation validation và audit; chưa đo chất lượng câu trả lời của model. Trước khi bật production, cần đặt `ASSISTANT_API_KEY`/`ASSISTANT_MODEL` và chạy bộ [Q01–Q36](system-assistant-acceptance-cases.md) bằng dữ liệu tổng hợp.

Lượt chạy toàn suite Go trên host gặp timeout ở test PDF có sẵn `TestPreparePDFAndPageLimit` (15s), và chạy riêng cũng gặp lỗi tương tự với Poppler native của môi trường này. Không sửa hoặc bỏ test PDF. Toàn suite đã pass trong Docker test stage với Poppler Linux của Dockerfile dự án; các module chatbot và contract cũng pass trên host.

Không kiểm chứng trực tiếp tenant production, không đổi DB production, không publish Docker Hub hoặc chạy deployment VM. Image local kiểm chứng trên kiến trúc native Docker Desktop; CI hiện có vẫn chịu trách nhiệm build/publish các nền tảng được cấu hình.
