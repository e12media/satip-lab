# Release Checklist

Use this before publishing a first OSS version or cutting a tagged release.

## Required checks

```bash
make test
make lint
docker build -t satip-lab:release-check .
docker run -d --name satip-lab-release-check \
  --network host \
  -e SATIP_LAB_PUBLIC_HOST=127.0.0.1 \
  -e SATIP_LAB_HTTP_PORT=18875 \
  -e SATIP_LAB_RTSP_PORT=1554 \
  -e SATIP_LAB_SSDP_PORT=0 \
  satip-lab:release-check
curl -fsS http://127.0.0.1:18875/desc.xml | grep -q 'SatIPServer'
curl -fsS http://127.0.0.1:18875/channels.m3u | grep -q 'ZDF HD'
go run ./cmd/satip-lab-smoke --host 127.0.0.1 --rtsp-port 1554
docker rm -f satip-lab-release-check
```

On Docker Desktop or other NAT-backed runtimes, the RTP smoke command may need `--rtp-destination <host-ip-from-container>`.

## OSS readiness

- Keep the README honest about simulated and non-simulated behavior.
- Update `docs/supported-profile.md` for every user-visible protocol change.
- Keep `docs/support-matrix.md` aligned with the supported profile before tagging a stable release.
- Confirm the Docker image contains generated `assets/*.ts` media assets, including `sample.ts`, `h264_aac_short.ts`, and `h264_silent.ts`.
- Confirm downstream examples do not require real tuner hardware.
- Tag releases only from a green CI run.
- Verify anonymous GHCR pull access for public release images.
- Verify the release image manifest contains `linux/amd64` and `linux/arm64`.

## Publishing

Tagged releases matching `v*.*.*` publish the Docker image to `ghcr.io/e12media/satip-lab` through `.github/workflows/release.yml`. The release workflow runs Go tests, vet, builds the Docker image, and smoke-tests the image before pushing.

Release tags publish:

- `ghcr.io/e12media/satip-lab:<version>` (for example `1.0.0`)
- `ghcr.io/e12media/satip-lab:<major>.<minor>` (for example `1.0`)
- `ghcr.io/e12media/satip-lab:latest` for final semver tags only, not prerelease tags containing `-`

Do not document an image tag as available until the release workflow has completed successfully for that tag.

Verify unauthenticated usage:

```bash
docker logout ghcr.io || true
docker pull ghcr.io/e12media/satip-lab:latest
docker buildx imagetools inspect ghcr.io/e12media/satip-lab:latest
```

If anonymous pull returns `403`, check the package visibility in GitHub package settings. Managing package visibility through the REST API requires a token with package scopes in addition to suitable organization/package permissions.
