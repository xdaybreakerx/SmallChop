# Operating SmallChop

The Compose application has two HTTP listeners. Caddy forwards application traffic to port 8080. Metrics use port 9090, with no published Compose port or Caddy route. A collector on the backend network can scrape `http://app:9090/metrics`; an ordinary public `/metrics` request returns 404.

## Health and dependency contract

| Endpoint | Meaning | Dependency work |
| --- | --- | --- |
| `GET /livez` | 200 while the process can serve HTTP | None |
| `GET /readyz` | 200 when required MongoDB is reachable; otherwise 503 | One Mongo ping bounded by the smaller of `MONGO_TIMEOUT`/`REQUEST_TIMEOUT` and the caller's context |
| `GET :9090/metrics` | Prometheus text exposition | None |

Health endpoints also accept HEAD and return `Cache-Control: no-store`. Other methods return 405. Redis is optional: its failure does not make readiness false. MongoDB is required for creation and cold redirects. A previously cached redirect can succeed even when aggregate readiness is 503. Local Compose checks liveness; Docker's health status does not claim database readiness. MongoDB is still required at app startup. Avoid restarting a healthy app solely because a dependency is unavailable.

With the local stack running:

```sh
curl -i http://127.0.0.1:8080/livez
curl -i http://127.0.0.1:8080/readyz
scripts/local.sh metrics
scripts/local.sh logs
```

Use your configured `LOCAL_HTTP_PORT` if it differs from 8080. The metrics command scrapes from inside the app container in the selected local project.

## Metric semantics

| Metric | Labels | Meaning |
| --- | --- | --- |
| `smallchop_http_requests_total` | `route`, `method`, `status_class` | Completed application requests, including limiter rejections and health checks |
| `smallchop_http_request_duration_seconds` | Same | Histogram covering application handler/limiter/dependency time; excludes network transit and completion-log emission |
| `smallchop_cache_reads_total` | `outcome`: hit/miss/error | Actual cache lookups; an empty cache miss is not a dependency error |
| `smallchop_cache_fills_total` | `outcome`: success/error | Fill attempts after successful authoritative reads |
| `smallchop_dependency_failures_total` | `dependency`, `operation` | Failed app operations: Redis get/set, Mongo find/save/ping; absent mappings are excluded |

Cache-disabled mode performs no cache lookups or fills. Mongo save represents the handler's save operation, which may involve several driver commands. These are application counters, not driver retries, server command counts, or per-link access counts. A failed cache get followed by a failed fill produces two distinct failed operations. Caller cancellation can also fail a dependency operation; a counter alone does not prove a server outage.

All metrics reset when the application process restarts. Use counter rates/increases rather than comparing values across restarts. Route labels use `/r/{code}`, `/shorten`, `/`, `/livez`, `/readyz` or `unmatched`. Unknown methods collapse to `other`; status labels are classes such as `3xx`. No URL, short code, IP, request ID or error text is a metric label. Scrapes are not counted as application requests.

Example queries for a Prometheus collector scraping the metrics listener:

```promql
# Redirect cache hit fraction; undefined when no cache reads occur.
sum(rate(smallchop_cache_reads_total{outcome="hit"}[5m]))
/ sum(rate(smallchop_cache_reads_total[5m]))

# Redis operation failures, separately from successful HTTP fallback.
sum by (operation) (rate(smallchop_dependency_failures_total{dependency="redis"}[5m]))

# HTTP server-error fraction for application routes.
sum(rate(smallchop_http_requests_total{route=~"/|/shorten|/r/\\{code\\}",status_class="5xx"}[5m]))
/ sum(rate(smallchop_http_requests_total{route=~"/|/shorten|/r/\\{code\\}"}[5m]))

# Estimated redirect p95 from histogram buckets, in seconds.
histogram_quantile(0.95,
  sum by (le) (rate(smallchop_http_request_duration_seconds_bucket{route="/r/{code}"}[5m])))
```

The histogram has finite buckets; its estimated p95 differs from individual request samples collected by k6. No Prometheus server or dashboard is required by this repository. Metrics listener access depends on the deployment's network boundary; it has no application authentication and should remain unpublished.

## Request logs

The app emits JSON logs to stdout. Every application request receives a server-generated `X-Request-ID`; caller-provided IDs are replaced. Completion logs contain that ID, fixed route, bounded method, final status and duration. Dependency warnings share the ID and identify only the dependency and operation. Destinations, raw paths/queries, request bodies, client addresses and raw driver errors are omitted. Request IDs support local correlation; this is not distributed tracing.

A request can return 308 while emitting Redis failure warnings because Mongo fallback succeeded. Check both HTTP outcomes and dependency counters. A 404 is an absent mapping, while unavailable required storage returns a bounded 503. For a returned request ID, filter the app JSON logs to follow its completion and dependency warnings.

## Reproducible failure and recovery check

From the repository root, with Docker Compose 2.24.4+:

```sh
python3 scripts/check-dependencies.py > /tmp/smallchop-dependency-check.json
```

The check uses example disposable credentials in temporary files, a free loopback port and fresh project-scoped storage. It refuses existing resources for `smallchop-local-dependency-check`, locks against overlapping local runs, and removes only the resources it creates. It uses subnet `172.30.82.0/29`; adjust the script's dedicated subnet if it conflicts with your environment. Real root `.env` files are not read.

It checks:

1. Healthy live/ready responses, cold miss/fill and warm-hit counters, exact redirects and response/log ID correlation.
2. Redis stopped: readiness remains 200, redirects fall back within the dependency budget, and get/fill errors increase.
3. Redis restored: a miss fills the cache and the next request hits it, with the same app container, PID and start time.
4. Mongo stopped: liveness remains 200, readiness returns 503, a warm cached redirect succeeds, and cold reads/creation return 503 with matching failure counters.
5. Redis-optional startup, bounded required-Mongo startup failure, and persisted mappings/unchanged legacy access counts after recovery.

Operational CI runs the real check and fast Go race/vet/lint checks. Benchmark CI validates both cache modes separately. Performance comparisons should use the final reviewed revision: instrumentation changes per-request work, so earlier measurements are not directly attributable to this version.
