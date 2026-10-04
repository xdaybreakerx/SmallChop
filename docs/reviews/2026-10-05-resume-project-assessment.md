**SmallChop repository and resume assessment — 5 October 2026**

SmallChop is worth keeping as a resume project for backend, platform, and operations roles. Its strongest next step is a small, demonstrably reliable Go service: correct request handling, visible dependency behavior, repeatable tests, and performance results another engineer can reproduce. The current architecture is sufficient for that. Budget one focused weekend, with a hard stop at 12 hours.

For backend roles, the useful evidence is HTTP behavior, cache semantics, database invariants, cancellation, and tests. For platform/ops roles, it is dependency failure handling, instrumentation, reproducible startup, and controlled delivery. The current repository demonstrates the technologies and basic integration; its operational claims are stronger than the available verification.

**Prerequisite completed — Dependency refresh on `chore/update-go-and-dependencies`.** The module, Docker builder, and CI now use [Go 1.27.1](https://go.dev/dl/?mode=json), the current stable release checked on 5 October 2026. Go dependencies and their application/test dependency graph were refreshed; the Redis client moved to v9.22.0, MongoDB driver to v2.9.1, miniredis to v2.39.0, and `x/time` to v0.16.0. MongoDB tests were migrated from the removed v1 `mtest` package to the v2 driver mock. Three unchecked response writes were handled so the current linter passes.

CI uses golangci-lint 2.14.0 and verified release commits for checkout, Go setup, lint, and SSH actions. Docker downloads locked modules and builds with `-mod=readonly`; `.env*` files are excluded from the build context. Runtime images are pinned to Alpine 3.24.2, Redis 8.10.2, MongoDB 9.0.2, and Caddy 2.11.6. Caddy's 2.11.7 source release has no published Alpine image yet, so 2.11.6 was the latest published Alpine image verified. Browser dependencies are HTMX 4.0.0 and Tailwind browser 4.3.3; an explicit HTMX setting preserves the existing error-response swap behavior.

Verification passed: race-enabled Go tests, `go vet`, module checksum verification, server build, golangci-lint (zero issues), Docker image build, and an isolated fresh-container smoke test. The browser submitted a URL and rendered the result correctly; cold and warm requests through Caddy both returned 308 with the expected destination; Redis populated the mapping with its TTL; MongoDB persisted the mapping and both access-count increments. Existing error-response behavior and rendered styling were checked in the browser. This was a smoke test, not a performance measurement or production deployment.

`govulncheck` found no affected imported packages or reachable vulnerable code. It reported one module-only advisory, GO-2026-5932, for the unused `golang.org/x/crypto/openpgp` package, with no fixed release. Verification used a disposable Compose project and fresh MongoDB storage; existing database volumes were not migrated. The README records the selected versions and the need for a supported upgrade path for existing MongoDB data. Original WIP and the old stash were preserved. The dependency refresh does not complete the reliability or delivery work below; the original source findings remain useful, except for the toolchain/version mismatches and floating dependency references addressed here.

**Execution order.** Work through priorities 0–4 for the weekend. Priorities 5–7 are follow-up work; priority 5 becomes a prerequisite if you intend to restore automatic production deployment. Each item includes its supporting findings, so the order below is the implementation order rather than a separate severity ranking.

**Merge preparation.** CD was changed to manual `workflow_dispatch` before opening the dependency-refresh PR, with the existing workflow temporarily disabled during PR creation. Opening and merging PRs no longer triggers production deployment. The database migration, same-commit test gate, image selection, and rollback work remain outstanding; do not manually deploy this baseline against existing MongoDB storage until its upgrade path is verified. The commented Redis-free handler and `docs/raw/` remain local WIP and are excluded from the dependency-refresh commit.

| Order | Work | Why it comes here | Scope |
| --- | --- | --- | --- |
| 0 | Resolve local WIP decisions, establish a disposable setup, and contain automatic deployment | Gives subsequent tests a repeatable environment and prevents experimental PRs reaching production | Weekend prerequisite |
| 1 | Test real handlers and correct request handling | Establishes trustworthy behavior before measuring reliability or performance | Weekend core |
| 2 | Bound dependency failures and define cache/counter behavior | Makes the redirect contract predictable during failures | Weekend core |
| 3 | Add minimal health/metrics and demonstrate failure recovery | Produces the strongest new platform/ops evidence | Weekend core |
| 4 | Publish the runbook and accurate resume wording; benchmark if time remains | Turns completed engineering work into inspectable resume evidence | Documentation is core; benchmark is the first extension |
| 5 | Finish test-gated immutable delivery and rehearse rollback | Strengthens the deployment claim once application behavior is verified | Next platform session; required before restoring automatic production releases |
| 6 | Enforce concurrent-create deduplication, or perform a backup/restore drill | Adds deeper database correctness or recovery evidence | Choose by target role |
| 7 | Add expiring links or a small JSON API | Adds product scope after the reliability baseline is credible | Optional |

**Weekend allocation.** Budget eight hours for a complete reliability demonstration, with up to four additional hours for overruns and measurement. These are timeboxes, not guarantees; setup work in the old environment may consume the extension.

| Timebox | Priority | Target result |
| --- | --- | --- |
| 0–1 hour | 0 | WIP decisions recorded, disposable local setup working, automatic deployment contained. |
| 1–3 hours | 1 | Real handler tests and the highest-impact request fixes, including proxy identity. |
| 3–5 hours | 2 | One cache fallback path, deliberate error statuses, bounded dependency work, and an explicit counter decision. |
| 5–7 hours | 3 | Minimal health/metrics and a recorded Redis outage/recovery demonstration. |
| 7–8 hours | 4 | A repeatable runbook and wording that reflects verified behavior. |
| Optional 8–10 hours | 1–4 | Finish core overruns first; otherwise capture a seeded benchmark. |
| Optional 10–12 hours | 1–4 | Finish benchmark verification or add a Mongo outage scenario and remaining boundary tests. |

Stop at 12 hours. If the core runs long, defer benchmarking and keep the throughput claim qualified or removed until it is reproduced. Do not move unresolved correctness fixes behind a new feature. A complete recovery demonstration is a sufficient weekend deliverable. For a delivery-focused application, use the extension and a later session for priority 5 rather than trying to fit every item into one weekend.

Only narrow structural changes should be necessary: dependency test seams, one owner for cache fallback, configuration values, and small middleware/health handlers. Keep Go, MongoDB, Redis, Caddy, HTMX, and Compose. Kubernetes, a service split, Kafka, multi-region replication, a database replacement, and an authentication system offer poor returns within this budget.

**Priority 0 — Establish a clean starting point and contain deployment.** The original source assessment found a completed MVP with local leftovers. At that check, all 12 GitHub PRs were merged, there are no open PRs or issues, and the README checklist is complete. The current algorithm branch was merged through [PR #12](https://github.com/xdaybreakerx/SmallChop/pull/12).

Before starting new implementation, decide what to retain from the following. These are proposed housekeeping actions; this assessment did not apply, discard, commit, or reset them.

- Uncommitted comments in `internal/handlers/handlers.go` contain a Redis-free benchmark variant. Preserve it as a reference or replace it with a repeatable benchmark later; it currently changes no runtime behavior.
- `stash@{0}`, dated 26 October 2024, changes Compose environment loading and DockerHub username interpolation in CD. Environment loading is already reflected in current code; the interpolation change is not. Review that change selectively because the stash predates later deployment work.
- The original feature branch has one unpushed README hyperlink correction. Local `main` was one commit behind remote `main`; the original committed application content matched remote `main`. The dependency-update branch was subsequently created from that checkout.
- `docs/raw/` contains untracked diagram sources/exports; `docs/reviews/` contains this assessment. Decide which documentation belongs in version control.

Timebox these decisions. Branch tidying should not consume the weekend. Restrict automatic deployment while the application and workflow are being validated; complete priority 5 before restoring automatic production releases.

| Current evidence | Required outcome |
| --- | --- |
| CD runs on PRs and pushes, independently of CI. It publishes `latest`, while Compose declares only `build: .` for the app ([CD](../../.github/workflows/go-cd.yml), lines 3–9, 31, 53–61; [Compose](../../docker-compose.yml), line 16). | Eligible PR runs can reach deployment; fork secret restrictions do not make this a release gate. Pulling the registry image does not tell this Compose service to run it. Restrict automatic deployment now; complete the same-commit test gate and explicit image selection in priority 5. |
| Mongo init hardcodes `url_shortener` but `.env.example` offers an arbitrary DB name; local override is ignored and contains only comments; Caddy is enabled in the base Compose file. | A new reviewer cannot rely on the advertised local setup. Supply a tested local configuration and a consistent database name. Document stopping containers separately from deleting their data. |

Compose startup ordering alone does not wait for dependencies to become healthy; use explicit health conditions where appropriate. See [Docker startup ordering](https://docs.docker.com/compose/how-tos/startup-order/).

**Priority 1 — Make request behavior and tests trustworthy.** Use the real handlers, retain small mocks at dependency boundaries, and fix the demonstrated request defects alongside regression tests. Cover the public behavior rather than reproducing implementation details.

| Current evidence | Required outcome |
| --- | --- |
| Handler tests define replacement HTTP functions instead of invoking `RootHandler`, `ShortenURLHandler`, or `RedirectHandler` ([handler tests](../../internal/handlers/handlers_test.go), lines 43, 84, 155). Actual handler coverage is 0%. | Green tests do not establish the application contract. Test the real handlers with the existing Mongo mock and miniredis tools; add small dependency interfaces only where they help specific tests. |
| Rate limiting keys on `RemoteAddr`, with 2 requests/second and burst 4, on both creation and redirect routes ([limiter](../../internal/middleware/limit.go), lines 44–52; [routes](../../internal/routes/routes.go), lines 12–13). | With this Caddy topology, users share the proxy's limiter bucket. An isolated probe produced `[204,204,204,204,429]` when the fifth request was a new client behind the same proxy. Establish a trusted proxy boundary and distinct creation/redirect policies. |
| URL handling accepts `https:///path`, drops `#installation` from a valid destination, and reports an unsupported scheme as HTTP 500. These were reproduced ([utility](../../internal/utils/shorten.go), lines 68–93; [handler](../../internal/handlers/handlers.go), lines 58–68). | Reject missing hosts, preserve valid URL semantics, distinguish validation errors from server failures, and bound request bodies. Configure the public base URL; output currently hardcodes `http://smallchop.net`. |
| Tests cover the retired timestamp short-code generator, not the production encoder, decoder, or sanitizer ([utility tests](../../internal/utils/shorten_test.go)). | Add round-trip/boundary tests and a fuzz target. A probe found `Decode("c") == Decode("bc") == 1`; 20 `9` characters overflow to a negative value other than the handler's `-1` sentinel. Reject overflow and decide whether codes must be canonical. |

Caddy supplies forwarded client headers, but the Go application must trust them only from the intended proxy; blindly accepting a caller's `X-Forwarded-For` would introduce bypasses. The published `8080:8080` mapping also needs an intentional production/local split. See [Caddy header behavior](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy#defaults).

Add `go test -race ./...` to CI and publish coverage as diagnostic evidence. Exercise `Encode`/`Decode` round trips, integer boundaries, and malformed strings with tests/fuzzing. Avoid chasing a coverage percentage: the useful milestone is that production paths and meaningful failure contracts are exercised. The current limiter test also reuses one recorder while expecting 12 allowed requests despite burst 4, masking those intermediate statuses; reset it for each request and assert the actual policy.

Completion evidence: real create/redirect methods are exercised; two proxied clients have independent allowances; malformed inputs return deliberate 4xx responses; valid destinations retain their query and fragment semantics. Add decoder boundaries before treating arbitrary public codes as fully validated. If fixes overrun the initial timebox, use the weekend contingency before adding a benchmark.

**Priority 2 — Make dependency failure behavior deliberate.** Resolve duplicate fallback, misleading errors, and unbounded application work before claiming graceful degradation or cache-independent performance.

| Current evidence | Required outcome |
| --- | --- |
| Both the cache repository and handler perform fallback lookups; all final Mongo lookup errors become 404 ([cache](../../internal/repository/redis_repo.go), lines 42–67; [handler](../../internal/handlers/handlers.go), lines 93–105). | A missing document caused two `find` commands in the probe; an injected database command error returned 404. Give fallback one owner, return 404 only for absent records, and use 503 for dependency unavailability. Cache population failure should not discard an already retrieved URL. |
| Every successful redirect invokes synchronous `IncrementAccessCount`, including cache hits ([handler](../../internal/handlers/handlers.go), lines 110–116). | Redis saves a database read, but the request still waits for a database write. A probe recorded a Mongo `update` on a warm hit. Measure this path before describing cache speedups or outage independence. |
| No explicit HTTP timeout fields, application dependency deadlines, health routes, metrics, or Compose health checks; startup exits on unavailable Redis ([main](../../cmd/server/main.go), lines 19–55). The ID counter uses `context.TODO()` ([Mongo repository](../../internal/repository/mongo_db_repo.go), line 172). | Existing driver defaults provide some limits, but the application has no explicit latency budget. Add bounded dependency calls and health semantics. Decide whether an optional cache may prevent startup. Preserve request cancellation through ID allocation. |

For the synchronous access counter, make an explicit product decision. It currently has no exposed analytics feature. The cheapest path to cache-independent redirects is to retire that unused per-link write and add aggregate operational counters, documenting that these are different measurements. If per-link analytics must remain, retain and measure their cost for this weekend; a reliable background worker, queue, retries, batching, and shutdown flushing are additional scope. Spawning an unbounded goroutine per redirect is not an adequate delivery mechanism.

Completion evidence: missing records and unavailable dependencies produce different statuses; database lookup is not repeated unnecessarily; cache write failure does not discard a valid destination; dependency calls finish within documented budgets. Pass request cancellation through ID allocation. Implement the health and metrics surfaces described in priority 3 once these semantics are defined.

**Priority 3 — Add the smallest useful operational feature and recovery demonstration.** Add `/livez`, `/readyz`, and request/cache instrumentation, then use them in an isolated Redis failure/recovery scenario. This is the main new feature for platform and ops relevance. Broader dashboards and extra failure scenarios can follow later.

Keep instrumentation small: requests by route template/method/status class, a request duration histogram, cache hit/miss/error counts, and dependency failure counts. Do not label metrics with full URLs, short codes, or IPs. Log request IDs, outcomes, and durations while avoiding the current logging of full destination URLs, which can contain tokens. A `/metrics` endpoint with example queries is sufficient for the first pass; a dashboard is optional. These choices follow [Prometheus instrumentation guidance](https://prometheus.io/docs/practices/instrumentation/).

For health checks, `/livez` should report process health without querying databases. `/readyz` should reflect the declared service contract with bounded probes: MongoDB is required for creation and cold reads; Redis can be optional if fallback is supported. A cached redirect may remain possible while aggregate readiness is false. Document that distinction, and avoid restarting healthy processes simply because a dependency is unavailable.

Completion evidence: stop Redis in the disposable environment, observe bounded MongoDB fallback and cache errors, restore Redis, and show caching recovering without restarting the Go service. Save commands and observations. Add Mongo failure and restart-persistence scenarios when time permits; the concurrent-create case below belongs to priority 6.

**Validation for priorities 1–3.** Use fast handler/repository tests for error contracts, and a small real Compose integration suite for dependency behavior. Existing Mongo mocks and miniredis are useful, but they do not prove database index enforcement, startup authentication, persistence, or network failure behavior.

| Scenario | Intended result to verify |
| --- | --- |
| Create a valid URL, then redirect | Returned code resolves to the exact expected `Location`; query and fragment semantics survive. |
| Warm cache, healthy dependencies | Redirect succeeds; metrics record a cache hit. If the counter is retired, no Mongo operation occurs. |
| Redis unavailable during runtime, Mongo healthy | Redirect uses MongoDB within a deadline; cache errors rise; recovery resumes cache use automatically. |
| Mongo unavailable, cached destination | Behavior matches the access-counter decision; do not claim database independence until proved. |
| Mongo unavailable, uncached destination or creation | Bounded 503 response; dependency error visible; never a misleading 404. |
| Valid but unknown code | 404 with one authoritative lookup, not two. |
| Invalid input, invalid code, overflow, unsupported method | Explicit 4xx response; no unnecessary database calls. |
| Two clients through Caddy; forged forwarded header from an untrusted peer | Independent client limits; direct spoofing cannot select another allowance. |
| Restart with retained Mongo volume | Existing code still resolves. |
| Concurrent identical creates, if deduplication is implemented | One mapping under the chosen uniqueness contract, checked against real MongoDB. |

**Priority 4 — Turn completed work into resume evidence.** First write the short runbook and correct the architecture/performance wording. Then spend remaining weekend time on benchmarking. Documentation should reflect completed behavior even if benchmarking is deferred.

**Resume positioning.** Describe the architecture as a containerized Go service with Redis, MongoDB, and Caddy. There is one application service; counting its database, cache and reverse proxy as separate application microservices overstates the design. Likewise, replace the README's production-readiness claims with the exact tested properties and the single-host limitation.

A supportable description today is:

> Built a containerized Go URL shortener with Redis cache-aside reads, MongoDB persistence, and Caddy HTTPS; configured GitHub Actions testing and deployment workflows.

After the proposed work is implemented and verified, a stronger direction is:

> Added request and cache metrics, bounded dependency failures, and automated integration tests validating Redis outage recovery and HTTP error behavior.

After a reproducible benchmark, add an actual result using this template:

> Sustained [measured successful redirects/second] at [measured p95 latency] on [hardware], with [error rate], using a documented [duration/workload] benchmark.

Use only the parts that are completed. The project becomes substantially more valuable when an interviewer can inspect one test, rerun one experiment, and ask why a specific reliability tradeoff was chosen.

**Benchmark extension: 2–3 hours once the baseline works.** First identify whether the old result measured `/`, `/r/{code}`, or `/shorten`. Each is a different workload. The active limiter permits only two sustained requests/second per perceived client on the latter routes, so a single-client throughput result requires an explicitly documented policy/configuration. A high total response rate can otherwise mostly represent rejection responses. This is a hypothesis to rule out, not a conclusion about the historical run.

Use a seeded set of known mappings and disable redirect following so the test stops at SmallChop. Verify redirect status and exact destination in correctness/preflight checks; count statuses throughout load. Measure warm cache, cold cache, and Redis unavailable separately, with identical access-count semantics. Reset cache state only in the disposable test environment. Declare any load-test-only rate policy and keep the ordinary policy under its own test.

Record commit, server and load-generator hardware, resource limits, image/tool versions, dataset size, concurrency, duration, warm-up, Caddy/TLS inclusion, and whether load generation shares the server host. Run a small repeated sample and retain raw output. Report successful redirects/second, p50/p95/p99 latency, 429/5xx counts, timeouts, CPU/memory, and cache hit ratio. Compare cache-on/cache-off only with the same work and configuration. The current uncommitted alternative handler also comments out counting, so using it as an A/B baseline would confound cache impact with removal of a database write.

Keeping `wrk` is sufficient; its [official documentation](https://github.com/wg/wrk) covers detailed latency output and Lua response handling. Response callbacks themselves can reduce generated load, so keep validation overhead consistent and record it. Byte transfer rate describes HTTP response traffic; it adds little evidence about database scale. Present a capacity test as capacity evidence, and sustained availability only if it was actually observed over time.

**Priority 5 — Finish controlled delivery before restoring automatic production releases.** The initial restriction in priority 0 only contains the existing problem. Complete this work to substantiate the CD claim. It is the next platform-focused session, or a prerequisite brought forward if deployment is part of the weekend's intended outcome.

**Allow 3–5 hours with working deployment access.** The existing workflow needs more than changing the trigger to substantiate reliable CD. Run tests before publishing/deploying the same commit; tag or resolve an immutable image; make production Compose use that image explicitly; pin action revisions and choose compatible toolchain/linter versions. CI currently edits formatting without failing on the resulting diff. The dependency refresh has already aligned `go.mod`, CI and the Docker builder and pinned action revisions. Complete the remaining release controls here; the historical CD host Go 1.19 setting has been replaced.

Replace `.env` atomically rather than appending it on every deployment. Replace whole-stack `compose down` with a deliberate app update, health wait, and smoke test; retain and rehearse the previous image rollback. A single replica may still have a brief interruption, so do not advertise zero downtime. Explicit Compose image selection is described in [Docker's service reference](https://docs.docker.com/reference/compose-file/services/#image). Add a dependency scan and review floating image tags before restoring a public deployment; this assessment makes no vulnerability finding about individual versions.

Completion evidence: a failed test prevents release; the running image identifies the tested commit; the smoke check verifies create/redirect behavior; rollback restores the previous image. Capture one successful release and rollback as a repeatable procedure.

**Priority 6 — Add deeper database or recovery evidence.** For backend roles, choose concurrent-create deduplication first. Allow 2–3 hours for implementation and integration verification, plus any existing-data cleanup.

| Current evidence | Required outcome |
| --- | --- |
| Deduplication is read-then-insert with no repository-declared unique index on `longURL` ([Mongo repository](../../internal/repository/mongo_db_repo.go), lines 86–112). | Concurrent creation can produce multiple codes for the same destination, and lookup lacks an explicitly provisioned index. Choose the uniqueness contract, provision an index, handle duplicate-key races, and verify against real MongoDB. A separately installed live index was not inspected. |

For concurrent deduplication, first audit existing duplicates before creating a unique index, then handle races by reading the winning mapping. The database enforces uniqueness; a preceding read alone cannot. See [MongoDB unique indexes](https://www.mongodb.com/docs/manual/core/index-unique/).

Completion evidence: concurrent submissions of the same destination produce one mapping under the documented contract, verified against real MongoDB.

For an ops-focused alternative, allow 2–3 hours for a backup and restore drill into an isolated database, assuming infrastructure access already works. Verify restored mappings resolve, record recovery time and backup age, and write the procedure. This provides recovery evidence beyond retaining a Docker volume. Choose one of these extensions before adding product scope.

**Priority 7 — Add optional product functionality.** These are valuable follow-ups once priorities 1–4 are complete, rather than prerequisites for a credible resume project.

**Expiring links: 3–5 hours.** Add `expiresAt`, enforce it in the application, and cap cache lifetime to the remaining link lifetime. [MongoDB TTL deletion](https://www.mongodb.com/docs/manual/core/index-ttl/) happens asynchronously, so enforce the expiry deadline when resolving a link. Define how a new expiry interacts with global destination deduplication. Test just-before/at/after expiry on warm and cold paths. Decide a temporary redirect/cache policy first: the current 308 is cacheable, so a client may reuse a destination without consulting the service. [HTTP 308 semantics](https://datatracker.ietf.org/doc/html/rfc9110#section-15.4.9) make this a meaningful design consideration. This feature creates a focused backend story without requiring accounts or a large UI.

A small JSON create endpoint with a documented request/response contract is a 1–2 hour alternative if machine consumers or API integration are important. Reuse the same creation logic and verify the actual endpoint. Neither feature requires accounts or a large frontend.

**Assessment evidence and limits.** The findings above are grounded in the original review below. The original assessment and reordering changed documentation only; the subsequent dependency refresh is recorded above.

**What is already useful.** The repository has a small, comprehensible Go layout, Redis cache-aside reads with a one-hour TTL, MongoDB persistence and an atomic ID counter, a multistage container build, Caddy TLS configuration, signal-driven HTTP shutdown, rate limiting, and CI/CD workflow definitions. Keeping one backend and a single-host Compose deployment is proportionate. The existing resume also has a cloud/Terraform project, so this project can contribute deeper runtime and backend evidence rather than repeating infrastructure provisioning.

**Original scope and verification, before dependency refresh.** Reviewed local branch `feature/bjf-shortening-algo` at `683dc32`, including the existing uncommitted handler comments. GitHub main was verified at `0cba38460b5607792444fe481a4ea0a69aedf809`, dated 2 November 2024. Committed local changes relative to that main are a README link correction; the application findings therefore also apply to the public main code. Existing changes and `docs/raw/` were preserved.

The [current resume](https://resume.xandersalathe.com/) emphasizes containerized microservices, approximately 18,931 requests/second, 4.88 ms average latency, and automated delivery. The repository contains no benchmark scripts, raw results, or workload description supporting those measurements. This review does not establish that the historical measurements were wrong; it establishes that their scope and reproducibility cannot currently be checked.

During the original review, executed `go test -race -coverprofile=/private/tmp/smallchop-review-coverage.out ./...` and `go vet ./...` with local Go 1.27.1 on macOS/arm64. Both passed. Coverage was **33.6% overall, 0% for production handlers, and 0% for route registration**. Temporary Go overlay probes exercised existing code without changing application files. Docker was installed but its daemon was stopped, so a real Compose boot, real database integration, deployment, and load test were not performed. No live service was load-tested. GitHub returned no runs in the queried recent-run list; historical successful deployment and present uptime remain unverified. A dependency vulnerability scan was not performed.
