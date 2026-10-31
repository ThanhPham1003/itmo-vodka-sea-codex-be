#!/usr/bin/env python3
"""Generate deterministic, obviously synthetic PDFs and ground truth; no personal data."""
import argparse
import json
from pathlib import Path
import random


def pdf(pages):
    objects = ["<< /Type /Catalog /Pages 2 0 R >>", ""]
    kids = []
    font_id = 3 + len(pages) * 2
    for index, (title, fields, noise) in enumerate(pages):
        page_id = len(objects) + 1
        kids.append(f"{page_id} 0 R")
        objects.append(f"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 {font_id} 0 R >> >> /Contents {page_id+1} 0 R >>")
        lines = ["SYNTHETIC - TEST ONLY - NOT A LEGAL DOCUMENT", title]
        lines += [f"{key}: {value}" for key, value in fields.items()]
        commands = ["BT /F1 14 Tf 40 780 Td"]
        for line in lines:
            safe = line.replace("\\", "\\\\").replace("(", "\\(").replace(")", "\\)")
            commands += [f"({safe}) Tj", "0 -36 Td"]
        commands += ["ET"]
        if noise:
            rng = random.Random(42 + index)
            commands += ["0.8 G 0.3 w"]
            for _ in range(120):
                x, y = rng.randint(0, 595), rng.randint(0, 842)
                commands += [f"{x} {y} m {x+2} {y+1} l S"]
        stream = "\n".join(commands)
        objects.append(f"<< /Length {len(stream.encode('ascii'))} >>\nstream\n{stream}\nendstream")
    objects[1] = f"<< /Type /Pages /Kids [{' '.join(kids)}] /Count {len(pages)} >>"
    objects.append("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
    result = bytearray(b"%PDF-1.4\n")
    offsets = [0]
    for number, obj in enumerate(objects, 1):
        offsets.append(len(result))
        result += f"{number} 0 obj\n{obj}\nendobj\n".encode("ascii")
    start = len(result)
    result += f"xref\n0 {len(offsets)}\n0000000000 65535 f \n".encode()
    for offset in offsets[1:]:
        result += f"{offset:010d} 00000 n \n".encode()
    result += f"trailer\n<< /Size {len(offsets)} /Root 1 0 R >>\nstartxref\n{start}\n%%EOF\n".encode()
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    fixtures = []
    # Subject codes and names are fictional and explicitly labelled as test values.
    for name, mismatch, noise, missing in [("match", False, False, False), ("mismatch", True, False, False), ("noisy-match", False, True, False), ("missing-field", False, True, True)]:
        identity = {"subject": "TEST-SUBJECT-001", "full_name": "SYNTHETIC PERSON ALPHA", "birth_date": "1990-01-02"}
        contract = dict(identity)
        if mismatch:
            contract["birth_date"] = "1991-01-02"
        if missing:
            contract["birth_date"] = ""
        documents = [("Synthetic identity", identity, noise), ("Synthetic contract", contract, noise)]
        (args.output / f"{name}.pdf").write_bytes(pdf(documents))
        fixtures.append({"id": name, "file": f"{name}.pdf", "pages": [{"page": i+1, "document_type": title, "fields": fields} for i, (title, fields, _) in enumerate(documents)], "issues": ["BIRTH_DATE_MISMATCH"] if mismatch else (["FIELD_NEEDS_VERIFICATION"] if missing else [])})
    (args.output / "ground-truth.json").write_text(json.dumps({"synthetic": True, "cases": fixtures}, indent=2) + "\n")
    print(f"Generated {len(fixtures)} synthetic two-document PDFs and ground-truth.json")


if __name__ == "__main__":
    main()
