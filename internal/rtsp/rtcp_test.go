package rtsp

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestRTCPStatusSenderPacketContainsSES1AppStatus(t *testing.T) {
	sender := NewRTCPStatusSender()

	packet := sender.Packet(RTCPStatus{
		SourceID:       1,
		TunerID:        1,
		Level:          88,
		Lock:           1,
		Quality:        15,
		Frequency:      11494,
		Polarity:       "h",
		DeliverySystem: "dvbs2",
		SymbolRate:     22000,
		PIDs:           "0,17,5100,5101,5102",
	})

	if len(packet)%4 != 0 {
		t.Fatalf("RTCP packet should be 32-bit padded, got %d bytes", len(packet))
	}
	if len(packet) < 28 || packet[0] != 0x80 || packet[1] != rtcpPacketTypeSenderReport {
		t.Fatalf("RTCP packet should start with sender report, got % x", packet[:min(len(packet), 8)])
	}
	app := rtcpSectionByType(t, packet, rtcpPacketTypeApp)
	if len(app) < 16 {
		t.Fatalf("APP section too short: %d bytes", len(app))
	}
	if string(app[8:12]) != "SES1" {
		t.Fatalf("APP name: got %q", string(app[8:12]))
	}
	statusLen := int(binary.BigEndian.Uint16(app[14:16]))
	if statusLen <= 0 || 16+statusLen > len(app) {
		t.Fatalf("APP status length %d outside section size %d", statusLen, len(app))
	}
	status := string(app[16 : 16+statusLen])
	for _, want := range []string{
		"ver=1.2",
		"src=1",
		"tuner=1,88,1,15,11494,h,dvbs2,22000",
		"pids=0,17,5100,5101,5102",
	} {
		if !strings.Contains(status, want) {
			t.Fatalf("status string missing %q: %q", want, status)
		}
	}
	for _, pad := range app[16+statusLen:] {
		if pad != 0 {
			t.Fatalf("APP padding should be zeroed, got % x", app[16+statusLen:])
		}
	}
}

func TestRTCPStatusSenderSectionLengthsMatchEncodedPacket(t *testing.T) {
	packet := NewRTCPStatusSender().Packet(RTCPStatus{
		SourceID:       1,
		TunerID:        2,
		Level:          42,
		Lock:           1,
		Quality:        7,
		Frequency:      11362,
		Polarity:       "h",
		DeliverySystem: "dvbs2",
		SymbolRate:     22000,
		PIDs:           "all",
	})

	sectionTypes := []byte{}
	offset := 0
	for offset < len(packet) {
		if offset+4 > len(packet) {
			t.Fatalf("truncated RTCP header at offset %d", offset)
		}
		sectionLen := int(binary.BigEndian.Uint16(packet[offset+2:offset+4])+1) * 4
		if sectionLen <= 0 || offset+sectionLen > len(packet) {
			t.Fatalf("invalid RTCP section length %d at offset %d in %d-byte packet", sectionLen, offset, len(packet))
		}
		sectionTypes = append(sectionTypes, packet[offset+1])
		offset += sectionLen
	}
	if offset != len(packet) {
		t.Fatalf("RTCP section lengths ended at %d, packet length %d", offset, len(packet))
	}
	want := []byte{rtcpPacketTypeSenderReport, rtcpPacketTypeSourceDescription, rtcpPacketTypeApp}
	if string(sectionTypes) != string(want) {
		t.Fatalf("RTCP section types: got %v want %v", sectionTypes, want)
	}
}

func rtcpSectionByType(t *testing.T, packet []byte, packetType byte) []byte {
	t.Helper()
	for offset := 0; offset+4 <= len(packet); {
		sectionLen := int(binary.BigEndian.Uint16(packet[offset+2:offset+4])+1) * 4
		if sectionLen <= 0 || offset+sectionLen > len(packet) {
			t.Fatalf("invalid RTCP section length %d at offset %d", sectionLen, offset)
		}
		section := packet[offset : offset+sectionLen]
		if section[1] == packetType {
			return section
		}
		offset += sectionLen
	}
	t.Fatalf("RTCP section type 0x%02x not found in % x", packetType, packet)
	return nil
}
