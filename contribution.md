# Contributing to SmallChop

See the [README](README.md) for the project overview, architecture, and workflows.

## Running Locally

Use Docker with Compose v2.20+ (including `up --wait`), Git, and Python 3 for the optional smoke check. Local startup uses a standalone Compose file, its own `.env.local`, and project-scoped MongoDB storage. Caddy serves HTTP on loopback; the application, Redis, and MongoDB have no published ports.

```sh
git clone https://github.com/xdaybreakerx/SmallChop
cd SmallChop
cp deploy/env/local.env.example .env.local
scripts/local.sh up
scripts/local.sh smoke
```

Open http://127.0.0.1:8080. Change `LOCAL_HTTP_PORT` in `.env.local` if that port is occupied. The example contains disposable development credentials; leave production credentials in `.env`. MongoDB initialization uses `MONGO_DB_NAME` consistently with the application. Changing database names or credentials requires fresh local storage; it does not update an initialized database.

`scripts/local.sh up` builds the application and waits for authenticated MongoDB/Redis checks, the application page, and Caddy. These are startup checks; application dependency health and failure recovery are later work. The smoke command checks form delivery, URL creation, and exact cold/warm redirect destinations through Caddy. It does not establish browser JavaScript behavior, cache usage, or performance.

```sh
scripts/local.sh status
scripts/local.sh logs
scripts/local.sh stop  # Stop containers; keep containers and MongoDB data.
scripts/local.sh up
scripts/local.sh down  # Remove containers/network; keep MongoDB data.
scripts/local.sh up
scripts/local.sh reset --delete-local-data  # Remove this local project's containers and data.
```

