# SmallChop

A Go based URL shortener

## Tech Stack:

-   Go
-   Redis (caching)
-   MongoDB (persistent db)
-   Caddy (Reverse Proxy + TLS)
-   HTMX

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

<details>
<summary>click here</summary>

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

### CI / CD

#### Pre-Commit (Local)

-   **Husky**
    -   Used to catch basic formatting, linting, and test failures before code is even committed.
    -   This can be bypassed if necessary but act as a first line of defense.

#### GitHub Actions

-   **Go-CI**
-   Ensures that code quality is maintained consistently across different environments and that no one bypasses quality checks.

### Todo

<details>
<summary>click here</summary>

-   [x] pre commit hooks
-   [x] testing
-   [x] rate limiter
-   [x] persistent storage
-   [x] caching layer
-   [x] cd with github actions
-   [x] deployment
-   [ ] better shortener algo

</details>

### Setup

#### MongoDB, Redis, and Caddy
1. Copy `.env.example` to `.env` and update the environment variables accordingly.
2. Ensure that `.env` is **not** committed to version control.

#### CI/CD Workflow 
1. Add relevant secrets to GitHub repository.
    - You'll need to add the following:
      - Dockerhub token, and username.
      - Deployment secrets (This example uses Digital Ocean)
      - A copy of your production .env file. 