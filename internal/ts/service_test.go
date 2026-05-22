package ts_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/e12media/satip-lab/internal/ts"
)

func TestSyntheticServiceTransportIsDistinctAndIncludesPATPMT(t *testing.T) {
	one := ts.SyntheticServiceTransport(ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	})
	two := ts.SyntheticServiceTransport(ts.ServiceProfile{
		ID:        "zdf-hd",
		Name:      "ZDF HD",
		ServiceID: 1002,
		PMTPID:    6100,
		VideoPID:  6110,
		AudioPID:  6120,
	})

	if len(one)%188 != 0 || len(two)%188 != 0 {
		t.Fatalf("transport streams must be packet aligned: %d %d", len(one), len(two))
	}
	if one[0] != 0x47 || one[188] != 0x47 || two[0] != 0x47 || two[188] != 0x47 {
		t.Fatal("expected MPEG-TS sync bytes at packet boundaries")
	}
	if bytes.Equal(one, two) {
		t.Fatal("distinct services should generate distinct transport streams")
	}
	if !bytes.Contains(one, []byte("das-erste-hd")) {
		t.Fatal("expected service marker in generated payload")
	}
	if !bytes.Contains(two, []byte("zdf-hd")) {
		t.Fatal("expected service marker in generated payload")
	}
}

func TestEITPresentFollowingSectionsUseServiceSchedule(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 29, 1, 30, 0, 0, loc)
	sections := ts.EITPresentFollowingSections(ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	}, ts.EITOptions{Now: now})

	if len(sections) != 2 {
		t.Fatalf("sections: got %d", len(sections))
	}
	for i, section := range sections {
		if section[0] != 0x4E {
			t.Fatalf("section %d table id: got 0x%x", i, section[0])
		}
		if got := int(section[6]); got != i {
			t.Fatalf("section %d section_number: got %d", i, got)
		}
		if got := int(section[8])<<8 | int(section[9]); got != 1 {
			t.Fatalf("section %d transport_stream_id: got %d", i, got)
		}
		if got := int(section[10])<<8 | int(section[11]); got != 1 {
			t.Fatalf("section %d original_network_id: got %d", i, got)
		}
	}
	present := sections[0]
	if got := int(present[3])<<8 | int(present[4]); got != 1001 {
		t.Fatalf("service id: got %d", got)
	}
	if got := int(present[14])<<8 | int(present[15]); got == 0 {
		t.Fatal("event id must be non-zero")
	}
	if got := present[18:21]; !bytes.Equal(got, []byte{0x00, 0x30, 0x00}) {
		t.Fatalf("present start UTC BCD: got % x", got)
	}
	if got := present[21:24]; !bytes.Equal(got, []byte{0x01, 0x00, 0x00}) {
		t.Fatalf("present duration BCD: got % x", got)
	}
	if got := present[24] >> 5; got != 4 {
		t.Fatalf("present running status: got %d", got)
	}
	if got := sections[1][24] >> 5; got != 1 {
		t.Fatalf("following running status: got %d", got)
	}
	if !bytes.Contains(present, []byte("Das Erste HD Lab Programme 01:30")) {
		t.Fatalf("missing short event name in present section: % x", present)
	}
	if !bytes.Contains(sections[1], []byte("Das Erste HD Lab Programme 03:30")) {
		t.Fatalf("missing short event name in following section: % x", sections[1])
	}
}

func TestEITPresentFollowingSectionsCapLongTitlesToSingleTSPacket(t *testing.T) {
	sections := ts.EITPresentFollowingSections(ts.ServiceProfile{
		ID:        "long-name",
		Name:      strings.Repeat("Very Long Service Name ", 20),
		ServiceID: 1999,
		PMTPID:    7000,
		VideoPID:  7001,
		AudioPID:  7002,
	}, ts.EITOptions{})

	for _, section := range sections {
		if len(section)+1 > 184 {
			t.Fatalf("EIT section plus pointer must fit one TS packet: got %d bytes", len(section)+1)
		}
		if len(section) < 31 {
			t.Fatalf("section too short: %d", len(section))
		}
		descriptorOffset := 26
		if section[descriptorOffset] != 0x4D {
			t.Fatalf("short event descriptor tag: got 0x%x", section[descriptorOffset])
		}
		descriptorLength := int(section[descriptorOffset+1])
		if descriptorOffset+2+descriptorLength+4 != len(section) {
			t.Fatalf("descriptor length does not align with section length: descriptor=%d section=%d", descriptorLength, len(section))
		}
	}
}

func TestSyntheticServiceTransportIncludesEITPresentFollowingPackets(t *testing.T) {
	payload := ts.SyntheticServiceTransport(ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	})

	sections := sectionsByPID(payload, 0x12)
	if len(sections) != 2 {
		t.Fatalf("EIT sections on PID 0x12: got %d", len(sections))
	}
	if sections[0][0] != 0x4E || sections[1][0] != 0x4E {
		t.Fatalf("EIT table ids: got 0x%x 0x%x", sections[0][0], sections[1][0])
	}
	if sections[0][6] != 0 || sections[1][6] != 1 {
		t.Fatalf("EIT section numbers: got %d %d", sections[0][6], sections[1][6])
	}
	if !bytes.Contains(sections[0], []byte("Das Erste HD Lab Programme")) {
		t.Fatalf("missing service EIT title: % x", sections[0])
	}
}

