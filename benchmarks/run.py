#!/usr/bin/env python3
"""Run the pinned, local redirect workload and preserve inputs/results."""

import argparse
from datetime import datetime, timezone
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import re
import resource
import subprocess
import sys
from urllib.parse import urlsplit

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent


def positive(value):
    number = int(value)
    if number <= 0:
        raise argparse.ArgumentTypeError("must be a positive integer")
    return number


def local_origin(value):
    url = urlsplit(value)
    loopback = url.hostname == "localhost"
    try:
        loopback = loopback or ipaddress.ip_address(url.hostname).is_loopback
    except ValueError:
        pass
    if (url.scheme != "http" or not loopback or url.username is not None
            or url.password is not None or url.path not in ("", "/")
            or url.query or url.fragment):
        raise ValueError("target must be a loopback HTTP origin without credentials")
    _ = url.port  # Validate the port before creating any artifacts.
    return value.rstrip("/")


def read_fixtures(path):
    fixtures = json.loads(path.read_text())
    if not isinstance(fixtures, list) or not fixtures:
        raise ValueError("fixtures must be a nonempty JSON array")
    codes = set()
    for fixture in fixtures:
        if not isinstance(fixture, dict):
            raise ValueError("each fixture must contain code and destination")
        code, destination = fixture.get("code"), fixture.get("destination")
        if not isinstance(code, str) or not re.fullmatch(r"[A-Za-z0-9]+", code):
            raise ValueError("fixture codes must be alphanumeric path segments")
        if code in codes:
            raise ValueError("fixture codes must be unique")
        codes.add(code)
        if not isinstance(destination, str):
            raise ValueError("fixture destinations must be absolute HTTP(S) URLs")
        url = urlsplit(destination)
        if (url.scheme not in ("http", "https") or not url.hostname
                or url.username is not None or url.password is not None
                or len(destination.encode()) > 2048):
            raise ValueError("fixture destinations must be absolute HTTP(S) URLs without credentials")
    return fixtures


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", required=True)
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--label", default="smoke", help="annotation only; does not change the app")
    parser.add_argument("--rate", type=positive, default=5, help="offered requests/second")
    parser.add_argument("--duration", type=positive, default=10, help="measurement seconds")
    parser.add_argument("--vus", type=positive, default=5, help="fixed generator concurrency budget")
    parser.add_argument("--timeout-ms", type=positive, default=3000)
    parser.add_argument("--k6", default="k6", help="path to the pinned k6 binary")
    parser.add_argument("--output", type=Path, help="new directory; existing paths are refused")
    parser.add_argument("--containers", nargs="*", default=[], help="suite-owned containers to sample with docker stats")
    args = parser.parse_args()
    target = local_origin(args.target)
    fixtures = read_fixtures(args.fixtures)
    version = subprocess.check_output([args.k6, "version"], text=True).strip()
    pinned = (HERE / ".k6-version").read_text().strip()
    if not version.startswith(f"k6 v{pinned} "):
        raise ValueError(f"expected k6 {pinned}; got {version}")
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S.%fZ")
    output = (args.output or HERE / "results" / timestamp).resolve()
    # Capture provenance without claiming the running application uses this revision.
    head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    dirty = bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True))
    config = dict(target=target, rate=args.rate, duration_seconds=args.duration,
                  vus=args.vus, timeout_ms=args.timeout_ms)
    metadata = dict(schema_version=1, started_at_utc=timestamp, label=args.label,
                    k6_version=version, harness_revision=head, harness_dirty=dirty,
                    script_sha256=hashlib.sha256((HERE / "redirect.js").read_bytes()).hexdigest(),
                    config=config, fixture_count=len(fixtures))
    output.mkdir(parents=True, exist_ok=False)
    (output / "fixtures.json").write_text(json.dumps(fixtures, indent=2) + "\n")
    (output / "run.json").write_text(json.dumps(metadata, indent=2) + "\n")
    # Ambient k6 flags must not silently override the archived workload/options.
    env = {k: v for k, v in os.environ.items() if not k.startswith(("K6_", "BENCH_"))}
    env.update(BENCH_CONFIG=json.dumps(config), BENCH_FIXTURES=str(output / "fixtures.json"),
               BENCH_SUMMARY=str(output / "summary.json"), K6_NO_USAGE_REPORT="true")
    sampler = None
    with (output / "console.log").open("w") as log, (output / "resources.jsonl").open("w") as samples, (output / "resources.log").open("w") as stats_log:
        try:
            if args.containers:
                sampler = subprocess.Popen(["docker", "stats", "--format", "{{json .}}", *args.containers],
                                           stdout=samples, stderr=stats_log)
            before = resource.getrusage(resource.RUSAGE_CHILDREN)
            try:
                result = subprocess.run([args.k6, "run", str(HERE / "redirect.js")],
                                        cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT,
                                        timeout=args.duration + args.timeout_ms / 1000 + 20)
                exit_code = result.returncode
            except subprocess.TimeoutExpired:
                exit_code = 124
            after = resource.getrusage(resource.RUSAGE_CHILDREN)
            metadata["generator_cpu_seconds"] = dict(user=after.ru_utime - before.ru_utime,
                                                     system=after.ru_stime - before.ru_stime)
        finally:
            if sampler is not None:
                sampler.terminate()
                try:
                    sampler.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    sampler.kill()
                    sampler.wait()
    metadata.update(exit_code=exit_code,
                    finished_at_utc=datetime.now(timezone.utc).isoformat())
    (output / "run.json").write_text(json.dumps(metadata, indent=2) + "\n")
    print(f"k6 exit {exit_code}; results: {output}")
    if exit_code:
        print((output / "console.log").read_text(), file=sys.stderr)
    return exit_code


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print(f"benchmark: {error}", file=sys.stderr)
        sys.exit(1)
