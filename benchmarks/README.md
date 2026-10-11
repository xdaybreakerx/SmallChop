# Application redirect benchmarks

This suite issues a fixed offered rate of GET requests to `/r/{code}` through a
local HTTP origin. Each response must be **308 with the exact expected Location**;
redirects are never followed. The workload cycles uniformly through known mappings.

## Requirements and quick run

Use Python 3.10+ on macOS/Linux, Docker with Compose **2.24.4+**, and **k6 2.3.0**
(pinned in [.k6-version](.k6-version)). Download the
appropriate binary from the [official release](https://github.com/grafana/k6/releases/tag/v2.3.0)
and verify its published checksum. The runner checks the version and disables usage
reporting. No cloud account or root environment files are required.

Run the full comparison from the repository root:

```sh
python3 benchmarks/suite.py --k6 /path/to/k6
```

This builds one app image, creates a fresh dedicated Compose project, seeds 1,000
deterministic mappings, and compares Mongo-only with Mongo-plus-Redis at **100, 500,
and 1,000 offered requests/second**, with **three repetitions** and **30 seconds of
measurement** per point. Each point starts a fresh app process using that same image,
warms every fixture, then runs a separate five-second offered-rate warm-up. MongoDB
is warmed in both modes; cached measurements use warmed Redis. Pair order alternates
across repetitions and rate points. Expect roughly 9 minutes of measurement plus
build/startup/warm-up overhead.

For a short integration check:

```sh
python3 benchmarks/suite.py --check --k6 /path/to/k6
```

This checks 64 mappings at 20/40 offered RPS, two repetitions and two-second
measurements. These short checks establish reproducibility/correctness, not portfolio
performance figures. CI runs them with the mock harness and qualification tests;
it has no performance threshold on shared runners.

The suite supplies disposable example credentials and temporary env files, loopback
ingress, `172.30.83.0/29` for the proxy network, and a dedicated Mongo volume. Default
project: `smallchop-local-benchmark`; existing resources are refused. A local lock
prevents two runs sharing that project. Only newly created project resources are
removed in cleanup, including after failures. Run one suite at a time per Docker
engine; the fixed proxy subnet must not overlap another Docker/VPN network. The app
and databases have no published ports.

The [Compose override](compose.yml) fixes each service at one CPU; memory limits are
512 MiB app, 1 GiB MongoDB, 256 MiB Redis and 256 MiB Caddy. MongoDB's WiredTiger cache
is 0.25 GiB; Redis's maxmemory is 128 MiB with no eviction. Both modes keep Redis
running. Benchmark-only redirect rate/burst are the greater of 10× the largest offered
rate and 2× the dataset size, recorded in each point's effective app configuration.
Normal application limiter defaults stay unchanged.

Customize a rate sweep or longer measurements:

```sh
python3 benchmarks/suite.py --k6 /path/to/k6 \
  --rates 100 250 500 1000 2000 --duration 60 --warmup 10 \
  --repetitions 3 --dataset 1000 --vus 200
```

`--vus` is a fixed generator budget, default 100. Failed warm-ups/measurement points
remain in the evidence and do not qualify for comparisons. An error after stack
startup begins stops the suite and still writes an incomplete report. Exit zero means all planned
points qualified; nonzero means failed/incomplete evidence, not necessarily a broken
service. Inspect it before changing the rate or generator budget.

## Individual existing-app workload

Create mappings in your disposable local application, then put their actual codes
and destinations in a JSON array matching [fixtures.example.json](fixtures.example.json).
The example illustrates the format; the harness does not create these mappings.
Keep private fixture files inside the ignored `benchmarks/results/` directory.

```sh
python3 benchmarks/run.py \
  --k6 /path/to/k6 \
  --target http://127.0.0.1:8080 \
  --fixtures /path/to/fixtures.json \
  --label local-smoke --rate 5 --duration 10 --vus 5
```

Run from any directory. Targets are restricted to loopback HTTP origins. The default
rate is below the application's normal single-client redirect allowance; higher
rates need an explicitly recorded benchmark-only limiter configuration. A 429 fails
correctness; it is never counted as a successful redirect. Ambient `K6_*` variables
are removed so they cannot silently change the archived configuration.

The runner creates a unique ignored `benchmarks/results/<UTC timestamp>/` directory.
Use `--output /path/to/new-directory` to select another location; existing directories
are refused. Results contain:

- `run.json`: workload, label, k6 version, harness revision/dirty state, script hash,
  timestamps and exit code. The harness revision does **not** establish the revision
  of the running application.
- `fixtures.json`: the exact workload inputs.
- `summary.json`: k6 metrics, latency percentiles, redirect checks, 429/5xx/transport
  error counts and thresholds.
- `console.log`: generator diagnostics, including scheduling warnings.
- `resources.jsonl`: Docker CPU/memory/network snapshots when the suite supplies
  its container IDs; `resources.log` records sampler diagnostics. These are samples,
  not averages; very short checks may have few/no samples. `run.json` also records
  generator CPU time.

Exit zero requires every redirect to pass, at least one successful redirect, no HTTP
failures and no dropped iterations. Use `successful_redirect_duration` for validated
response latency; `http_req_duration` also includes errors. Inspect the status metrics
(`rate_limited_redirects`, `server_error_redirects`, `transport_errors`) and
`successful_redirects` count alongside latency. There is no arbitrary performance
threshold yet. Dropped iterations invalidate a matched offered-rate comparison and
can reflect insufficient VUs or generator resources as well as slow responses.

## Check the harness

```sh
python3 benchmarks/check-harness.py --k6 /path/to/k6
python3 -m unittest discover -s benchmarks -p 'test_*.py' -v
```

This runs the actual k6 script against a disposable Python HTTP server on a random
loopback port. It verifies exact destinations/escaping/fragments, no destination
fetches, nonzero exits for wrong Location, wrong status, 429, 503, timeouts and dropped
iterations, plus input/output protection. It does not run the application or measure
MongoDB/Redis performance. Add `--output /path/to/new-directory` to retain mock evidence.

## Mongo-only versus Mongo-plus-Redis comparison

`suite.py` sets `CACHE_ENABLED=false` for Mongo-only and `true` for the cached mode.
False bypasses startup/read/fill completely; defaults remain enabled. `run.py --label`
is annotation only and never configures an existing app.

Before/after each measurement, the suite snapshots MongoDB's
[`metrics.commands.find.total`](https://www.mongodb.com/docs/manual/reference/command/serverstatus/)
and Redis [INFO](https://redis.io/docs/latest/commands/info/) GET/SET calls and keyspace
hits/misses. No probes run during the HTTP measurement. Dedicated instances contain
only this workload; health checks use pings/page requests, not lookup commands.
These counters are instance-wide and must not be used to infer this workload's path
on a shared database.

A qualified Mongo-only point requires exactly one find per HTTP request and no cache
GET/SET/hits/misses. A qualified warm-cache point requires one Redis hit per request,
zero misses/fills and zero Mongo finds. Both require valid redirects, no HTTP failures
or dropped iterations, and no counter reset/restart. Counter disagreements invalidate
the point even when its redirects are correct.

## Results and interpretation

Full-suite output is an ignored `benchmarks/results/suite-<timestamp>/` directory:

- `suite.json`: plan, point qualification/reasons, source revision/dirty state, app
  source digest, image IDs/limits, fixture digest and Docker/generator context.
- `seed.json` and `fixtures.json`: deterministic dataset and actual Mongo indexes.
- Point directories: raw workload outputs plus `backend.json` snapshots and
  `application.json` effective non-sensitive configuration/image IDs. Separate
  `*-warmup/` directories are excluded from measurements.
- `comparison.json` and `comparison.md`: per-rate/mode results, median/min/max run p95,
  successful RPS, and median paired p95 change where all repetitions qualify.
- `lifecycle.log`: stack build/startup/cleanup diagnostics.

Rebuild the derived report without rerunning the stack:

```sh
python3 benchmarks/report.py /path/to/suite-output
```

The report's median run p95 is **not** a percentile over pooled requests. Successful
RPS is valid completions divided by configured measurement seconds, allowing in-flight
completion during grace. The highest qualified tested rate is a tested point, not
maximum capacity; no latency SLO is imposed. Preserve failed points and resource
context when discussing saturation. Run longer repetitions on a quiet host before
using figures in a resume/case study; the suite cannot eliminate shared-host or
load-generator contention. Commit/review the application revision first so the final
evidence identifies reproducible code. No improvement is assumed or required.

A warm-cache result describes that workload; it does not guarantee an improvement
for cold/mixed traffic. Mock output is harness evidence only. Preserve the historical
wrk results separately from this method.

The workload uses k6's [constant-arrival-rate executor](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/),
[redirect controls](https://grafana.com/docs/k6/latest/using-k6/http-requests/#follow-redirects),
and [thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/) so failed
checks affect the exit code. Results use [custom summary output](https://grafana.com/docs/k6/latest/results-output/end-of-test/custom-summary/).
