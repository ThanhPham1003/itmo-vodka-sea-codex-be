# Docker và push image lên Docker Hub

## Các file

- [`Dockerfile`](../Dockerfile): multi-stage build với Go 1.26; runtime Debian slim, CA certificates, timezone và user `10001:10001`.
- [`.dockerignore`](../.dockerignore): chỉ gửi input cần build, loại local config, secret, Git history, uploads và binary trên máy.
- [`config/app.docker.yaml`](../config/app.docker.yaml): bind HTTP `0.0.0.0:8080`, nhận secret qua env; PostgreSQL mặc định dùng TLS.
- [`config/docker.env.example`](../config/docker.env.example): cấu hình mẫu để kết nối DB local từ Docker Desktop.
- [`.github/workflows/cicd.yaml`](../.github/workflows/cicd.yaml): kiểm tra source/container trước khi push image đa kiến trúc.

Image chứa `/app/api`, `/app/keygen` và YAML mẫu không có secrets. PASETO key và password được đưa vào lúc chạy, không dùng build args. Runtime có Poppler để render và tách PDF. Lưu file tại `/app/storage/uploads`.

## Build và chạy local

Docker daemon cần đang hoạt động:

```bash
docker build --pull -t sea-codex-server:local .
```

Nếu chưa có key pair từ hướng dẫn chạy local, tạo **một lần**:

```bash
umask 077
docker run --rm --entrypoint /app/keygen sea-codex-server:local > .env.keys
```

Dùng lại `.env.keys` qua các lần chạy. Copy env mẫu và điền password/cấu hình DB phù hợp:

```bash
cp config/docker.env.example .env.docker
```

`POSTGRES_HOST=host.docker.internal` dùng cho PostgreSQL chạy trên máy có Docker Desktop. Trên Linux hoặc server, đặt host database có thể truy cập từ container; nếu kết nối host Linux qua tên này thì thêm `--add-host=host.docker.internal:host-gateway` vào lệnh chạy. Database phải được chuẩn bị trước và cho phép kết nối từ container. Lần đầu khởi tạo schema, đặt `POSTGRES_AUTO_MIGRATE=true` trong `.env.docker`, khởi động một instance; sau đó đặt lại false. Bật OCR bằng PROCESSING_ENABLED=true, model và key runtime. Mặc định worker tắt.

```bash
docker run --rm --name sea-codex-api \
  --env-file .env.docker \
  --env-file .env.keys \
  --publish 127.0.0.1:8080:8080 \
  --mount type=volume,src=sea-codex-uploads,dst=/app/storage/uploads \
  sea-codex-server:local
```

```bash
curl -i http://127.0.0.1:8080/health
```

`.env.docker` và `.env.keys` bị gitignore và không được đưa vào Docker build context. Env file được Docker đọc để truyền biến môi trường vào container; API vẫn không tự đọc `.env`. Muốn sử dụng YAML riêng, mount file chỉ đọc rồi truyền `--config=/đường/dẫn/config.yaml` sau tên image.

## Kiểm tra 503 khi tạo gợi ý cấu hình

Sau khi build/deploy image chứa thay đổi logging, các API suggestions ghi thêm event
`catalog suggestion failed` với cùng `request_id` như access log:

```bash
docker logs --since 10m <container-id> 2>&1 | grep 'catalog suggestion failed'
```

- `operation`: `document_type`, `dossier_type` hoặc `comparison_rules`.
- `stage`: `ai_request` (gọi/đọc kết quả provider), `decode` (JSON sai schema), `validation` (cấu hình không hợp lệ), `audit` (ghi audit thất bại).
- `stage=decode` có thêm `decode_error` (ví dụ `unknown_field`, `type_mismatch`), `json_field` nếu tên trường thuộc schema hoặc các wrapper phổ biến, và `expected_type`/`actual_type` khi sai kiểu. Tên trường tùy ý có thể chứa dữ liệu cá nhân nên không được ghi ra log.
- Validation loại hồ sơ có thêm `validation_error`, `json_field` và `item_index` (đếm từ 0) khi lỗi thuộc phần tử mảng. Ví dụ `duplicate_document_role`, `missing_trigger_reason`, `schema_version_unavailable`, `unknown_identifier`, `document_type_not_selected`. Chỉ ghi mã quy tắc, tên trường cố định và chỉ số; không ghi giá trị cấu hình.
- `reason`: ví dụ `timeout`, `network_error`, `provider_http_error`, `output_truncated`, `invalid_json_content`, `invalid_schema`, `invalid_definition`.
- Lỗi AI client có thêm `provider`, `model`, `timeout_ms` đang áp dụng, `attempt` (số lần gọi), `upstream_status` (0 khi chưa có HTTP response), và `provider_error_code` nếu thuộc danh sách mã lỗi đã biết.

