package ts

import (
	"encoding/binary"
	"fmt"
	"time"
)

const (
	defaultEITClock         = "2026-03-29T01:30:00+01:00"
	maxShortEventTitleBytes = 146
)

type ServiceProfile struct {
	ID        string
	Name      string
	ServiceID int
	PMTPID    int
	VideoPID  int
	AudioPID  int
}

type EITOptions struct {
	Now      time.Time
	Suppress bool
}

func SyntheticServiceTransport(profile ServiceProfile) []byte {
	return SyntheticServiceTransportWithOptions(profile, EITOptions{Now: defaultEITNow()})
}

func SyntheticServiceTransportWithOptions(profile ServiceProfile, eit EITOptions) []byte {
	packets := [][]byte{
		syntheticPacket(0, true, 0, psiPayload(patSection(profile))),
		syntheticPacket(uint16(profile.PMTPID), true, 0, psiPayload(pmtSection(profile))),
	}
	if !eit.Suppress {
		for i, section := range EITPresentFollowingSections(profile, eit) {
			packets = append(packets, syntheticPacket(0x12, true, byte(i%16), psiPayload(section)))
		}
	}

	for i := 0; i < 300; i++ {
		videoMarker := pesPayload(0xE0, []byte(fmt.Sprintf("satip-lab:%s:%s:video:%06d", profile.ID, profile.Name, i)))
		audioMarker := pesPayload(0xC0, []byte(fmt.Sprintf("satip-lab:%s:%s:audio:%06d", profile.ID, profile.Name, i)))
		packets = append(packets,
			syntheticPacket(uint16(profile.VideoPID), i == 0, byte(i%16), videoMarker),
			syntheticPacket(uint16(profile.AudioPID), i == 0, byte(i%16), audioMarker),
		)
	}

	out := make([]byte, 0, len(packets)*188)
	for _, packet := range packets {
		out = append(out, packet...)
	}
	return out
}

func EITPresentFollowingSections(profile ServiceProfile, opts EITOptions) [][]byte {
	now := opts.Now
	if now.IsZero() {
		now = defaultEITNow()
	}
	presentStart := now
	presentDuration := densityForSyntheticService(profile.ID)
	followingStart := presentStart.Add(presentDuration)
	followingDuration := densityForSyntheticService(profile.ID)
	return [][]byte{
		eitSection(profile, 0, 1, 4, presentStart, presentDuration),
		eitSection(profile, 1, 1, 1, followingStart, followingDuration),
	}
}

func MalformedPSI(payload []byte) []byte {
	out := append([]byte(nil), payload...)
	pmtPID, ok := corruptPAT(out)
	if ok {
		corruptPMT(out, pmtPID)
	}
	return out
}

func ContinuityCounterErrors(payload []byte) []byte {
	out := append([]byte(nil), payload...)
	seenByPID := make(map[uint16]int)
	for offset := 0; offset+188 <= len(out); offset += 188 {
		packet := out[offset : offset+188]
		if packet[0] != 0x47 {
			continue
		}
		pid := PID(packet)
		seen := seenByPID[pid]
		seenByPID[pid] = seen + 1
		if seen%2 == 0 {
			packet[3] = (packet[3] & 0xF0) | ((packet[3] + 7) & 0x0F)
		}
	}
	return out
}

func corruptPAT(payload []byte) (uint16, bool) {
	for offset := 0; offset+188 <= len(payload); offset += 188 {
		packet := payload[offset : offset+188]
		if PID(packet) != 0 || packet[1]&0x40 == 0 {
			continue
		}
		tableIDOffset, ok := psiTableIDOffset(packet)
		if !ok || packet[tableIDOffset] != 0x00 || tableIDOffset+12 > len(packet) {
			continue
		}
		pmtPID := binary.BigEndian.Uint16([]byte{packet[tableIDOffset+10] & 0x1F, packet[tableIDOffset+11]})
		packet[tableIDOffset] = 0xFF
		return pmtPID, true
	}
	return 0, false
}

func corruptPMT(payload []byte, pmtPID uint16) bool {
	for offset := 0; offset+188 <= len(payload); offset += 188 {
		packet := payload[offset : offset+188]
		if PID(packet) != pmtPID || packet[1]&0x40 == 0 {
			continue
		}
		tableIDOffset, ok := psiTableIDOffset(packet)
		if !ok || packet[tableIDOffset] != 0x02 {
			continue
		}
		packet[tableIDOffset] = 0xFF
		return true
	}
	return false
}

func psiTableIDOffset(packet []byte) (int, bool) {
	if len(packet) != 188 || packet[0] != 0x47 {
		return 0, false
	}
	payloadOffset, ok := payloadOffset(packet)
	if !ok || payloadOffset >= len(packet) {
		return 0, false
	}
	pointer := int(packet[payloadOffset])
	tableIDOffset := payloadOffset + 1 + pointer
	return tableIDOffset, tableIDOffset < len(packet)
}

func payloadOffset(packet []byte) (int, bool) {
	adaptationControl := (packet[3] >> 4) & 0x03
	switch adaptationControl {
	case 0x01:
		return 4, true
	case 0x03:
		offset := 5 + int(packet[4])
		return offset, offset < len(packet)
	default:
		return 0, false
	}
}

func patSection(profile ServiceProfile) []byte {
	section := []byte{
		0x00,
		0xB0, 0x0D,
		0x00, 0x01,
		0xC1,
		0x00,
		0x00,
		byte(profile.ServiceID >> 8), byte(profile.ServiceID),
		0xE0 | byte(profile.PMTPID>>8), byte(profile.PMTPID),
	}
	return appendCRC(section)
}