The helper always selects `deploy/compose.local.yml`, `.env.local`, and the `smallchop-local` project, ignoring any automatic production override. To run an independent experiment, prefix every command with `LOCAL_PROJECT=smallchop-local-my-test` and choose a free `LOCAL_HTTP_PORT` in `.env.local`. If running projects simultaneously, also choose distinct `LOCAL_PROXY_SUBNET` ranges and matching `LOCAL_PROXY_IP`/`LOCAL_APP_PROXY_IP` addresses. Reset is restricted to project names starting with `smallchop-local`; it does not delete production volumes. Project separation follows [Docker's Compose project-name behavior](https://docs.docker.com/compose/how-tos/project-name/).

The production setup is in `deploy/docker-compose.yml`, `deploy/caddy/Caddyfile`, and `deploy/env/production.env.example`. CD is manual; image selection, test gating, rollback, and the existing MongoDB upgrade path still need to be verified before enabling automatic releases. Do not use the local reset command to migrate production storage.

### Request behavior and configuration

- Creation accepts one `url` field in an `application/x-www-form-urlencoded` POST body. The destination must be an absolute HTTP(S) URL with a hostname and without embedded credentials. URLs are limited to 2,048 bytes; form bodies to 16 KiB. Invalid input returns 400; oversized form bodies return 413; unsupported methods return 405.
- New destinations retain their path escaping, query encoding/order, and fragments. HTTP(S) localhost/private destinations are allowed; the application redirects browsers without fetching destinations. Validation does not assess destination reputation. Previously discarded fragments cannot be recovered from existing records.
- Public codes represent positive signed 64-bit IDs using the existing alphabet. Empty, zero, noncanonical aliases (such as `bc` for `c`), invalid characters, and overflow return 400. Existing encoder-generated positive-ID codes retain their meaning; absent mappings return 404. Dependency failure classification and duplicate fallback are still pending reliability work.
- `PUBLIC_BASE_URL` is a required HTTP(S) origin at startup, without credentials, query, fragment, or a path prefix. `deploy/env/production.env.example` uses the placeholder `https://short.example`; choose an actual origin when restoring public hosting. Local Compose sets it from `LOCAL_HTTP_PORT`. Request Host/forwarded-host values do not determine generated links.
- Defaults are 2 requests/second with burst 4 for creation and 10 requests/second with burst 20 for redirects, independently per client IP. Override with `CREATE_RATE_PER_SECOND`, `CREATE_RATE_BURST`, `REDIRECT_RATE_PER_SECOND`, and `REDIRECT_RATE_BURST`. These limits are policy choices, not capacity measurements. Limits are per app instance; clients behind the same NAT share an IP bucket. Exceeding a policy returns 429.
- Caddy is the public entry point; the Go app has no published host port. A dedicated Compose proxy network assigns Caddy and the app distinct IPs, keeping MongoDB/Redis on the backend network. Compose sets `TRUSTED_PROXY_IPS` to Caddy's exact IP. Caddy overwrites `X-Forwarded-For` with its immediate client's address; the app accepts that single address only from a trusted peer. Other callers are identified by their connection IP. Missing/malformed forwarding from a trusted peer returns 400. Outside Compose, leave `TRUSTED_PROXY_IPS` empty for direct access or provide exact trusted proxy IPs, never broad private ranges.

Production defaults use `PROXY_SUBNET=172.30.81.0/29`, `PROXY_IP=172.30.81.2`, and `APP_PROXY_IP=172.30.81.3`; local defaults use the corresponding `LOCAL_*` values under `172.30.80.0/29`. Change the subnet and both addresses together if they overlap an existing Docker/VPN network. Trust applies to this single-ingress topology; a CDN or extra proxy requires an explicit policy change. The network/address configuration follows [Docker's static-IP requirements](https://docs.docker.com/reference/compose-file/services/#ipv4_address), and the header policy uses [Caddy's upstream header controls](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy#headers).

### Troubleshooting

-   Ports Already in Use:
    -   Change `LOCAL_HTTP_PORT` in `.env.local` and run `scripts/local.sh up` again.
-   Docker Network Subnet Overlaps:
    -   Choose an unused `LOCAL_PROXY_SUBNET` and matching distinct `LOCAL_PROXY_IP` and `LOCAL_APP_PROXY_IP` addresses in `.env.local`.
-   Environment Variables Not Loaded:
    -   Copy `deploy/env/local.env.example` to `.env.local` and check the required credentials and database name. The helper does not use the production `.env`.
-   MongoDB Authentication Fails After Changing Credentials:
    -   Existing storage keeps its original users. Restore the original local credentials, or explicitly delete disposable local data with `scripts/local.sh reset --delete-local-data` before starting again.
-   Permission Issues:
    -   If you encounter permission issues with volumes, adjust the permissions or run Docker with appropriate privileges.

## Development checks and dependency baseline

Use Go 1.27.1 and golangci-lint 2.14.0. CI reads the toolchain from `go.mod`; the Docker builder uses the same Go release and downloads locked modules without updating them. Run the following checks from the repository root:

```sh
golangci-lint run
go vet ./...
go test -race -coverprofile=/tmp/smallchop-coverage.out ./...
go tool cover -func=/tmp/smallchop-coverage.out
go test ./internal/utils -run '^$' -fuzz '^FuzzEncodeDecode$' -fuzztime=5s
go test ./internal/utils -run '^$' -fuzz '^FuzzDecode$' -fuzztime=5s
```

The selected dependency baseline, recorded on 5 October 2026:

- Go Redis client v9.22.0, MongoDB driver v2.9.1, miniredis v2.39.0, and `x/time` v0.16.0.
- HTMX 4.0.0 and Tailwind browser 4.3.3; the existing HTMX error-response swap behavior is retained.
- Redis 8.10.2, MongoDB 9.0.2, Caddy 2.11.6, and Alpine 3.24.2 for the application runtime.
- GitHub Actions are pinned to release commits.

MongoDB 9.0.2 was selected for fresh containers. This baseline does not migrate existing database volumes; changing server major versions requires a supported upgrade path.

## Deployment configuration

Production configuration lives under `deploy/`. Real `.env`, `.env.local`, and `.env.github-actions` files remain ignored at the repository root; copy examples from `deploy/env/` when configuring an environment. Local reference/review documents remain ignored.

Compose paths are anchored to the repository root. The local helper selects them automatically. For a production configuration check, first configure root `.env`, then run from the repository root:

```sh
docker compose --project-directory . --env-file .env -f deploy/docker-compose.yml config --quiet
```

Use these explicit arguments for direct production Compose commands. Bare `docker compose` does not select the relocated configuration. `--project-directory .` preserves root-relative environment/build paths and the existing default project name; choose any intended override with another `-f` argument. See [Docker's Compose path and project-directory rules](https://docs.docker.com/reference/cli/docker/compose/).

The CD workflow is manual (`workflow_dispatch`), so opening or merging a PR does not trigger deployment. Before using it, resolve same-commit test gating, explicit immutable image selection, environment replacement, health/smoke checks, rollback, and any required database upgrade. The current production app service declares `build: .`; the Docker Hub image pulled by the workflow is not explicitly selected by Compose. The workflow also appends to `.env` and stops the whole stack before starting it again.

## Submit a pull request

If you'd like to contribute, please fork the repository and open a pull request to the `main` branch.
