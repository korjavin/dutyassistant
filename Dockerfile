# Stage 1: Build environment
# Use a specific version of golang-alpine for reproducibility
FROM golang:1.23-alpine AS builder

WORKDIR /app

# --- Backend Build ---
# Copy Go module files first for better caching
COPY go.mod go.sum ./

# Copy all source code and vendor dependencies in one layer
COPY . .

# are not overwritten by a later COPY instruction.
RUN cd web && npm run build

# Add cache busting to HTML
RUN sed -i "s/BUILD_TIME/$(date +%s)/g" /app/web/index.html

# Compile the Go application to a static, CGo-free binary using vendored dependencies.
# The -w and -s flags strip debugging information, reducing the binary size.
# The -mod=vendor flag ensures we use vendored dependencies.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o /roster-bot ./cmd/roster-bot/

# Stage 2: Final production image
# Use alpine instead of scratch to include CA certificates for HTTPS
FROM alpine:latest

# Install CA certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Set the working directory for the application.
WORKDIR /app

# Copy the compiled application binary from the builder stage.
COPY --from=builder /roster-bot /roster-bot

# Copy the built frontend assets from the builder stage.
# Copy the entire web directory structure (index.html, js/, dist/, vendor/)
COPY --from=builder /app/web/index.html ./web/index.html
COPY --from=builder /app/web/js ./web/js
COPY --from=builder /app/web/dist ./web/dist
COPY --from=builder /app/web/vendor ./web/vendor

# The application will store its persistent data (e.g., SQLite database) in /app/data.
# This path will be targeted by a volume mount defined in docker-compose.yml.
# The directory will be created by the Docker daemon when mounting the volume.

# Expose the port the web server will listen on.
EXPOSE 8080

# Define the container's entrypoint to run the application.
ENTRYPOINT ["/roster-bot"]
