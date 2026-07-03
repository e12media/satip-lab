# Cross-Implementation Validation

Use this optional recipe when a SAT>IP client should be checked against
`satip-lab` and at least one independent implementation. The goal is not to make
independent servers behave like `satip-lab`; it is to expose shared assumptions
before they become client bugs.

This is a developer or client-repository recipe, not a default `satip-lab` CI
gate. Keep failures triaged as evidence:

- A client fails against every server: likely client bug or test harness bug.
- A client passes independent servers but fails `satip-lab`: likely lab gap.
- A client passes `satip-lab` but fails an independent server: either client
  compatibility gap or candidate profile behavior. Promote non-spec behavior
  only with `captured-trace` or `owned-hardware` evidence.

## Reference Implementations

| Server | Use in this recipe | Notes |
|--------|--------------------|-------|
| `satip-lab` | Deterministic baseline | Full lab API, stable DACH catalog, deterministic RTSP/RTP, XMLTV, scenarios, and RTCP APP status. |
| minisatip | Independent RTSP/session comparison | Upstream minisatip documents `-a x:y:z` simulated adapter counts. The LinuxServer image exposes `RUN_OPTS` for minisatip command-line flags. RTP delivery may require real or virtual input beyond a simulated adapter. |
| SatPI | Independent virtual-input candidate | Upstream SatPI documents virtual tuners with FILE, STREAMER, and CHILDPIPE inputs. Prefer SatPI when the validation target requires independent RTP payload delivery without tuner hardware. |

Sources for independent server capabilities:

- minisatip upstream: <https://github.com/catalinii/minisatip>
- LinuxServer minisatip image: <https://docs.linuxserver.io/images/docker-minisatip/>
- SatPI upstream: <https://github.com/Barracuda09/SATPI>

## Local Compose Fixture

Run these commands from the `satip-lab` repository root.

The fixture starts `satip-lab` and a minisatip container on distinct host ports:

| Server | HTTP | RTSP |
|--------|------|------|
| `satip-lab` | `http://127.0.0.1:18875` | `rtsp://127.0.0.1:1554/` |
| minisatip | `http://127.0.0.1:28875` | `rtsp://127.0.0.1:2554/` |

Fixture coverage is intentionally narrow:

| Area | Covered by this fixture |
|------|--------------------------|
| HTTP/device XML | Yes, for both services. |
| RTSP setup/session lifecycle | Yes, for both services. |
| RTP payload delivery | Guaranteed for `satip-lab`; independent-server RTP requires minisatip with real/virtual input, SatPI FILE/CHILDPIPE input, or another full server setup. See issue #39 for the no-hardware fixture follow-up. |
| SSDP discovery | No. Multicast and UDP 1900 conflict in a multi-server compose network. Run discovery checks one server at a time on a LAN or Linux host network. See issue #39 for the repeatable discovery fixture follow-up. |

Start both services:

```bash
docker compose -f docs/compatibility/cross-implementation-compose.yml up -d --build
```

Check HTTP readiness:

```bash
curl -fsS http://127.0.0.1:18875/desc.xml | grep -q 'SatIPServer'
curl -fsS http://127.0.0.1:28875/desc.xml | grep -qi 'sat'
```

On Docker Desktop or another NAT-backed runtime, resolve the host address that
containers can use for UDP RTP return traffic:

```bash
HOST_FROM_SATIP_LAB=$(
  docker compose -f docs/compatibility/cross-implementation-compose.yml \
    exec -T satip-lab getent hosts host.docker.internal | awk '{print $1; exit}'
)
```

Collect baseline `satip-lab` RTSP/RTP evidence:

```bash
go run ./cmd/satip-lab-smoke \
  --json \
  --profile generic-satip-1.2 \
  --host 127.0.0.1 \
  --rtsp-port 1554 \
  --rtp-destination "$HOST_FROM_SATIP_LAB" \
  > /tmp/satip-lab-smoke.json
```

If the independent server is configured with a real or virtual transport-stream
input, collect the same style of evidence:

```bash
HOST_FROM_MINISATIP=$(
  docker compose -f docs/compatibility/cross-implementation-compose.yml \
    exec -T minisatip getent hosts host.docker.internal | awk '{print $1; exit}'
)

go run ./cmd/satip-lab-smoke \
  --json \
  --profile minisatip \
  --host 127.0.0.1 \
  --rtsp-port 2554 \
  --rtp-destination "$HOST_FROM_MINISATIP" \
  > /tmp/minisatip-smoke.json
```

