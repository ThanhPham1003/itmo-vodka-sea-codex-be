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
