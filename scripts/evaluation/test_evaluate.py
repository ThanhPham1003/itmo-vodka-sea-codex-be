import unittest
from evaluate import evaluate


class EvaluationTest(unittest.TestCase):
    def test_missing_predictions_are_not_silently_ignored(self):
        truth = {"cases": [{"id": "a", "pages": [{"page": 1, "document_type": "ID", "fields": {"name": "SYNTHETIC"}}], "issues": ["MISMATCH"]}]}
        report = evaluate(truth, {"cases": []})
        self.assertEqual(report["classification_accuracy"], 0)
        self.assertEqual(report["field_exact_match"], 0)
        self.assertEqual(report["issue_recall"], 0)
        self.assertIsNone(report["total_recorded_cost_usd"])
        report = evaluate(truth, {"cases": [{"id": "a", "pages": truth["cases"][0]["pages"], "issues": ["MISMATCH", "FALSE_ALARM"], "latency_ms": 10}]})
        self.assertEqual(report["classification_accuracy"], 1)
        self.assertEqual(report["issue_precision"], 0.5)
        self.assertEqual(report["mean_latency_ms"], 10)


if __name__ == "__main__":
    unittest.main()
