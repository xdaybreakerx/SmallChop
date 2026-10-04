# Build stage
FROM golang:1.27.1-alpine AS builder

# Install git to fetch dependencies
RUN apk add --no-cache git

# Set the working directory inside the container
WORKDIR /go/src/app

# Download the locked dependencies before copying application sources
COPY go.mod go.sum ./
RUN go mod download

# Copy the project and build without changing the dependency lockfile
COPY . .

# Build the Go app (assumes main.go is under ./cmd/server/)
RUN go build -mod=readonly -o /go/bin/app ./cmd/server/


# Final stage
FROM alpine:3.24.2

# Install CA certificates to allow HTTPS
RUN apk --no-cache add ca-certificates

# Copy the built Go binary
COPY --from=builder /go/bin/app /app

# Copy the templates folder from the build stage
COPY --from=builder /go/src/app/internal/templates /internal/templates

# Set the entry point to the Go app
ENTRYPOINT ["/app"]

# Label for metadata
LABEL Name=gochop Version=0.0.1

# Expose the port the app will run on
EXPOSE 8080
