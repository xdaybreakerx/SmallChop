# SmallChop

SmallChop is a URL shortener built with Go, crafted for scalability and high performance. Designed to handle high traffic seamlessly, SmallChop offers fast and reliable URL shortening with a simple, containerized setup that's easy to deploy.

Designed as a lightweight, containerized application, SmallChop leverages Docker and a microservice-oriented architecture to deliver quick, reliable URL shortening and redirection.
It features a caching layer with Redis for ultra-fast access to frequently requested URLs, persistent storage with MongoDB, and a reverse proxy with Caddy for secure, seamless HTTPS access.

GitHub Actions runs formatting, linting, and tests on PRs. The deployment workflow currently requires manual dispatch while release controls and the database upgrade path are being verified.

The previous public domain and hosting have expired. The current application is verified locally; restoring a public deployment is separate follow-up work.

## Tech Stack:

### Core Technologies

-   Go:
    -   Powers the URL shortening service, providing high performance and efficient concurrency for handling requests.
-   HTMX:
    -   Manages the entire front-end interactivity through server-driven templates. It allows dynamic updates by injecting values directly from server responses, simplifying the frontend and enabling a highly responsive UI.
-   Redis:
    -   Serves as a caching layer, ensuring frequently accessed URLs are retrieved quickly. This helps reduce load on the database and improves response times.
-   MongoDB:
    -   Provides persistent storage for original URLs and their shortened counterparts. MongoDB’s document-based structure allows flexible storage of URL data.
-   Caddy:
    -   Acts as the reverse proxy and TLS provider, enabling secure HTTPS access and efficient routing of requests to the backend service.

### Dependency baseline (5 October 2026)

- Go 1.27.1; CI reads `go.mod`, and the Docker builder uses the same release.
- Go Redis client v9.22.0, MongoDB driver v2.9.1, miniredis v2.39.0, and `x/time` v0.16.0.
- HTMX 4.0.0 and Tailwind browser 4.3.3, with the existing error-response swap behavior retained.
- Docker services: Redis 8.10.2, MongoDB 9.0.2, Caddy 2.11.6; application runtime: Alpine 3.24.2.
- golangci-lint 2.14.0; GitHub Actions are pinned to verified release commits.

For local Go development, use Go 1.27.1 and run `go test -race ./...` and `go vet ./...`.
The Docker build downloads the locked modules without updating them. Caddy 2.11.6 is the latest verified published Alpine image; the 2.11.7 source release has no published Alpine tag yet.
MongoDB 9.0.2 was selected for a fresh-container baseline. Existing databases need a supported server upgrade path before switching major server versions; this dependency update does not migrate existing volumes.
CD is manual (`workflow_dispatch`) so opening or merging a PR cannot restart production against the new database major version. Complete the release-gating and database upgrade work before running it.

### Repository layout

```text
cmd/server/                 Application entry point
internal/                   Go packages, package tests, and HTML templates
deploy/
  docker-compose.yml        Production Compose configuration
  compose.local.yml         Disposable local Compose configuration
  caddy/                    Production and local Caddyfiles
  mongo/                    MongoDB initialization script
  env/                      Example environment files
scripts/                    Local lifecycle and HTTP smoke helpers
docs/assets/                Published architecture diagrams
docs/bruno/                 API request collection
.github/workflows/          CI and manual CD
.husky/hooks/               Local commit checks
Dockerfile                  Application image build (repository-root context)
```

Real `.env`, `.env.local`, and `.env.github-actions` files remain ignored at the repository root. Local reference/review documents remain ignored. Copy examples from `deploy/env/` into the root when setting up an environment.

Compose paths are deliberately anchored to the repository root. The local helper handles this automatically. For future production configuration checks, run from the root after configuring `.env`:

```sh
docker compose --project-directory . --env-file .env -f deploy/docker-compose.yml config --quiet
```

