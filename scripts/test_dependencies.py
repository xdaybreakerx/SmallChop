"""Containment and failure-evidence checks without starting Docker."""
import contextlib
import fcntl
import io
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("check_dependencies", Path(__file__).with_name("check-dependencies.py"))
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


class DependencyCheckTests(unittest.TestCase):
    def test_existing_resources_are_never_reused_or_cleaned(self):
        for resource in ["ps", "volume", "network"]:
            with self.subTest(resource=resource):
                calls = []

                def run(args, **kwargs):
                    calls.append(args)
                    return "existing-resource" if args[1] == resource else ""

                with patch.object(check, "run", side_effect=run):
                    with self.assertRaises(RuntimeError):
                        check.check_dependencies()
                self.assertFalse(any(args[1] == "compose" for args in calls))

    def test_failed_setup_retains_evidence_and_cleanup_outcome(self):
        for cleanup_fails in [False, True]:
            with self.subTest(cleanup_fails=cleanup_fails):
                calls = []

                def run(args, **kwargs):
                    calls.append(args)
                    if args[1] != "compose":
                        return ""
                    if "up" in args:
                        raise RuntimeError("synthetic setup failure")
                    if "down" in args and cleanup_fails:
                        raise RuntimeError("synthetic cleanup failure")
                    return ""

                output = io.StringIO()
                with patch.object(check, "run", side_effect=run), contextlib.redirect_stdout(output):
                    with self.assertRaises(RuntimeError):
                        check.check_dependencies()
                result = json.loads(output.getvalue())
                self.assertEqual(result["result"], "FAIL")
                self.assertEqual(result["error"], "synthetic setup failure")
                self.assertEqual(result["cleanup"], "failed" if cleanup_fails else "completed")
                self.assertTrue(any("down" in args and "--volumes" in args for args in calls))

    def test_overlapping_local_check_is_refused_before_docker(self):
        project = "smallchop-local-lock-unit-test"
        path = Path(tempfile.gettempdir()) / (project + ".lock")
        with path.open("a") as owned_lock:
            fcntl.flock(owned_lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            with patch.object(check, "PROJECT", project), patch.object(check, "check_dependencies") as operation:
                with self.assertRaisesRegex(RuntimeError, "Another local"):
                    check.main()
                operation.assert_not_called()


if __name__ == "__main__":
    unittest.main()