If minisatip accepts RTSP but times out waiting for RTP with only `-a 1:0:0`,
record that as a harness limitation, not a `satip-lab` regression. The simulated
adapter is useful for independent RTSP/session behavior, but it is not a
no-hardware RTP fixture by itself. Use SatPI FILE input, real tuner hardware, or
another full server setup when independent RTP payload comparison is required.

Stop the fixture:

```bash
docker compose -f docs/compatibility/cross-implementation-compose.yml down -v
```

## What To Compare

Compare only behaviors that the selected servers can all exercise:

| Gate | Compare |
|------|---------|
| HTTP/device XML | Descriptor path, friendly name, SAT>IP device type, and advertised URLs. |
| Channel source | Whether the client can import the server's advertised channel list or test tune URL. Do not expect identical channel catalogs. |
| RTSP setup | Status line, required method order, `CSeq`, `Session`, and `Transport` header handling. |
| Playback | `PLAY` success, first RTP timing, payload type 33, MPEG-TS sync byte, and teardown behavior when the server has RTP input. |
| Session cleanup | `TEARDOWN` response and whether a later client run starts cleanly. |
| Diagnostics | Client-side logs, server evidence JSON, and any packet captures needed to explain divergences. |

For SSDP discovery, run one server at a time rather than this multi-service
compose fixture. On Linux runners or local Linux hosts, host networking is the
least surprising option because M-SEARCH multicast and UDP port 1900 are
environment-sensitive:

```bash
docker run --rm --network host \
  -e SATIP_LAB_PUBLIC_HOST=127.0.0.1 \
  -e SATIP_LAB_HTTP_PORT=18875 \
  -e SATIP_LAB_RTSP_PORT=1554 \
  ghcr.io/e12media/satip-lab:latest
```

Then run the client project's normal SAT>IP discovery test against that single
server. Repeat with the independent server. Keep discovery results separate from
the RTSP/RTP evidence JSON because discovery failures often come from multicast
routing, host networking, or port conflicts rather than SAT>IP session behavior.

For JSON evidence generated by `satip-lab-smoke`, this compact view is enough
for most PRs:

```bash
jq -S '{
  profile,
  session_id_format,
  rtsp: [.rtsp[] | {method, status_line, headers}],
  rtp
}' /tmp/satip-lab-smoke.json
```

Run the same projection for each server and attach the outputs to the client
PR, a `satip-lab` issue, or a compatibility evidence update.

## GitHub Actions Shape

Client repositories can copy this compose fixture, vendor it from `satip-lab`, or
fetch the exact revision they want to test. Keep the job separate from the normal
deterministic `satip-lab` job so independent server image or network failures do
not block everyday client CI.

```yaml
jobs:
  satip-cross-implementation:
    runs-on: ubuntu-latest
    continue-on-error: true
    steps:
      - uses: actions/checkout@v4
        with:
          path: client
      - uses: actions/checkout@v4
        with:
          repository: e12media/satip-lab
          path: satip-lab
      - name: Start SAT>IP servers
        run: |
          docker compose \
            -f satip-lab/docs/compatibility/cross-implementation-compose.yml \
            up -d --build
      - name: Run client matrix
        working-directory: client
        run: |
          ./run-client-tests --satip-http http://127.0.0.1:18875 --satip-rtsp rtsp://127.0.0.1:1554/
          ./run-client-tests --satip-http http://127.0.0.1:28875 --satip-rtsp rtsp://127.0.0.1:2554/
      - name: Stop SAT>IP servers
        if: always()
        run: docker compose -f satip-lab/docs/compatibility/cross-implementation-compose.yml down -v
```

Replace `./run-client-tests` with the client project's integration command.
Remove `continue-on-error` only after the independent implementation setup is
stable enough for that client repository.

## Divergence Workflow

1. Save the exact command, server image/version, profile, and evidence output.
2. Check whether the behavior is spec-compatible or a server-specific quirk.
3. If `satip-lab` is missing spec-compatible behavior, open a normal bug or
   enhancement issue.
4. If the behavior is non-spec and useful for clients, attach `captured-trace`
   or `owned-hardware` evidence before promoting it into a compatibility
   profile.
