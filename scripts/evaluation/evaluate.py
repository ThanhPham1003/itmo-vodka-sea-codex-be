#!/usr/bin/env python3
"""Evaluate recorded predictions against synthetic ground truth; never fabricate AI results."""
import argparse
import json
import math
from pathlib import Path


def evaluate(truth, predictions):
    expected = {case["id"]: case for case in truth["cases"]}
    actual = {}
    for case in predictions["cases"]:
        if case["id"] in actual or case["id"] not in expected:
            raise ValueError("duplicate or unknown prediction case")
        actual[case["id"]] = case
    classification = fields = class_total = field_total = tp = fp = fn = 0
    latency = []
    cost = 0.0
    cost_cases = 0
    for key, case in expected.items():
        prediction = actual.get(key, {})
        pages = {p["page"]: p for p in prediction.get("pages", [])}
        if len(pages) != len(prediction.get("pages", [])):
            raise ValueError("duplicate page prediction")
        valid_pages = {p["page"] for p in case["pages"]}
        if set(pages) - valid_pages:
            raise ValueError("unknown prediction page")
        for page in case["pages"]:
            found = pages.get(page["page"], {})
            class_total += 1
            classification += found.get("document_type") == page["document_type"]
            for name, value in page["fields"].items():
                field_total += 1
                fields += found.get("fields", {}).get(name) == value
        wanted, detected = set(case["issues"]), set(prediction.get("issues", []))
        tp += len(wanted & detected)
        fp += len(detected - wanted)
        fn += len(wanted - detected)
        for metric in ("latency_ms", "cost_usd"):
            if metric in prediction:
                number = float(prediction[metric])
                if number < 0 or not math.isfinite(number):
                    raise ValueError("invalid numeric metric")
                if metric == "latency_ms":
                    latency.append(number)
                else:
                    cost += number
                    cost_cases += 1
    ratio = lambda numerator, denominator: numerator / denominator if denominator else None
    return {"cases": len(expected), "recorded_cases": len(actual), "missing_cases": len(expected)-len(actual), "classification_accuracy": ratio(classification, class_total), "field_exact_match": ratio(fields, field_total), "issue_precision": ratio(tp, tp+fp), "issue_recall": ratio(tp, tp+fn), "true_positives": tp, "false_positives": fp, "false_negatives": fn, "mean_latency_ms": ratio(sum(latency), len(latency)), "latency_recorded_cases": len(latency), "total_recorded_cost_usd": cost if cost_cases else None, "cost_recorded_cases": cost_cases}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--truth", type=Path, required=True)
    parser.add_argument("--predictions", type=Path, required=True)
    args = parser.parse_args()
    try:
        report = evaluate(json.loads(args.truth.read_text()), json.loads(args.predictions.read_text()))
    except (ValueError, KeyError, TypeError) as exc:
        parser.error(str(exc))
    print(json.dumps(report, indent=2, allow_nan=False))


if __name__ == "__main__":
    main()
