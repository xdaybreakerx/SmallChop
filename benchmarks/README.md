# Redirect benchmark harness

This suite issues a fixed offered rate of GET requests to `/r/{code}` through a
local HTTP origin. Each response must be **308 with the exact expected Location**;
redirects are never followed. The workload cycles uniformly through known mappings.

## Requirements and quick run

Use Python 3 and **k6 2.3.0** (pinned in [.k6-version](.k6-version)). Download the
appropriate binary from the [official release](https://github.com/grafana/k6/releases/tag/v2.3.0)
and verify its published checksum. The runner checks the version and disables usage
reporting. No cloud account or application dependency is required.

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
```

This runs the actual k6 script against a disposable Python HTTP server on a random
loopback port. It verifies exact destinations/escaping/fragments, no destination
fetches, nonzero exits for wrong Location, wrong status, 429, 503, timeouts and dropped
iterations, plus input/output protection. It does not run the application or measure
MongoDB/Redis performance. Add `--output /path/to/new-directory` to retain mock evidence.

## Mongo-only versus Mongo-plus-Redis comparison

This scaffold does not switch application backend modes or orchestrate the stack.
`--label` is annotation only. Complete these prerequisites before collecting a cache
comparison:

1. Add an explicit cache bypass that skips both read and fill. Stopping Redis includes
   failure overhead and is a different experiment.
2. Add cache-hit/miss/error and Mongo-lookup instrumentation to confirm each path.
3. Seed one reproducible dataset; fix ingress, application revision, limits, resource
   allocation and fixture order for both modes. Preserve effective configuration and
   host/container/generator resource context with the evidence.
4. Warm MongoDB in both modes and Redis in the cached mode. Run warm-up separately,
   long enough to cover every fixture, and exclude its output from measured results.
5. Alternate repeated runs at identical offered rates/durations/VU budgets. Compare
   validated latency, successful counts, errors and scheduling outcomes. Then use a
   controlled rate sweep to investigate capacity.

A warm-cache result describes that workload; it does not guarantee an improvement
for cold/mixed traffic. Mock output is harness evidence only. Preserve the historical
wrk results separately from this method.

The workload uses k6's [constant-arrival-rate executor](https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/),
[redirect controls](https://grafana.com/docs/k6/latest/using-k6/http-requests/#follow-redirects),
and [thresholds](https://grafana.com/docs/k6/latest/using-k6/thresholds/) so failed
checks affect the exit code. Results use [custom summary output](https://grafana.com/docs/k6/latest/results-output/end-of-test/custom-summary/).