Ví dụ `stage=ai_request`, `reason=provider_http_error`, `upstream_status=401` chỉ ra provider từ chối request; `stage=validation` chỉ ra kết quả AI bị kiểm tra cấu hình từ chối. Không kết luận timeout chỉ từ thời gian access log. Thiếu `PROCESSING_TIMEOUT` trong env thì giá trị lấy từ YAML, mặc định `90s` nếu YAML không khai báo.

Log không ghi API key, prompt, nội dung phản hồi AI, URL provider hay raw error. HTTP response của API vẫn dùng thông báo lỗi chung.

Gợi ý loại tài liệu dùng `response_format.type=json_schema` với `strict=true` cho provider OpenAI-compatible. Schema yêu cầu object gốc `name`, `description`, `fields`, `special_requirements`, boolean đúng kiểu và mã định danh rỗng. Prompt kèm ví dụ JSON đầy đủ. Anthropic nhận schema trong prompt theo message contract hiện có. Kết quả vẫn được decode nghiêm ngặt và kiểm tra nghiệp vụ tại backend. OCR, trợ lý và gợi ý quy tắc so sánh giữ định dạng gọi AI hiện tại.

Gợi ý loại hồ sơ cũng dùng schema nghiêm ngặt, với `document_type` giới hạn trong danh sách tài liệu active của công ty đang chọn. AI chỉ đề xuất tên, mô tả và các yêu cầu tài liệu; backend gán thứ tự, mã định danh và pin phiên bản hiện hành từ catalog (không dùng cứng phiên bản 1). Draft trả về vẫn có `definition.documents[].schema_version`, mã rỗng và `rules: []`; gợi ý không tự lưu hoặc kích hoạt. Không chấp nhận loại tài liệu ngoài danh sách đã chọn, cặp loại tài liệu/role trùng, hoặc điều kiện không có lý do.

## Kiểm tra 409 khi ghi quyết định kiểm duyệt

Backend ghi event `dossier decision rejected` với `request_id`, `dossier_id`,
`decision_action`, `reason`, `dossier_status`, `expected_version`,
`current_version` và số `invalid_required_fields`:

```bash
docker logs --since 10m <container-id> 2>&1 | grep 'dossier decision rejected'
```

- `version_mismatch`: request dùng phiên bản cũ; tải lại hồ sơ và xem lại trước khi quyết định, không tự gửi lại mutation.
- `invalid_required_fields`: còn trường bắt buộc trống hoặc `validation` khác `VALID`. Xác nhận cảnh báo không thay thế việc bổ sung/sửa dữ liệu; mở tab Tài liệu, kiểm tra bản gốc và lưu điều chỉnh kèm lý do.
- `dossier_not_ready`, `pending_documents`, `unresolved_issues`: chưa đạt điều kiện trạng thái/tài liệu/đối soát.
- `dossier_locked`, `processing_in_progress`: hồ sơ đã kết thúc hoặc đang xử lý nên không cho phép quyết định tương ứng.

Không suy ra xung đột phiên bản chỉ từ HTTP 409. Nếu `expected_version` và
`current_version` bằng nhau, kiểm tra `reason` và các điều kiện nghiệp vụ.
Log không chứa giá trị trường trích xuất hay nội dung ý kiến kiểm duyệt.

## Thiết lập GitHub và Docker Hub

1. Tạo repository trên Docker Hub, mặc định tên `sea-codex-server` trong namespace của tài khoản đăng nhập.
2. Tạo Docker Hub access token có quyền push vào repository đó.
3. Trong GitHub repository, vào **Settings → Secrets and variables → Actions**, thêm hai **repository secrets**:

   | Secret | Giá trị |
   | --- | --- |
   | `DOCKERHUB_USERNAME` | Docker Hub username, đồng thời là namespace image |
   | `DOCKERHUB_TOKEN` | Access token có quyền push |

4. Nếu muốn tên repository khác, thêm **repository variable** `DOCKERHUB_REPOSITORY`, ví dụ `itmo-vodka-sea-codex-be`. Chỉ điền tên repository, không gồm username hoặc tag. Nếu bỏ trống, dùng `sea-codex-server`.
5. Commit/push các file này lên GitHub. Workflow chạy khi có push lên `main`, pull request vào `main`, push tag `v*`, hoặc thao tác **Run workflow**.

