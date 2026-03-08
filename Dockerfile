# ── Build stage ───────────────────────────────────────────────────────────────
# Use the full Go image to compile the binary
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy dependency files first (better Docker layer caching)
COPY go.mod ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Compile to a single binary
RUN go build -o /bin/redemption-server ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
# Use a tiny image - we only need the compiled binary, not the full Go toolchain
FROM alpine:3.19

WORKDIR /app

# Copy compiled binary from build stage
COPY --from=builder /bin/redemption-server /bin/redemption-server

# Copy the default data directory (staff mapping CSV lives here)
COPY data/ ./data/

EXPOSE 8080

ENTRYPOINT ["/bin/redemption-server"]