func pmtSection(profile ServiceProfile) []byte {
	section := []byte{
		0x02,
		0xB0, 0x17,
		byte(profile.ServiceID >> 8), byte(profile.ServiceID),
		0xC1,
		0x00,
		0x00,
		0xE0 | byte(profile.VideoPID>>8), byte(profile.VideoPID),
		0xF0, 0x00,
		0x02, 0xE0 | byte(profile.VideoPID>>8), byte(profile.VideoPID), 0xF0, 0x00,
		0x03, 0xE0 | byte(profile.AudioPID>>8), byte(profile.AudioPID), 0xF0, 0x00,
	}
	return appendCRC(section)
}

func eitSection(profile ServiceProfile, sectionNumber, lastSectionNumber, runningStatus int, start time.Time, duration time.Duration) []byte {
	descriptor := shortEventDescriptor(fmt.Sprintf("%s Lab Programme %02d:%02d", profile.Name, start.In(start.Location()).Hour(), start.In(start.Location()).Minute()))
	section := []byte{
		0x4E,
		0xB0, 0x00,
		byte(profile.ServiceID >> 8), byte(profile.ServiceID),
		0xC1,
		byte(sectionNumber),
		byte(lastSectionNumber),
		0x00, 0x01,
		0x00, 0x01,
		byte(lastSectionNumber),
		0x4E,
	}
	eventID := eventID(profile.ID, start)
	utc := start.UTC()
	mjd := modifiedJulianDate(utc)
	loopLength := len(descriptor)
	event := []byte{
		byte(eventID >> 8), byte(eventID),
		byte(mjd >> 8), byte(mjd),
		bcdByte(utc.Hour()), bcdByte(utc.Minute()), bcdByte(utc.Second()),
		bcdByte(int(duration / time.Hour)),
		bcdByte(int(duration/time.Minute) % 60),
		bcdByte(int(duration/time.Second) % 60),
		byte((runningStatus&0x07)<<5) | byte((loopLength>>8)&0x0F),
		byte(loopLength),
	}
	event = append(event, descriptor...)
	section = append(section, event...)
	sectionLength := len(section) - 3 + 4
	section[1] = 0xB0 | byte((sectionLength>>8)&0x0F)
	section[2] = byte(sectionLength)
	return appendCRC(section)
}

func shortEventDescriptor(title string) []byte {
	titleBytes := []byte(title)
	if len(titleBytes) > maxShortEventTitleBytes {
		titleBytes = titleBytes[:maxShortEventTitleBytes]
	}
	descriptor := []byte{0x4D, 0x00, 'e', 'n', 'g', byte(len(titleBytes))}
	descriptor = append(descriptor, titleBytes...)
	descriptor = append(descriptor, 0x00)
	descriptor[1] = byte(len(descriptor) - 2)
	return descriptor
}

func eventID(serviceID string, start time.Time) uint16 {
	crc := mpegCRC32([]byte(fmt.Sprintf("%s-%s", serviceID, start.UTC().Format("20060102150405"))))
	id := uint16(crc & 0xFFFF)
	if id == 0 {
		return 1
	}
	return id
}

func densityForSyntheticService(serviceID string) time.Duration {
	switch serviceID {
	case "zdf-hd":
		return 30 * time.Minute
	case "arte-hd":
		return 45 * time.Minute
	case "phoenix-hd":
		return 90 * time.Minute
	case "3sat-hd":
		return 2 * time.Hour
	default:
		return time.Hour
	}
}

func modifiedJulianDate(t time.Time) int {
	year, month, day := t.Date()
	if month <= 2 {
		year--
		month += 12
	}
	return int(365.25*float64(year)) + int(30.6001*float64(month+1)) + day - 14956
}

func bcdByte(v int) byte {
	return byte((v/10)<<4 | (v % 10))
}

func defaultEITNow() time.Time {
	t, err := time.Parse(time.RFC3339, defaultEITClock)
	if err != nil {
		return time.Date(2026, 3, 29, 1, 30, 0, 0, time.FixedZone("CET", 3600))
	}
	return t
}

func psiPayload(section []byte) []byte {
	payload := make([]byte, 0, len(section)+1)
	payload = append(payload, 0x00)
	payload = append(payload, section...)
	return payload
}

func pesPayload(streamID byte, marker []byte) []byte {
	packetLen := len(marker) + 3
	payload := []byte{0x00, 0x00, 0x01, streamID, byte(packetLen >> 8), byte(packetLen), 0x80, 0x00, 0x00}
	payload = append(payload, marker...)
	return payload
}

func appendCRC(section []byte) []byte {
	crc := mpegCRC32(section)
	return append(section, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))
}

func mpegCRC32(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, b := range data {
		crc ^= uint32(b) << 24
		for i := 0; i < 8; i++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ 0x04C11DB7
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func syntheticPacket(pid uint16, payloadUnitStart bool, continuity byte, payload []byte) []byte {
	packet := make([]byte, 188)
	packet[0] = 0x47
	packet[1] = byte((pid >> 8) & 0x1F)
	if payloadUnitStart {
		packet[1] |= 0x40
	}
	packet[2] = byte(pid)
	packet[3] = 0x10 | (continuity & 0x0F)
	if len(payload) > 184 {
		payload = payload[:184]
	}
	copy(packet[4:], payload)
	for i := 4 + len(payload); i < len(packet); i++ {
		packet[i] = 0xFF
	}
	return packet
}

func PID(packet []byte) uint16 {
	if len(packet) < 3 {
		return 0
	}
	return binary.BigEndian.Uint16([]byte{packet[1] & 0x1F, packet[2]})
}