Không cần password database, PASETO key hoặc OCR key để build/push image. Không cấu hình các secrets runtime đó trong workflow build.

## Luồng CI

| Job | Công việc | Khi chạy |
| --- | --- | --- |
| `test` | gofmt check, vet, unit/HTTP tests, build qua `make check` | Mọi trigger |
| `container` | Build image `linux/amd64`, load vào runner; kiểm tra `--help` và quyền runtime ở hai step riêng, in UID/quyền thư mục khi kiểm tra | Sau khi `test` pass |
| `publish` | Login Docker Hub, build/push `linux/amd64` và `linux/arm64` bằng Buildx/QEMU | Sau cả hai job pass, chỉ trên `main` hoặc tag `v*` |

Pull request không login/push Docker Hub và không cần Docker Hub secrets. Chạy thủ công từ nhánh khác `main`/tag `v*` chỉ kiểm tra, không publish. Build cache dùng GitHub Actions cache; token của GitHub chỉ cần `contents: read`.

Tag image:

| Sự kiện | Tag |
| --- | --- |
| Push `main` hoặc chạy thủ công trên `main` | `latest` và `sha-<full-commit-sha>` |
| Push tag như `v1.0.0` | `v1.0.0` và `sha-<full-commit-sha>` |
| Pull request | Image local trên CI runner, không publish |

Release tag không cập nhật `latest`; tag `latest` luôn được cập nhật từ `main`. Có thể pull bằng:

```bash
docker pull <dockerhub-username>/sea-codex-server:latest
```

Thay repository nếu đã đặt `DOCKERHUB_REPOSITORY`. Workflow chỉ publish image, chưa deploy lên VM/Kubernetes.

## Phạm vi kiểm chứng

Test stage `docker build --target test .` chạy vet và Go tests với Poppler thật. CI cài poppler-utils và chạy thêm unit tests của script đánh giá. Source được kiểm tra bằng `make check`; binary được cross-compile với `CGO_ENABLED=0` cho Linux `amd64` và `arm64`. Workflow được kiểm tra bằng actionlint.

Khi kiểm tra lỗi CI, đã khởi động Docker Desktop và tái hiện bằng image `linux/amd64`: `--help` trả exit code 0, còn kiểm tra quyền đọc config trả exit code 1. Nguyên nhân là `COPY --chmod=0444` tạo thư mục `/app/config` với mode `0444`, khiến UID `10001` không đi vào được thư mục.

Dockerfile đã tách việc tạo thư mục mode `0755` khỏi thiết lập file config mode `0444`. Workflow tách hai bước kiểm tra, hiển thị quyền các đường dẫn và trace từng assertion. Test CLI bổ sung bảo đảm `--help`/`-h` thành công ngay cả khi không có config hoặc DB. Chưa push image hoặc kích hoạt workflow trên GitHub từ máy local.

Đã build đầy đủ image sửa cho `linux/amd64` và chạy lại cả hai step trên Docker local: `--help` trả 0; UID `10001`, quyền đi vào thư mục config, đọc file config và ghi storage đều pass, step runtime trả 0. `make check` và actionlint cũng pass. Kiểm chứng container của bản sửa này thực hiện trên `amd64`; manifest đa kiến trúc và publish vẫn do CI thực hiện sau khi code được push.

Tham khảo: [Docker GitHub Actions](https://docs.docker.com/build/ci/github-actions/), [multi-stage build](https://docs.docker.com/build/building/best-practices/), [official Go image tags](https://raw.githubusercontent.com/docker-library/official-images/master/library/golang).

## Image có trợ lý hướng dẫn

Knowledge của assistant nằm trong `internal/assistant/knowledge/articles/*.md`, được `go:embed` vào binary khi Docker build. Dockerfile/CI hiện tại đã copy toàn bộ `internal/`, vì vậy không cần service/vector DB/volume hay token AI trong CI. Tests dùng provider giả. Runtime mặc định AI tắt; cấu hình `ASSISTANT_ENABLED`, `ASSISTANT_API_KEY`, `ASSISTANT_MODEL` và các giới hạn theo [runbook](system-assistant-design.md). Không đưa key vào build args hoặc frontend. Overlay `config/compose.assistant.yaml` bổ sung env vào service `api` của Compose trên VM; không sửa/xóa database.
