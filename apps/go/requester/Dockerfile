# Use the official Golang image to create a build artifact.
# This is based on Debian and sets the GOPATH to /go.
# https://hub.docker.com/_/golang
FROM golang:1.24.1-alpine as builder

# Create and change to the app directory.
WORKDIR /code

# Copy local code to the container image.
COPY apps/go/requester/ /code/apps/go/requester
COPY packages/go/ /code/packages/go

WORKDIR /code/apps/go/requester
# Install dependencies
RUN go mod download
RUN go build -o /code/bin/requester worker/main.go

# Use a Docker multi-stage build to create a lean production image.
FROM alpine
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /code/bin/requester /app/requester

# Run the service on container startup.
CMD ["/app/requester"]