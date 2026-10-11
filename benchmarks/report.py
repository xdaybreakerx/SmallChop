#!/usr/bin/env python3
"""Rebuild a comparison report from archived suite measurements."""

import argparse
import json
from pathlib import Path
from statistics import median

COUNTERS = ("mongo_find", "redis_get", "redis_set", "redis_hits", "redis_misses")


def qualify(directory, mode, before, after):
    run = json.loads((directory / "run.json").read_text())
    metrics = json.loads((directory / "summary.json").read_text())["metrics"]

    def value(name, field, default=0):
        return metrics.get(name, {}).get("values", {}).get(field, default)

    requests = value("http_reqs", "count")
    successes = value("successful_redirects", "count")
    delta = {key: after[key] - before[key] for key in COUNTERS}
    reasons = []
    if (run["exit_code"] != 0 or requests <= 0 or successes != requests
            or value("valid_redirects", "rate") != 1 or value("http_req_failed", "rate") != 0):
        reasons.append("redirect correctness/HTTP threshold failed")
    if value("dropped_iterations", "count") != 0:
        reasons.append("generator dropped iterations")
    if (any(n < 0 for n in delta.values()) or after["mongo_uptime"] < before["mongo_uptime"]
            or after["redis_run_id"] != before["redis_run_id"]):
        reasons.append("backend counters reset or instance restarted")
    expected = dict(mongo_find=requests, redis_get=0, redis_set=0, redis_hits=0, redis_misses=0)
    if mode == "mongo-redis":
        expected.update(mongo_find=0, redis_get=requests, redis_hits=requests)
    elif mode != "mongo-only":
        raise ValueError(f"unknown mode: {mode}")
    if delta != expected:
        reasons.append("backend counters do not confirm the declared path")
    latency = metrics.get("successful_redirect_duration", {}).get("values", {})
    if "p(95)" not in latency or "p(99)" not in latency:
        reasons.append("successful latency summary is missing")
    return dict(mode=mode, qualified=not reasons, reasons=reasons, requests=requests,
                successes=successes, successful_rps=successes / run["config"]["duration_seconds"],
                dropped_iterations=value("dropped_iterations", "count"),
                rate_limited=value("rate_limited_redirects", "count"),
                server_errors=value("server_error_redirects", "count"),
                transport_errors=value("transport_errors", "count"),
                latency_ms=latency, backend_delta=delta,
                cache_hit_ratio=delta["redis_hits"] / delta["redis_get"] if delta["redis_get"] else None)


def write_report(output):
    suite = json.loads((output / "suite.json").read_text())
    points = suite["measurements"]
    def key(point):
        return point["rate"], point["mode"], point["repetition"]
    complete = len(points) == len(suite["plan"]) and {key(p) for p in points} == {key(p) for p in suite["plan"]}
    groups = []
    for rate in suite["config"]["rates"]:
        for mode in ("mongo-only", "mongo-redis"):
            runs = [p for p in points if p["rate"] == rate and p["mode"] == mode]
            valid = [p for p in runs if p["qualified"]]
            qualified = (len(valid) == suite["config"]["repetitions"]
                         and {p["repetition"] for p in valid} == set(range(1, suite["config"]["repetitions"] + 1)))
            p95s = [p["latency_ms"]["p(95)"] for p in valid]
            groups.append(dict(rate=rate, mode=mode, qualified=qualified,
                               valid_runs=len(valid), total_runs=len(runs),
                               median_run_p95_ms=median(p95s) if p95s else None,
                               min_run_p95_ms=min(p95s) if p95s else None,
                               max_run_p95_ms=max(p95s) if p95s else None,
                               median_successful_rps=median(p["successful_rps"] for p in valid) if valid else None))
    comparisons = []
    for rate in suite["config"]["rates"]:
        pair = [g for g in groups if g["rate"] == rate]
        if all(g["qualified"] for g in pair):
            changes = []
            for repetition in range(1, suite["config"]["repetitions"] + 1):
                baseline = next(p for p in points if p["rate"] == rate and p["repetition"] == repetition and p["mode"] == "mongo-only")
                cached = next(p for p in points if p["rate"] == rate and p["repetition"] == repetition and p["mode"] == "mongo-redis")
                base_p95 = baseline["latency_ms"]["p(95)"]
                if base_p95 > 0:
                    changes.append(100 * (base_p95 - cached["latency_ms"]["p(95)"]) / base_p95)
            comparisons.append(dict(rate=rate, median_paired_p95_reduction_percent=median(changes) if changes else None))
    result = dict(schema_version=1, complete=complete,
                  qualified=complete and all(p["qualified"] for p in points) and not suite.get("error")
                  and not suite.get("cleanup", "").startswith("FAILED"),
                  groups=groups, comparisons=comparisons,
                  highest_fully_qualified_tested_rate={mode: max(
                      (g["rate"] for g in groups if g["mode"] == mode and g["qualified"]), default=None)
                      for mode in ("mongo-only", "mongo-redis")})
    (output / "comparison.json").write_text(json.dumps(result, indent=2) + "\n")
    lines = ["# Local redirect benchmark comparison", "",
             f"Complete: {complete}. All points qualified: {result['qualified']}.", "",
             "Warm MongoDB in both modes; warmed Redis in the cached mode. Uniform, deterministic access.",
             "Latencies are HTTP request duration for validated 308 responses. Values below are medians of run p95s, not pooled percentiles.", "",
             "| Offered RPS | Mode | Qualified runs | Median run p95 (ms) | Run p95 range (ms) | Median successful RPS |",
             "| --- | --- | --- | --- | --- | --- |"]
    for group in groups:
        def number(key):
            return f"{group[key]:.3f}" if group[key] is not None else "—"
        lines.append(f"| {group['rate']} | {group['mode']} | {group['valid_runs']}/{suite['config']['repetitions']} | "
                     f"{number('median_run_p95_ms')} | {number('min_run_p95_ms')}–{number('max_run_p95_ms')} | "
                     f"{number('median_successful_rps')} |")
    lines += ["", "Paired p95 reductions are reported only where every repetition in both modes qualifies. Positive means lower cached latency; negative means higher."]
    for comparison in comparisons:
        change = comparison["median_paired_p95_reduction_percent"]
        if change is not None:
            lines.append(f"- {comparison['rate']} offered RPS: {change:.2f}% median paired p95 reduction.")
    lines += ["", "Successful RPS is completed valid redirects divided by configured measurement seconds; in-flight completions may finish during grace.",
              "See suite.json and each point's backend.json, summary.json, run.json and resource samples for raw evidence.",
              "The highest qualified tested rate is a tested point, not maximum capacity or an SLO. No latency SLO is imposed.",
              "Shared local host/Docker/generator contention and short runs limit generalization. A warm-cache workload does not describe cold/mixed traffic.", ""]
    failed = [p for p in points if not p["qualified"]]
    if failed:
        lines += ["## Unqualified points", ""]
        lines += [f"- {p['directory']}: {'; '.join(p['reasons'])}" for p in failed]
        lines.append("")
    if suite.get("error"):
        lines += ["Suite stopped before completion. See suite.json error and lifecycle.log.", ""]
    (output / "comparison.md").write_text("\n".join(lines))
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    write_report(args.output.resolve())
