# Stage 1: Build environment
FROM golang:1.23-alpine AS builder

# Install frontend build dependencies
RUN apk add --no-cache nodejs npm

WORKDIR /app

# Frontend build
COPY web/package.json web/package-lock.json* ./web/
RUN cd web && npm install

COPY web/ ./web/
RUN cd web && npm run build

# Backend build
COPY go.mod go.sum ./
COPY . .

# Build Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o /roster-bot ./cmd/roster-bot/

# Stage 2: Final image
FROM scratch

WORKDIR /app

# Copy binary and frontend assets
COPY --from=builder /roster-bot /roster-bot
COPY --from=builder /app/web/dist ./web/dist

EXPOSE 8080

ENTRYPOINT ["/roster-bot"]
