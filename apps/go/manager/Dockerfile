# Use the official Golang image to create a build artifact.
# This is based on Debian and sets the GOPATH to /go.
# https://hub.docker.com/_/golang
FROM golang:1.24.1-alpine as builder

# Create and change to the app directory.
WORKDIR /code

# Copy local code to the container image.
COPY apps/go/manager/ /code/apps/go/manager
COPY packages/go/ /code/packages/go

WORKDIR /code/apps/go/manager
# Install dependencies
RUN go mod download
RUN go build -o /code/bin/manager worker/main.go

# Use a Docker multi-stage build to create a lean production image.
FROM alpine
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /code/bin/manager /app/manager

# Run the service on container startup.
CMD ["/app/manager"]