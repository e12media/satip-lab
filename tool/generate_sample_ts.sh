#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${ROOT}/assets/sample.ts"
H264_AAC_OUT="${ROOT}/assets/h264_aac_short.ts"
H264_SILENT_OUT="${ROOT}/assets/h264_silent.ts"
ZDF_SERVICE_ID=1002
ZDF_PMT_PID=6100
ZDF_VIDEO_PID=6110
ZDF_AUDIO_PID=6120

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "ffmpeg is required to generate assets/*.ts" >&2
  exit 1
fi
if ! command -v ffprobe >/dev/null 2>&1; then
  echo "ffprobe is required to verify generated assets/*.ts" >&2
  exit 1
fi

mkdir -p "$(dirname "${OUT}")"

ffmpeg -hide_banner -loglevel error -y \
  -f lavfi -i 'testsrc=size=640x360:rate=25' \
  -f lavfi -i 'sine=frequency=440' \
  -c:v mpeg2video -b:v 2M \
  -c:a mp2 -b:a 128k \
  -f mpegts -mpegts_transport_stream_id 1 \
  -t 30 \
  "${OUT}"

echo "Wrote ${OUT}"

ffmpeg -hide_banner -loglevel error -y \
  -f lavfi -i 'testsrc2=size=640x360:rate=25' \
  -f lavfi -i 'sine=frequency=440:sample_rate=48000' \
  -c:v libx264 -preset ultrafast -tune zerolatency -profile:v baseline -level 3.0 -pix_fmt yuv420p -b:v 900k \
  -c:a aac -b:a 96k -ar 48000 -ac 2 \
  -streamid "0:${ZDF_VIDEO_PID}" -streamid "1:${ZDF_AUDIO_PID}" \
  -f mpegts -mpegts_transport_stream_id 2 -mpegts_service_id "${ZDF_SERVICE_ID}" -mpegts_pmt_start_pid "${ZDF_PMT_PID}" \
  -t 12 \
  "${H264_AAC_OUT}"

echo "Wrote ${H264_AAC_OUT}"

ffmpeg -hide_banner -loglevel error -y \
  -f lavfi -i 'testsrc2=size=640x360:rate=25' \
  -f lavfi -i 'anullsrc=channel_layout=stereo:sample_rate=48000' \
  -c:v libx264 -preset ultrafast -tune zerolatency -profile:v baseline -level 3.0 -pix_fmt yuv420p -b:v 900k \
  -c:a aac -b:a 32k -ar 48000 -ac 2 \
  -streamid "0:${ZDF_VIDEO_PID}" -streamid "1:${ZDF_AUDIO_PID}" \
  -f mpegts -mpegts_transport_stream_id 3 -mpegts_service_id "${ZDF_SERVICE_ID}" -mpegts_pmt_start_pid "${ZDF_PMT_PID}" \
  -t 12 \
  "${H264_SILENT_OUT}"

echo "Wrote ${H264_SILENT_OUT}"

"${ROOT}/tool/verify_sample_ts.sh"
