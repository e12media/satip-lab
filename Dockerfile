# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM debian:bookworm-slim AS ts-asset
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates ffmpeg \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /build
COPY tool/generate_sample_ts.sh tool/verify_sample_ts.sh tool/
RUN chmod +x tool/generate_sample_ts.sh \
  && mkdir -p assets \
  && ./tool/generate_sample_ts.sh

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
COPY --from=ts-asset /build/assets/ assets/
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/satip-lab ./cmd/satip-lab \
  && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/satip-lab-compat-evidence ./cmd/satip-lab-compat-evidence \
  && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/satip-lab-mcp ./cmd/satip-lab-mcp \
  && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/satip-lab-smoke ./cmd/satip-lab-smoke \
  && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/satip-labctl ./cmd/satip-labctl

FROM debian:bookworm-slim
LABEL org.opencontainers.image.title="SAT>IP Lab Server" \
  org.opencontainers.image.description="Deterministic SAT>IP lab server for client development and CI" \
  org.opencontainers.image.source="https://github.com/e12media/satip-lab" \
  org.opencontainers.image.licenses="MIT"
WORKDIR /app
COPY --from=ts-asset /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/satip-lab /app/satip-lab
COPY --from=build /out/satip-lab-compat-evidence /app/satip-lab-compat-evidence
COPY --from=build /out/satip-lab-mcp /app/satip-lab-mcp
COPY --from=build /out/satip-lab-smoke /app/satip-lab-smoke
COPY --from=build /out/satip-labctl /app/satip-labctl
COPY --from=build /src/assets/ /app/assets/
COPY --from=build /src/fixtures/ /app/fixtures/
EXPOSE 554/tcp 8875/tcp 1900/udp
ENV SATIP_LAB_BIND=0.0.0.0 \
  SATIP_LAB_PUBLIC_HOST=127.0.0.1 \
  SATIP_LAB_HTTP_PORT=8875 \
  SATIP_LAB_RTSP_PORT=554 \
  SATIP_LAB_PUBLIC_HTTP_PORT=0 \
  SATIP_LAB_PUBLIC_RTSP_PORT=0 \
  SATIP_LAB_TUNERS=2 \
  SATIP_LAB_SSDP_PORT=1900 \
  SATIP_LAB_CATALOG= \
  SATIP_LAB_TOPOLOGY= \
  SATIP_LAB_TS_PATH= \
  SATIP_LAB_SAMPLE_PROFILE=h264_aac_short \
  SATIP_LAB_PROFILE=generic-satip-1.2 \
  SATIP_LAB_VENDOR_PROFILE=spec \
  SATIP_LAB_EPG_CLOCK=fixed:2026-03-29T01:30:00+01:00 \
  SATIP_LAB_SCENARIO=normal
ENTRYPOINT ["/app/satip-lab"]
