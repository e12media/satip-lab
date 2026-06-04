# CI Integration (Client Repos)

Use `satip-lab` as a **service container** in GitHub Actions when testing SAT>IP clients.

The `ghcr.io/e12media/satip-lab:latest` image is public and can be pulled by GitHub Actions service containers without registry credentials. The image supports `linux/amd64` and `linux/arm64`.

GitHub Actions creates `services` before job steps run, so a same-job `docker build` only works if you start the simulator yourself with `docker run` in the steps instead of using `services`.

GitHub Actions starts service containers before job steps, but the application inside the container may still be booting. Keep the readiness step in every job before the first simulator request.

## GitHub Actions service container

```yaml
jobs:
  satip-integration:
    runs-on: ubuntu-latest
    services:
      satip-lab:
        image: ghcr.io/e12media/satip-lab:latest
        ports:
          - 554:554
          - 8875:8875
        env:
          SATIP_LAB_PUBLIC_HOST: 127.0.0.1
          SATIP_LAB_SSDP_PORT: "0"   # SSDP unreliable in GHA; use manual IP
    steps:
      - uses: actions/checkout@v4

      - name: Wait for simulator
        run: |
          for i in $(seq 1 30); do
            curl -fsS http://127.0.0.1:8875/desc.xml && exit 0
            sleep 1
          done
          exit 1

      - name: Smoke M3U
        run: curl -fsS http://127.0.0.1:8875/channels.m3u | grep -q "ZDF HD"

      # - name: Run client tests
      #   run: ... your client integration tests ...
```

## Larger catalog fixture

Use the bundled 25-service DACH fixture when your client test needs channel-list scale, grouping, favorites, or TV grid behavior:

```yaml
services:
  satip-lab:
    image: ghcr.io/e12media/satip-lab:latest
    ports:
      - 554:554
      - 8875:8875
    env:
      SATIP_LAB_PUBLIC_HOST: 127.0.0.1
      SATIP_LAB_SSDP_PORT: "0"
      SATIP_LAB_CATALOG: /app/fixtures/astra-19.2e-dach.yaml
```

For project-specific fixtures, mount your YAML file into the container and point `SATIP_LAB_CATALOG` at the mounted path. The simulator validates the file before opening ports, so readiness polling will fail fast on invalid test data.

## Tuner-busy diagnostics

For startup-only coverage, run a dedicated CI job or matrix entry with `SATIP_LAB_SCENARIO=tuner_busy`:

```yaml
services:
  satip-lab:
    image: ghcr.io/e12media/satip-lab:latest
    ports:
      - 554:554
      - 8875:8875
    env:
      SATIP_LAB_PUBLIC_HOST: 127.0.0.1
      SATIP_LAB_SSDP_PORT: "0"
      SATIP_LAB_SCENARIO: tuner_busy
```

The same behavior is also available at runtime for repeatable diagnostics inside a normal simulator job:

```bash
curl -fsS -X POST "$SATIP_TEST_HTTP_URL/api/scenario" \
  -H 'Content-Type: application/json' \
  -d '{"name":"tuner_busy"}'
```

Both forms make valid RTSP `SETUP` requests return `503 Service Unavailable` with `Reason: tuner busy` before any tuner or session is allocated.

## Go client recipe

```yaml
jobs:
  satip-go:
    runs-on: ubuntu-latest
    services:
      satip-lab:
        image: ghcr.io/e12media/satip-lab:latest
        ports:
          - 554:554
          - 8875:8875
        env:
          SATIP_LAB_PUBLIC_HOST: 127.0.0.1
          SATIP_LAB_SSDP_PORT: "0"
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
      - name: Wait for simulator
        run: |
          for i in $(seq 1 30); do
            curl -fsS http://127.0.0.1:8875/api/schema && exit 0
            sleep 1
          done
          exit 1
      - name: Run client tests
        run: go test ./...   # adapt this command to your client integration test package or tags
        env:
          SATIP_TEST_HTTP_URL: http://127.0.0.1:8875
          SATIP_TEST_RTSP_URL: rtsp://127.0.0.1:554/
```

## Node.js client recipe

