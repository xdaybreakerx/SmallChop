# SmallChop

A Go based URL shortener designed to be easily scalable, and highly performant.

## Tech Stack:

-   Go
-   Redis (caching)
-   MongoDB (persistent db)
-   Caddy (Reverse Proxy + TLS)
-   HTMX

## High Level Diagram

![diagram](./docs/assets/high-level.png)

## Architecture and App Flow

![diagram](./docs/assets/routes.png)

<details>
<summary>click here</summary>

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

  -   Used to catch basic formatting, linting, and test failures before code is even committed.
  -   This can be bypassed if necessary but act as a first line of defense.

### GitHub Actions

#### **CI Pipeline**

  -   Ensures that code quality is maintained consistently across different environments and that no one bypasses quality checks.

#### **CD Pipeline**
    
The CD pipeline consists of two primary jobs:

1. Build Job: Handles code checkout, builds the Docker image, and pushes it to DockerHub.
2. Deploy Job: Connects to the production server and deploys the latest Docker image.

## App Setup

### CI/CD Workflow

1. Add relevant secrets to GitHub repository.
    - You'll need to add the following:
        - Dockerhub token, and username.
        - Deployment secrets (This example uses Digital Ocean)
        - A copy of your production .env file.

### Running Locally

#### Prerequisites

Ensure you have the following installed on your system:

-   Docker (version 20.10 or higher)
-   Docker Compose (version 1.29 or higher)
-   Git

#### Installation

1. Clone the Repo

```
git clone https://github.com/xdaybreakerx/SmallChop
cd ./SmallChop
```

2. Set up environment variables

    - Copy `.env.example` to `.env` and update the environment variables accordingly.
    - Ensure that `.env` is **not** committed to version control.

3. Running the app locally

```
docker-compose up --build
```

4. Accessing the application

    - Web Application: http://localhost:${APP_PORT} (default is http://localhost:8080)
    - Redis: Not exposed externally
    - MongoDB: Not exposed externally

5. Services overview
 <details>
 <summary>click here</summary>

app Service

-   Build Context: The current directory (contains your application’s Dockerfile)
-   Ports: Exposes port 8080 (or as defined in .env)
-   Depends On: redis, mongo
-   Environment Variables: Loaded from .env

redis Service

-   Image: redis:alpine
-   Command: Starts Redis with a password from .env
-   Environment Variables: Loaded from .env
-   Ports: Not exposed externally

mongo Service

-   Image: mongo:latest
-   Volumes:
    -   mongo-data for persistent storage
-   mongo-user-init.js for initialization
-   Environment Variables: Loaded from .env
-   Ports: Not exposed externally

caddy Service (Production Only)

-   Image: caddy:2.8.4-alpine
-   Ports:
    -   Exposes port 80 for HTTP
    -   Exposes port 443 for HTTPS
-   Volumes:
    -   caddy_data for Caddy data
    -   caddy_config for configuration
-   Caddyfile for server configuration
-   Environment Variables: Loaded from .env

Additional Notes

-   Data Persistence: Volumes are used for MongoDB and Caddy to ensure data persists between container restarts.
-   Environment Variables: Keep the .env file secure, especially in production.
-   Docker Compose Override: The docker-compose.override.yml file is used to customize the Compose configuration for local development.

    </details>

6. Cleaning Up

To stop and remove all containers, networks, and volumes created by Docker Compose:

```
docker-compose down -v
```

#### Troubleshooting

-   Ports Already in Use:
    -   Ensure that the ports defined in docker-compose.yml and .env are not being used by other applications.
-   Environment Variables Not Loaded:
    -   Double-check the .env file and ensure all necessary variables are defined.
-   Permission Issues:
    -   If you encounter permission issues with volumes, adjust the permissions or run Docker with appropriate privileges.

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