Use these explicit arguments for any direct production Compose command; bare `docker compose` no longer selects the relocated configuration. Keeping `--project-directory .` preserves root-relative environment/build paths and the existing default project name rather than selecting `deploy`. Existing commands or automation must adopt the new file path; select any intended override explicitly with another `-f` argument. See [Docker's Compose path and project-directory rules](https://docs.docker.com/reference/cli/docker/compose/).

### Development & Deployment Tools

-   Docker:
    -   Containerizes each service (Go app, Redis, MongoDB, and Caddy) to ensure consistency across development, testing, and production environments. Docker Compose is used to manage multi-container setups.
-   Husky:
    -   Implements pre-commit hooks for formatting, linting, and running tests locally, helping enforce code quality standards before changes are committed.
-   gofmt:
    -   Automatically formats Go code, maintaining a consistent coding style across the project.
-   golangci-lint:
    -   Runs static analysis on Go code to catch potential errors, improve code quality, and enforce best practices.
-   go test:
    -   Used for running unit tests, ensuring that the application functions as expected and helping prevent regressions.
-   GitHub Actions:
    -   Powers CI/CD, building and pushing Docker images to Docker Hub and providing a manually triggered deployment workflow.

## High Level Diagram

The architecture diagram below illustrates SmallChop’s core components, showing how user requests are managed through a reverse proxy, caching layer, and database for high efficiency.
![diagram](./docs/assets/high-level.png)

<details>
<summary>Click here for a discussion about scaling this service.</summary>

In an enterprise environment, SmallChop would typically be deployed with Kubernetes to enable high scalability and manageability. By using Kubernetes, the application could run across multiple pods and nodes, allowing for automatic scaling in response to traffic spikes. This setup would also enable seamless updates and rollbacks through Kubernetes’ built-in deployment strategies, such as rolling updates. Additionally, a load balancer would be essential to distribute incoming traffic evenly across instances, ensuring high availability and minimizing latency. This would also allow easy integration of a more robust secret manager than what is currently implemented in this project.

For a project of this scale and purpose, the deployment design uses Docker Compose on a single DigitalOcean Droplet. The previous hosting has expired; the standalone local Compose setup is the current runnable demonstration. One Go application with a cache, database, and reverse proxy keeps the infrastructure proportionate to the project.

</details>

## Architecture and App Routes

SmallChop exposes routes for creating, retrieving, and redirecting shortened URLs, with caching mechanisms for high-frequency requests.
![diagram](./docs/assets/routes.png)

<details>
<summary>click here for a simple text diagram of the app architecture.</summary>

## Architecture

```
            +---------------------+
            |     User Requests   |
            +---------------------+
                      |
                      v
         +----------------------------+
         |      URL Shortener API     |
         |        (Go Service)        |
         +----------------------------+
                      |
                      v
  +------------------------------------------+
  |          Caching Layer (Redis)           |
  +------------------------------------------+
                      |
                      v
  +------------------------------------------+
  |       Persistent Storage (MongoDB)       |
  +------------------------------------------+
```

## MVP Architecture

```
            +---------------------+
            |     User Requests   |
            +---------------------+
                      |
                      v
         +----------------------------+
         |      URL Shortener API     |
         |        (Go Service)        |
         +----------------------------+
                      |
                      v
  +------------------------------------------+
  |            Redis as a DB                 |
  +------------------------------------------+
```

</details>

## CI / CD Pipelines

### Pre-Commit (Local)

#### Husky

Husky pre-commit hooks ensure that basic formatting, linting, and tests are enforced on all changes before they're committed, while GitHub Actions provides continuous integration and deployment to maintain code quality across environments.

### GitHub Actions

#### **CI Pipeline**

Checks formatting without accepting changes to Go sources, validates the local deployment configuration/helper syntax, runs lint and race-enabled tests, and exercises bounded encoder/decoder fuzz targets. A `go-coverage` artifact provides coverage diagnostics; there is no percentage gate. The same checks can be run locally:

```sh
go test -race -coverprofile=/tmp/smallchop-coverage.out ./...
go tool cover -func=/tmp/smallchop-coverage.out
go vet ./...
go test ./internal/utils -run '^$' -fuzz '^FuzzEncodeDecode$' -fuzztime=5s
go test ./internal/utils -run '^$' -fuzz '^FuzzDecode$' -fuzztime=5s
```

#### **CD Pipeline**

The CD pipeline consists of two primary jobs:

1. Build Job: Handles code checkout, builds the Docker image, and pushes it to DockerHub.
2. Deploy Job: Connects to the production server and deploys the latest Docker image.

## 🤝 Contributing
### Running Locally

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

#### Request behavior and configuration

- Creation accepts one `url` field in an `application/x-www-form-urlencoded` POST body. The destination must be an absolute HTTP(S) URL with a hostname and without embedded credentials. URLs are limited to 2,048 bytes; form bodies to 16 KiB. Invalid input returns 400; oversized form bodies return 413; unsupported methods return 405.
- New destinations retain their path escaping, query encoding/order, and fragments. HTTP(S) localhost/private destinations are allowed; the application redirects browsers without fetching destinations. Validation does not assess destination reputation. Previously discarded fragments cannot be recovered from existing records.
- Public codes represent positive signed 64-bit IDs using the existing alphabet. Empty, zero, noncanonical aliases (such as `bc` for `c`), invalid characters, and overflow return 400. Existing encoder-generated positive-ID codes retain their meaning; absent mappings return 404. Dependency failure classification and duplicate fallback are still pending reliability work.
- `PUBLIC_BASE_URL` is a required HTTP(S) origin at startup, without credentials, query, fragment, or a path prefix. `deploy/env/production.env.example` uses the placeholder `https://short.example`; choose an actual origin when restoring public hosting. Local Compose sets it from `LOCAL_HTTP_PORT`. Request Host/forwarded-host values do not determine generated links.
- Defaults are 2 requests/second with burst 4 for creation and 10 requests/second with burst 20 for redirects, independently per client IP. Override with `CREATE_RATE_PER_SECOND`, `CREATE_RATE_BURST`, `REDIRECT_RATE_PER_SECOND`, and `REDIRECT_RATE_BURST`. These limits are policy choices, not capacity measurements. Limits are per app instance; clients behind the same NAT share an IP bucket. Exceeding a policy returns 429.
- Caddy is the public entry point; the Go app has no published host port. A dedicated Compose proxy network assigns Caddy and the app distinct IPs, keeping MongoDB/Redis on the backend network. Compose sets `TRUSTED_PROXY_IPS` to Caddy's exact IP. Caddy overwrites `X-Forwarded-For` with its immediate client's address; the app accepts that single address only from a trusted peer. Other callers are identified by their connection IP. Missing/malformed forwarding from a trusted peer returns 400. Outside Compose, leave `TRUSTED_PROXY_IPS` empty for direct access or provide exact trusted proxy IPs, never broad private ranges.

Production defaults use `PROXY_SUBNET=172.30.81.0/29`, `PROXY_IP=172.30.81.2`, and `APP_PROXY_IP=172.30.81.3`; local defaults use the corresponding `LOCAL_*` values under `172.30.80.0/29`. Change the subnet and both addresses together if they overlap an existing Docker/VPN network. Trust applies to this single-ingress topology; a CDN or extra proxy requires an explicit policy change. The network/address configuration follows [Docker's static-IP requirements](https://docs.docker.com/reference/compose-file/services/#ipv4_address), and the header policy uses [Caddy's upstream header controls](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy#headers).

#### Troubleshooting

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

### Submit a pull request

If you'd like to contribute, please fork the repository and open a pull request to the `main` branch.
### Todo

<details>
<summary>click here for the todo list</summary>

-   [x] HTMX + Go + Redis MVP
-   [x] pre commit hooks
-   [x] testing
-   [x] ci with github actions
-   [x] rate limiter
-   [x] persistent storage
-   [x] change Redis to caching layer
-   [x] cd with github actions
-   [x] deployment
-   [x] better shortener algo

</details>

### References and further reading.

-   This project used the [tutorial from Annis Souames of Stream.io](https://getstream.io/blog/url-shortener/) for the basic HTMX + Go + Redis implementation. 
-   This project referenced this [Stack Overflow discussion](https://stackoverflow.com/questions/742013/how-do-i-create-a-url-shortener) about Bijective Functions for implementing the more complex shortening algorithm.
-   If you're curious about Caddy vs Nginx, this [article by Tyler Langlois](https://blog.tjll.net/reverse-proxy-hot-dog-eating-contest-caddy-vs-nginx/) discusses performance considerations.
-   This projects URL shortening short codes are derived from sequential integer IDs from MongoDB. As this increments, future short URLs can be predicted. In order to mitigate this, we could implement hashing or randomization however this is excessive for this projects scope. 
