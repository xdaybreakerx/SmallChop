# SmallChop

SmallChop is a URL shortener built with Go, crafted for scalability and high performance. Designed to handle high traffic seamlessly, SmallChop offers fast and reliable URL shortening with a simple, containerized setup that's easy to deploy.

Designed as a lightweight, containerized application, SmallChop leverages Docker and a microservice-oriented architecture to deliver quick, reliable URL shortening and redirection.
It features a caching layer with Redis for ultra-fast access to frequently requested URLs, persistent storage with MongoDB, and a reverse proxy with Caddy for secure, seamless HTTPS access.

GitHub Actions runs formatting, linting, and tests on PRs. The deployment workflow currently requires manual dispatch while release controls and the database upgrade path are being verified.

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

However, for a project of this scale and purpose, using Kubernetes and a load balancer would be overkill. Instead, SmallChop is deployed using Docker Compose on a single DigitalOcean Droplet, which provides a streamlined, cost-effective environment suitable for demonstration and portfolio purposes. This approach keeps infrastructure simple while showcasing containerized microservices architecture. It maintains the essential components—caching, persistent storage, and reverse proxy—while remaining accessible and manageable for a smaller deployment. This setup can later be adapted to a more advanced Kubernetes environment if needed, making SmallChop flexible and adaptable for future growth.

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

Ensures that code quality is maintained consistently across different environments and that no one bypasses quality checks.

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
cp .env.local.example .env.local
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

The helper always selects `compose.local.yml`, `.env.local`, and the `smallchop-local` project, ignoring any automatic production override. To run an independent experiment, prefix every command with `LOCAL_PROJECT=smallchop-local-my-test` and choose a free `LOCAL_HTTP_PORT` in `.env.local`. Reset is restricted to project names starting with `smallchop-local`; it does not delete production volumes. Project separation follows [Docker's Compose project-name behavior](https://docs.docker.com/compose/how-tos/project-name/).

The original `docker-compose.yml`, `Caddyfile`, and `.env.example` describe the production setup. CD is manual; the image selection, test gate, rollback, and existing MongoDB upgrade path still need the work recorded in [Priority 5](docs/reviews/2026-10-05-resume-project-assessment.md). Do not use the local reset command to migrate production storage.

#### Troubleshooting

-   Ports Already in Use:
    -   Change `LOCAL_HTTP_PORT` in `.env.local` and run `scripts/local.sh up` again.
-   Environment Variables Not Loaded:
    -   Copy `.env.local.example` to `.env.local` and check the required credentials and database name. The helper does not use the production `.env`.
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
