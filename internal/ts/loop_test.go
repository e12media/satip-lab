package ts

import (
	"bytes"
	"testing"
)

func TestTimestampLoopKeepsPCRPTSandDTSMonotonicAcrossWrap(t *testing.T) {
	payload := append(
		timestampTestPacket(0x100, 90000, 93600, 90000),
		timestampTestPacket(0x100, 93600, 97200, 93600)...,
	)
	loop := &TimestampLoop{}

	first := append([]byte(nil), loop.Next("normal", payload)...)
	second := append([]byte(nil), loop.Next("normal", payload)...)

	firstPCR, firstPTS, firstDTS := timestampsFromTestPacket(t, first)
	secondPCR, secondPTS, secondDTS := timestampsFromTestPacket(t, second)
	if firstPCR != 90000 || firstPTS != 93600 || firstDTS != 90000 {
		t.Fatalf("first loop timestamps: PCR=%d PTS=%d DTS=%d", firstPCR, firstPTS, firstDTS)
	}
	if secondPCR != 97200 || secondPTS != 100800 || secondDTS != 97200 {
		t.Fatalf("second loop timestamps: PCR=%d PTS=%d DTS=%d", secondPCR, secondPTS, secondDTS)
	}
}

func TestTimestampLoopPreservesTimelineWhenPayloadKeyChanges(t *testing.T) {
	payload := timestampTestPacket(0x100, 90000, 93600, 90000)
	changed := append([]byte(nil), payload...)
	changed[40] ^= 0x01
	loop := &TimestampLoop{}

	_ = loop.Next("normal", payload)
	next := loop.Next("continuity_errors", changed)

	pcr, pts, dts := timestampsFromTestPacket(t, next)
	if pcr != 93600 || pts != 97200 || dts != 93600 {
		t.Fatalf("changed payload timestamps: PCR=%d PTS=%d DTS=%d", pcr, pts, dts)
	}
}

func TestTimestampLoopLeavesTimestampFreeSyntheticPayloadUnchanged(t *testing.T) {
	payload := SyntheticServiceTransport(ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	})
	loop := &TimestampLoop{}

	for offset := 0; offset < len(payload); offset += chunkSize {
		got := loop.Next("normal", payload)
		end := min(offset+chunkSize, len(payload))
		if !bytes.Equal(got, payload[offset:end]) {
			t.Fatalf("synthetic chunk at offset %d changed", offset)
		}
	}
}

func timestampTestPacket(pid uint16, pcr, pts, dts int64) []byte {
	packet := make([]byte, transportPacketSize)
	for i := range packet {
		packet[i] = 0xFF
	}
	packet[0] = 0x47
	packet[1] = 0x40 | byte(pid>>8)&0x1F
	packet[2] = byte(pid)
	packet[3] = 0x30
	packet[4] = 7
	packet[5] = 0x10
	writePCRBase(packet[6:12], pcr)
	packet[12] = 0x00
	packet[13] = 0x00
	packet[14] = 0x01
	packet[15] = 0xE0
	packet[16] = 0x00
	packet[17] = 0x00
	packet[18] = 0x80
	packet[19] = 0xC0
	packet[20] = 10
	writePESTimestampWithPrefix(packet[21:26], pts, 0x30)
	writePESTimestampWithPrefix(packet[26:31], dts, 0x10)
	return packet
}

func writePESTimestampWithPrefix(encoded []byte, value int64, prefix byte) {
	encoded[0] = prefix
	writePESTimestamp(encoded, value)
}

func timestampsFromTestPacket(t *testing.T, packet []byte) (int64, int64, int64) {
	t.Helper()
	if len(packet) < transportPacketSize {
		t.Fatalf("packet length: got %d want at least %d", len(packet), transportPacketSize)
	}
	packet = packet[:transportPacketSize]
	return readPCRBase(packet[6:12]), readPESTimestamp(packet[21:26]), readPESTimestamp(packet[26:31])
}