func TestSyntheticServiceTransportUsesExpectedPATAndPMTPIDs(t *testing.T) {
	payload := ts.SyntheticServiceTransport(ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	})

	pids := firstPIDs(payload, 6)
	want := []uint16{0, 5100, 0x12, 0x12, 5101, 5102}
	for i := range want {
		if pids[i] != want[i] {
			t.Fatalf("pid[%d]: got %d want %d; all=%v", i, pids[i], want[i], pids)
		}
	}
	if payload[1]&0x40 == 0 {
		t.Fatalf("first packet should set payload unit start")
	}
	if payload[188+1]&0x40 == 0 {
		t.Fatalf("second packet should set payload unit start")
	}
	if got := payload[4]; got != 0x00 {
		t.Fatalf("first PSI pointer field: got 0x%x", got)
	}
	if got := payload[5]; got != 0x00 {
		t.Fatalf("first PSI table id: got 0x%x", got)
	}
	if got := payload[188+5]; got != 0x02 {
		t.Fatalf("second PSI table id: got 0x%x", got)
	}
}

func TestMalformedPSICorruptsTableHeadersOnly(t *testing.T) {
	payload := ts.SyntheticServiceTransport(ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	})

	malformed := ts.MalformedPSI(payload)

	if len(malformed) != len(payload) {
		t.Fatalf("length changed: got %d want %d", len(malformed), len(payload))
	}
	if malformed[0] != 0x47 || malformed[188] != 0x47 {
		t.Fatal("malformed PSI should preserve TS sync bytes")
	}
	if malformed[5] == payload[5] {
		t.Fatalf("PAT table id was not corrupted: got 0x%x", malformed[5])
	}
	if malformed[188+5] == payload[188+5] {
		t.Fatalf("PMT table id was not corrupted: got 0x%x", malformed[188+5])
	}
	if payload[5] != 0x00 || payload[188+5] != 0x02 {
		t.Fatal("MalformedPSI must not mutate the input payload")
	}
}

func TestMalformedPSIIgnoresPayloadWithoutPSI(t *testing.T) {
	payload := bytes.Repeat([]byte{0x47, 0x1F, 0xFF, 0x10}, 47)
	payload = append(payload, bytes.Repeat([]byte{0xFF}, 188)...)

	malformed := ts.MalformedPSI(payload)

	if !bytes.Equal(malformed, payload) {
		t.Fatal("payload without PAT/PMT PSI should remain unchanged")
	}
}

func TestContinuityCounterErrorsCorruptCountersOnly(t *testing.T) {
	payload := ts.SyntheticServiceTransport(ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	})

	corrupted := ts.ContinuityCounterErrors(payload)

	if len(corrupted) != len(payload) {
		t.Fatalf("length changed: got %d want %d", len(corrupted), len(payload))
	}
	if corrupted[0] != 0x47 || corrupted[188] != 0x47 {
		t.Fatal("continuity counter corruption should preserve TS sync bytes")
	}
	if corrupted[3]&0xF0 != payload[3]&0xF0 {
		t.Fatalf("header flags changed: got 0x%x want 0x%x", corrupted[3]&0xF0, payload[3]&0xF0)
	}
	if corrupted[3]&0x0F == payload[3]&0x0F {
		t.Fatalf("continuity counter did not change: got 0x%x", corrupted[3]&0x0F)
	}
	if payload[3]&0x0F != 0 {
		t.Fatal("ContinuityCounterErrors must not mutate the input payload")
	}

	videoCounters := countersForPID(corrupted, 5101, 4)
	if len(videoCounters) < 2 {
		t.Fatalf("expected video counters, got %v", videoCounters)
	}
	if got, want := videoCounters[1], byte((videoCounters[0]+1)&0x0F); got == want {
		t.Fatalf("expected per-PID continuity jump, counters=%v", videoCounters)
	}
}

func firstPIDs(payload []byte, count int) []uint16 {
	out := make([]uint16, 0, count)
	for offset := 0; offset+188 <= len(payload) && len(out) < count; offset += 188 {
		out = append(out, ts.PID(payload[offset:offset+188]))
	}
	return out
}

func countersForPID(payload []byte, pid uint16, count int) []byte {
	out := make([]byte, 0, count)
	for offset := 0; offset+188 <= len(payload) && len(out) < count; offset += 188 {
		packet := payload[offset : offset+188]
		if ts.PID(packet) == pid {
			out = append(out, packet[3]&0x0F)
		}
	}
	return out
}

func sectionsByPID(payload []byte, pid uint16) [][]byte {
	var out [][]byte
	for offset := 0; offset+188 <= len(payload); offset += 188 {
		packet := payload[offset : offset+188]
		if ts.PID(packet) != pid || packet[1]&0x40 == 0 {
			continue
		}
		start := 5 + int(packet[4])
		if start+3 > len(packet) {
			continue
		}
		length := int(packet[start+1]&0x0F)<<8 | int(packet[start+2])
		end := start + 3 + length
		if end > len(packet) {
			continue
		}
		out = append(out, append([]byte(nil), packet[start:end]...))
	}
	return out
}