```yaml
jobs:
  satip-node:
    runs-on: ubuntu-latest
    services:
      satip-lab:
        image: ghcr.io/e12media/satip-lab:latest
        ports:
          - 554:554
          - 8875:8875
        env:
          SATIP_LAB_PUBLIC_HOST: 127.0.0.1
          SATIP_LAB_SSDP_PORT: "0"
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
      - run: npm ci
      - name: Wait for simulator
        run: |
          for i in $(seq 1 30); do
            curl -fsS http://127.0.0.1:8875/channels.m3u | grep -q 'rtsp://' && exit 0
            sleep 1
          done
          exit 1
      - name: Run client tests
        run: npm test   # adapt this to your project's SAT>IP integration test script
        env:
          SATIP_TEST_HTTP_URL: http://127.0.0.1:8875
          SATIP_TEST_RTSP_URL: rtsp://127.0.0.1:554/
```

## Python client recipe

```yaml
jobs:
  satip-python:
    runs-on: ubuntu-latest
    services:
      satip-lab:
        image: ghcr.io/e12media/satip-lab:latest
        ports:
          - 554:554
          - 8875:8875
        env:
          SATIP_LAB_PUBLIC_HOST: 127.0.0.1
          SATIP_LAB_SSDP_PORT: "0"
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: "3.13"
      - run: python -m pip install -r requirements-dev.txt   # adapt to your project
      - name: Wait for simulator
        run: |
          for i in $(seq 1 30); do
            curl -fsS http://127.0.0.1:8875/desc.xml | grep -q 'SatIPServer' && exit 0
            sleep 1
          done
          exit 1
      - name: Run client tests
        run: pytest -m satip   # adapt this marker/command to your test suite
        env:
          SATIP_TEST_HTTP_URL: http://127.0.0.1:8875
          SATIP_TEST_RTSP_URL: rtsp://127.0.0.1:554/
```

## Docker Compose client recipe

Use this when the client itself runs in a container and should reach `satip-lab` by service name instead of host ports.

```yaml
services:
  satip-lab:
    image: ghcr.io/e12media/satip-lab:latest
    environment:
      SATIP_LAB_PUBLIC_HOST: satip-lab
      SATIP_LAB_PROFILE: generic-satip-1.2
      SATIP_LAB_SSDP_PORT: "0"

  client-tests:
    build: .
    depends_on:
      - satip-lab
    environment:
      SATIP_TEST_HTTP_URL: http://satip-lab:8875
      SATIP_TEST_RTSP_URL: rtsp://satip-lab:554/
    command: >
      sh -c 'until curl -fsS http://satip-lab:8875/desc.xml >/dev/null;
      do sleep 1; done;
      ./run-client-tests'
```

```bash
docker compose run --rm client-tests
```

Replace `./run-client-tests` with your client test command. If your client test image does not include `curl`, add an equivalent wait helper or make the test entrypoint poll `SATIP_TEST_HTTP_URL` before opening RTSP sessions.

## Build image in CI (this repo)

See `.github/workflows/ci.yml` — builds the image, curls endpoints after `docker run`, and verifies RTSP/RTP packet delivery.

## Local compose

```bash
docker compose up --build
curl http://127.0.0.1:8875/desc.xml
```

## Docker Desktop

Set `SATIP_LAB_PUBLIC_HOST=host.docker.internal` so URLs inside M3U work from the host OS.

If you publish different host ports, set advertised ports too:

```bash
SATIP_LAB_PUBLIC_HTTP_PORT=18875 \
SATIP_LAB_PUBLIC_RTSP_PORT=1554 \
docker compose up --build
```

RTP over Docker port forwarding depends on how the client and Docker VM route UDP. For CI, prefer running the simulator and client in the same Docker network, or use host networking on Linux runners.

This repository also ships a small source-level smoke probe for maintainers:

```bash
go run ./cmd/satip-lab-smoke --host 127.0.0.1 --rtsp-port 554
```

Use `--rtp-destination <host-ip-from-container>` when the simulator runs behind Docker NAT and cannot infer the host UDP address from the RTSP TCP connection.

## Typical client test gates

| Gate | satip-lab helps |
|------|-----------------|
| Discovery | Partial (SSDP may need manual IP in CI) |
| M3U / channels | Yes |
| RTSP session | Yes |
| Playback | Yes (looped TS; player-dependent) |
| Production hardware sign-off | **No** — real hardware required |
