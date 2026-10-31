# Synthetic dataset và đánh giá

Chạy từ root, Python 3, không cần thư viện ngoài:

```bash
python3 scripts/evaluation/generate.py --output /tmp/sea-synthetic
python3 -m unittest discover -s scripts/evaluation
```

Generator tạo 4 PDF, mỗi file có hai loại giấy tờ, gồm khớp, lệch ngày sinh, nhiễu dạng đốm và thiếu trường. Mọi trang có nhãn **SYNTHETIC / TEST ONLY**, không mô phỏng con dấu, QR, chữ ký hoặc mẫu giấy tờ thật. `ground-truth.json` có đáp án theo trang và lỗi có chủ đích. Nhiễu này chưa đại diện đầy đủ ảnh chụp mờ/góc nghiêng ngoài thực tế.

Tạo hai document types bằng API, dùng tên/mô tả “Synthetic identity” và “Synthetic contract”; khai báo subject/full_name kiểu text, birth_date kiểu date. Dùng các code/key server trả về khi tạo dossier type, đặt luật DATE_EQUAL ghép theo subject. Tải từng fixture lên, chờ worker hoàn tất và ghi **kết quả thật** vào `predictions.json`. Chuẩn hóa code/key tự sinh về tên logic trong ground truth; không so mã UUID giữa các lần chạy. Dùng PII reveal có audit nếu trường được khai báo PII. Một prediction có dạng:

```json
{"cases":[{"id":"mismatch","pages":[{"page":1,"document_type":"Synthetic identity","fields":{"subject":"TEST-SUBJECT-001","full_name":"SYNTHETIC PERSON ALPHA","birth_date":"1990-01-02"}}],"issues":["BIRTH_DATE_MISMATCH"],"latency_ms":1234,"cost_usd":0.01}]}
```

Ví dụ chỉ mô tả định dạng, không phải kết quả benchmark. Ghi đủ các trang/case. Đo thời gian từ upload đến xử lý xong; chi phí phải lấy từ telemetry/billing provider nếu có, không ước lượng ngầm. Script không tự gọi AI, không tạo kết quả dự đoán và không tự gửi hồ sơ đi.

```bash
python3 scripts/evaluation/evaluate.py \
  --truth /tmp/sea-synthetic/ground-truth.json \
  --predictions /path/to/predictions.json
```

Kết quả gồm accuracy phân loại theo trang, exact match từng trường, precision/recall issue theo case, latency trung bình và tổng chi phí có ghi nhận. Case/trang/trường thiếu prediction tính sai; chi phí/thời gian chưa có trả null. Null precision/recall nghĩa là không có mẫu tương ứng, không phải 100%. Đây là công cụ đánh giá trên tập giả lập; chưa có kết quả định lượng trên hồ sơ thật.
