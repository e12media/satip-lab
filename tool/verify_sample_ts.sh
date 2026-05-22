#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ZDF_SERVICE_ID=1002
ZDF_PMT_PID=6100
ZDF_VIDEO_HEX_ID=0x17de
ZDF_AUDIO_HEX_ID=0x17e8

if ! command -v ffprobe >/dev/null 2>&1; then
  echo "ffprobe is required to verify generated assets/*.ts" >&2
  exit 1
fi

require_line() {
  local body="$1"
  local expected="$2"
  local label="$3"
  if ! grep -qx "${expected}" <<<"${body}"; then
    echo "Expected ${label}: ${expected}" >&2
    echo "${body}" >&2
    exit 1
  fi
}

verify_zdf_profile_asset() {
  local file="$1"
  local program
  local video
  local audio

  program="$(ffprobe -v error -show_entries program=program_id,pmt_pid -of default=noprint_wrappers=1 "${file}")"
  require_line "${program}" "program_id=${ZDF_SERVICE_ID}" "${file} program id"
  require_line "${program}" "pmt_pid=${ZDF_PMT_PID}" "${file} PMT PID"

  video="$(ffprobe -v error -select_streams v:0 -show_entries stream=codec_name,id -of default=noprint_wrappers=1 "${file}")"
  require_line "${video}" "codec_name=h264" "${file} video codec"
  require_line "${video}" "id=${ZDF_VIDEO_HEX_ID}" "${file} video PID"

  audio="$(ffprobe -v error -select_streams a:0 -show_entries stream=codec_name,id -of default=noprint_wrappers=1 "${file}")"
  require_line "${audio}" "codec_name=aac" "${file} audio codec"
  require_line "${audio}" "id=${ZDF_AUDIO_HEX_ID}" "${file} audio PID"
}

verify_zdf_profile_asset "${ROOT}/assets/h264_aac_short.ts"
verify_zdf_profile_asset "${ROOT}/assets/h264_silent.ts"

echo "Verified ZDF sample profile assets"
