"""Regression tests for benchmark qualification, planning and isolation guards."""

import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from report import qualify, write_report
from suite import fixtures, plan, Stack, SuiteLock


class QualificationTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.output = Path(self.temporary.name)
        self.run = dict(exit_code=0, config=dict(duration_seconds=2))
        self.metrics = {name: dict(values=values) for name, values in {
            "http_reqs": dict(count=20), "successful_redirects": dict(count=20),
            "valid_redirects": dict(rate=1), "http_req_failed": dict(rate=0),
            "dropped_iterations": dict(count=0),
            "successful_redirect_duration": {"p(95)": 2, "p(99)": 4},
        }.items()}
        self.before = dict(mongo_find=100, redis_get=100, redis_set=20,
                           redis_hits=100, redis_misses=10, mongo_uptime=10, redis_run_id="same")
        self.after = dict(self.before, mongo_find=120, mongo_uptime=20)

    def evaluate(self, mode="mongo-only"):
        (self.output / "run.json").write_text(json.dumps(self.run))
        (self.output / "summary.json").write_text(json.dumps(dict(metrics=self.metrics)))
        return qualify(self.output, mode, self.before, self.after)

    def test_valid_paths_and_wrong_mode(self):
        self.assertTrue(self.evaluate()["qualified"])
        self.assertFalse(self.evaluate("mongo-redis")["qualified"])
        self.after.update(mongo_find=100, redis_get=120, redis_hits=120)
        cached = self.evaluate("mongo-redis")
        self.assertTrue(cached["qualified"])
        self.assertEqual(cached["cache_hit_ratio"], 1)
        self.assertFalse(self.evaluate()["qualified"])

    def test_fast_errors_cannot_qualify(self):
        self.metrics["successful_redirects"]["values"]["count"] = 0
        self.metrics["valid_redirects"]["values"]["rate"] = 0
        self.metrics["successful_redirect_duration"]["values"]["p(95)"] = 0.001
        self.assertFalse(self.evaluate()["qualified"])

    def test_drops_and_counter_resets_cannot_qualify(self):
        self.metrics["dropped_iterations"]["values"]["count"] = 1
        self.assertFalse(self.evaluate()["qualified"])
        self.metrics["dropped_iterations"]["values"]["count"] = 0
        self.after["redis_run_id"] = "restarted"
        self.assertFalse(self.evaluate()["qualified"])
        self.after["redis_run_id"] = "same"
        self.after["mongo_find"] = 1
        self.assertFalse(self.evaluate()["qualified"])

    def test_unexpected_cache_fills_invalidate_warm_result(self):
        self.after.update(mongo_find=100, redis_get=120, redis_hits=120, redis_set=21)
        self.assertFalse(self.evaluate("mongo-redis")["qualified"])

    def test_incomplete_or_invalid_pairs_do_not_report_improvement(self):
        point = self.evaluate()
        point.update(rate=10, repetition=1, directory="baseline")
        suite = dict(config=dict(rates=[10], repetitions=1), plan=plan([10], 1), measurements=[point])
        (self.output / "suite.json").write_text(json.dumps(suite))
        result = write_report(self.output)
        self.assertFalse(result["complete"])
        self.assertEqual(result["comparisons"], [])
        cached = copy.deepcopy(point)
        cached.update(mode="mongo-redis", directory="cached", qualified=False)
        suite["measurements"].append(cached)
        (self.output / "suite.json").write_text(json.dumps(suite))
        self.assertEqual(write_report(self.output)["comparisons"], [])
        cached["qualified"] = True
        cached["latency_ms"]["p(95)"] = 3  # A slower cache result is legitimate.
        (self.output / "suite.json").write_text(json.dumps(suite))
        self.assertEqual(write_report(self.output)["comparisons"][0]["median_paired_p95_reduction_percent"], -50)


class IsolationTests(unittest.TestCase):
    def test_concurrent_project_lock(self):
        with tempfile.TemporaryDirectory() as temporary:
            project = "smallchop-local-benchmark-unit-" + Path(temporary).name
            with SuiteLock(project):
                with self.assertRaisesRegex(RuntimeError, "Another local suite"):
                    with SuiteLock(project):
                        self.fail("second lock was acquired")
            with SuiteLock(project):
                pass  # Lock is released when the owning suite exits.

    def test_fixture_encoding_and_order(self):
        rows = fixtures(64)
        self.assertEqual(rows[0]["code"], "c")
        self.assertEqual(rows[len("bcdfghjklmnpqrstvwxyzBCDFGHJKLMNPQRSTVWXYZ0123456789") - 1]["code"], "cb")
        self.assertEqual(len({row["destination"] for row in rows}), 64)
        schedule = plan([20, 40], 2)
        self.assertEqual(len(schedule), 8)
        self.assertEqual([p["mode"] for p in schedule if p["rate"] == 20],
                         ["mongo-only", "mongo-redis", "mongo-redis", "mongo-only"])

    def test_existing_resources_are_never_cleaned(self):
        for index, kind in enumerate(("containers", "volumes", "networks")):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temporary:
                work = Path(temporary)
                from argparse import Namespace
                args = Namespace(project="smallchop-local-benchmark", rates=[20], dataset=64)
                stack = Stack(args, work, work)
                responses = ["5.5.1"] + [""] * index + ["existing-resource"]
                with patch.object(stack, "run", side_effect=responses), patch.object(stack, "compose") as compose:
                    with self.assertRaisesRegex(RuntimeError, "refusing reuse/deletion"):
                        stack.start()
                    stack.close()
                    compose.assert_not_called()


if __name__ == "__main__":
    unittest.main()
