# GoChop.it

A Go based URL shortener

## Tech Stack:

-   Go
-   Redis (caching)
-   MongoDB (persistent db)
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
-   [ ] better shortener algo
-   [ ] deployment and CD workflow

</details>

### Setup

#### MongoDB

1. Copy `mongo-user-init-example.js` to `mongo-user-init.js` and replace the placeholder values with your own credentials.
2. Copy `.env.example` to `.env` and update the environment variables accordingly.
3. Ensure that `.env` is **not** committed to version control.