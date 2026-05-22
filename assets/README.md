# Transport stream asset

`satip-lab` can loop `sample.ts` for RTP streaming after RTSP `PLAY` when
`SATIP_LAB_TS_PATH=assets/sample.ts` is set.

The Docker image also includes service sample profiles:

| File | Profile | Purpose |
|------|---------|---------|
| `h264_aac_short.ts` | `SATIP_LAB_SAMPLE_PROFILE=h264_aac_short` | Short H.264/AAC test pattern used for ZDF HD. |
| `h264_silent.ts` | `SATIP_LAB_SAMPLE_PROFILE=h264_silent` | Same style of H.264 test pattern with silent AAC audio for audio-selection tests. |

With a sample profile enabled, only ZDF HD uses the sample. Other services keep
their distinct synthetic MPEG-TS packets so routing tests can still tell
channels apart. The sample assets are generated with ZDF HD's advertised service
id and PMT/video/audio PID layout so PID-filtering clients can decode them.
`SATIP_LAB_TS_PATH` overrides all sample profiles and loops one file for every
service.

The files are generated (not committed) to keep the repository small:

```bash
./tool/generate_sample_ts.sh
```

Docker images build these files during `docker build` using `ffmpeg`.
Generation also runs `ffprobe` verification for the ZDF HD service id and
PMT/video/audio PID layout.

Without `SATIP_LAB_TS_PATH` or a sample profile, the simulator uses generated
service-specific TS payloads. These are intended for protocol and demux tests,
not production TV viewing.
