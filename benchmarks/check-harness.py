#!/usr/bin/env python3
"""Verify the real k6 script against disposable loopback HTTP fixtures."""

import argparse
from collections import Counter
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import subprocess
import tempfile
import threading
import time

HERE = Path(__file__).resolve().parent


def verify(k6, output):
    requests = Counter()
    mode = "valid"
    destinations = {}

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            request_mode = mode
            requests[self.path] += 1
            if request_mode in ("timeout", "dropped"):
                time.sleep(0.3)
            status = {"wrong-status": 200, "limited": 429, "server-error": 503}.get(request_mode, 308)
            destination = destinations.get(self.path, "https://example.org/unexpected")
            if request_mode == "wrong-location":
                destination += "-wrong"
            try:
                self.send_response(status)
                self.send_header("Location", destination)
                self.send_header("Content-Length", "0")
                self.end_headers()
            except (BrokenPipeError, ConnectionResetError):
                pass  # The timeout scenario deliberately abandons the response.

        def log_message(self, *args):
            pass

    class Server(ThreadingHTTPServer):
        daemon_threads = False  # server_close joins even delayed fixture workers.

    server = Server(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    target = f"http://127.0.0.1:{server.server_port}"
    destinations.update({"/r/c": target + "/destination/a%2Fb?x=1&x=2#section",
                         "/r/d": "https://example.org/second"})
    fixtures = output / "fixtures.json"
    fixtures.write_text(json.dumps([dict(code=path.rsplit("/", 1)[1], destination=url)
                                    for path, url in destinations.items()]))
    evidence = []
    try:
        for mode in ("valid", "wrong-location", "wrong-status", "limited", "server-error", "timeout", "dropped"):
            run_output = output / mode
            args = ["python3", str(HERE / "run.py"), "--k6", k6,
                    "--target", target, "--fixtures", str(fixtures), "--label", "mock-" + mode,
                    "--rate", "40" if mode == "dropped" else "4", "--duration", "1",
                    "--vus", "1" if mode == "dropped" else "2",
                    "--timeout-ms", "50" if mode == "timeout" else "1000",
                    "--output", str(run_output)]
            result = subprocess.run(args, capture_output=True, text=True)
            assert (result.returncode == 0) == (mode == "valid"), result.stdout + result.stderr
            data = json.loads((run_output / "summary.json").read_text())
            metrics = data["metrics"]
            valid = metrics["valid_redirects"]["values"]["rate"]
            dropped = metrics["dropped_iterations"]["values"]["count"]
            assert valid == (1 if mode in ("valid", "dropped") else 0), data
            assert (dropped > 0) == (mode == "dropped"), data
            assert (metrics["rate_limited_redirects"]["values"]["count"] > 0) == (mode == "limited")
            assert (metrics["server_error_redirects"]["values"]["count"] > 0) == (mode == "server-error")
            assert (metrics["transport_errors"]["values"]["count"] > 0) == (mode == "timeout")
            if mode == "valid":
                assert metrics["successful_redirects"]["values"]["count"] >= 4, data
                assert "p(95)" in metrics["successful_redirect_duration"]["values"], data
            assert metrics["valid_redirects"]["thresholds"]["rate==1"]["ok"] == (valid == 1)
            assert metrics["dropped_iterations"]["thresholds"]["count==0"]["ok"] == (dropped == 0)
            metadata = json.loads((run_output / "run.json").read_text())
            assert metadata["exit_code"] == result.returncode
            assert metadata["config"]["rate"] == (40 if mode == "dropped" else 4)
            evidence.append(dict(case=mode, exit_code=result.returncode,
                                 valid_redirect_rate=valid, dropped_iterations=dropped))
            print(f"PASS {mode}")
        assert set(requests) == set(destinations), f"redirects followed or fixtures skipped: {requests}"
        # Invalid input and pre-existing output must fail without replacing artifacts.
        old_metadata = (output / "valid" / "run.json").read_bytes()
        repeat = subprocess.run(args[:-1] + [str(output / "valid")], capture_output=True)
        assert repeat.returncode != 0
        assert (output / "valid" / "run.json").read_bytes() == old_metadata
        remote = subprocess.run(["python3", str(HERE / "run.py"), "--target",
                                 "https://example.com", "--fixtures", str(fixtures)], capture_output=True)
        assert remote.returncode != 0 and b"loopback HTTP origin" in remote.stderr
        print("PASS input validation, output preservation, exact fixtures, no redirect following")
        (output / "verification.json").write_text(json.dumps(evidence, indent=2) + "\n")
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--k6", default="k6")
    parser.add_argument("--output", type=Path, help="new directory for mock evidence; otherwise temporary")
    args = parser.parse_args()
    if args.output:
        args.output.mkdir(parents=True, exist_ok=False)
        verify(args.k6, args.output.resolve())
    else:
        with tempfile.TemporaryDirectory(prefix="smallchop-k6-check-") as directory:
            verify(args.k6, Path(directory))
