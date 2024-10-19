# Build stage
FROM golang:alpine AS builder

# Install git to fetch dependencies
RUN apk add --no-cache git

# Set the working directory inside the container
WORKDIR /go/src/app

# Copy the entire project into the container
COPY . .

# Fetch dependencies
RUN go get -d -v ./...

# Build the Go app (assumes main.go is under ./cmd/server/)
RUN go build -o /go/bin/app ./cmd/server/


# Final stage
FROM alpine:latest

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