# Stage 1: Build environment
# Use a specific version of golang-alpine for reproducibility
FROM golang:1.23-alpine AS builder

# Install frontend build dependencies (Node.js, npm for Tailwind CSS)
RUN apk add --no-cache nodejs npm

WORKDIR /app

# --- Frontend Build ---
# First, copy only the package management files to leverage Docker's layer caching.
# This step assumes package.json and package-lock.json will exist in the /web directory.
COPY web/package.json ./web/
COPY web/package-lock.json* ./web/
RUN cd web && npm install

# Copy the rest of the frontend source code
COPY web/ /app/web/

# Build the frontend assets. This script should be defined in package.json.
RUN cd web && npm run build

# --- Backend Build ---
# Copy Go module files first for better caching
COPY go.mod go.sum ./

# Copy all source code and vendor dependencies in one layer
COPY . .

# Compile the Go application to a static, CGo-free binary using vendored dependencies.
# The -w and -s flags strip debugging information, reducing the binary size.
# The -mod=vendor flag ensures we use vendored dependencies.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o /roster-bot ./cmd/roster-bot/

# Stage 2: Final production image
# Use the 'scratch' image, which is completely empty, for a minimal attack surface.
FROM scratch

# Set the working directory for the application.
WORKDIR /app

# Copy the compiled application binary from the builder stage.
COPY --from=builder /roster-bot /roster-bot

# Copy the built frontend assets from the builder stage.
# The application will need to be configured to serve files from this directory.
COPY --from=builder /app/web/dist ./web/dist

# The application will store its persistent data (e.g., SQLite database) in /app/data.
# This path will be targeted by a volume mount defined in docker-compose.yml.
# The directory will be created by the Docker daemon when mounting the volume.

# Expose the port the web server will listen on.
EXPOSE 8080

# Define the container's entrypoint to run the application.
ENTRYPOINT ["/roster-bot"]