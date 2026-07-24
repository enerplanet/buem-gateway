FROM debian:bookworm-slim

ARG DEBIAN_FRONTEND=noninteractive
ENV GO_VERSION=1.26.1

# -----------------------------
# Install system dependencies
# -----------------------------
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential wget ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# -----------------------------
# Install Go
# -----------------------------
RUN wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz -O /tmp/go.tar.gz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz
ENV GOROOT="/usr/local/go"
ENV PATH="${GOROOT}/bin:${PATH}"

# -----------------------------
# Copy and build the connector (as root)
# -----------------------------
WORKDIR /app
COPY . .
RUN go build -o bin/app ./cmd/app

# -----------------------------
# Create non-root user and fix permissions
# -----------------------------
RUN useradd -m -u 10001 appuser \
    && chown -R appuser:appuser /app
USER appuser

CMD ["./bin/app"]
