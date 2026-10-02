FROM debian:bookworm-slim AS build

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
# Set by .github/workflows/docker-publish.yml; the defaults match internal/version's own.
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
RUN go build -ldflags "\
    -X github.com/enerplanet/buem-gateway/internal/version.Version=${VERSION} \
    -X github.com/enerplanet/buem-gateway/internal/version.Commit=${COMMIT} \
    -X github.com/enerplanet/buem-gateway/internal/version.Date=${DATE}" \
    -o bin/buem-gateway ./cmd/buem-gateway

# -----------------------------
# Runtime image: the binary only
# -----------------------------
FROM debian:bookworm-slim

# wget serves the compose healthchecks.
RUN apt-get update && apt-get install -y --no-install-recommends wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /app/bin/buem-gateway ./bin/buem-gateway

RUN useradd -m -u 10001 appuser \
    && chown -R appuser:appuser /app
USER appuser

CMD ["./bin/buem-gateway"]
