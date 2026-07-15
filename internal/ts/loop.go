package ts

const (
	transportPacketSize        = 188
	timestampMask        int64 = 1<<33 - 1
	timestampHalfRange         = 1 << 32
	defaultTimestampStep       = 90000 / 25
)

// TimestampLoop returns MPEG-TS chunks while keeping PCR, PTS, and DTS
// continuous when a finite payload repeats.
type TimestampLoop struct {
	key             string
	payload         []byte
	rendered        []byte
	offset          int
	nextLoop        bool
	timestampOffset int64
	cycleDuration   int64
}

func (l *TimestampLoop) Next(key string, payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	if l.rendered == nil || key != l.key {
		l.key = key
		l.payload = payload
		l.cycleDuration = timestampCycleDuration(payload)
		l.rendered = rewriteTimestamps(payload, l.timestampOffset)
		if l.offset < 0 || l.offset >= len(l.rendered) {
			l.offset = 0
		}
	}
	if l.nextLoop {
		l.timestampOffset = (l.timestampOffset + l.cycleDuration) & timestampMask
		l.rendered = rewriteTimestamps(l.payload, l.timestampOffset)
		l.offset = 0
		l.nextLoop = false
	}

	chunk, next := (&Source{}).ChunkAt(l.rendered, l.offset)
	l.offset = next
	l.nextLoop = next == 0
	return chunk
}

type timestampTrack struct {
	initialized bool
	previousRaw int64
	current     int64
	minimum     int64
	maximum     int64
	minimumStep int64
}

type timestampTrackKey struct {
	pid  uint16
	kind byte
}

func timestampCycleDuration(payload []byte) int64 {
	tracks := make(map[timestampTrackKey]*timestampTrack)
	visitTransportTimestamps(payload, func(pid uint16, kind byte, raw int64, _ []byte) {
		key := timestampTrackKey{pid: pid, kind: kind}
		track := tracks[key]
		if track == nil {
			track = &timestampTrack{}
			tracks[key] = track
		}
		track.observe(raw)
	})

	duration := int64(defaultTimestampStep)
	for _, track := range tracks {
		step := track.minimumStep
		if step == 0 {
			step = defaultTimestampStep
		}
		candidate := track.maximum - track.minimum + step
		if candidate > duration {
			duration = candidate
		}
	}
	return duration
}

func (t *timestampTrack) observe(raw int64) {
	if !t.initialized {
		t.initialized = true
		t.previousRaw = raw
		t.current = raw
		t.minimum = raw
		t.maximum = raw
		return
	}
	delta := (raw - t.previousRaw) & timestampMask
	if delta >= timestampHalfRange {
		delta -= timestampMask + 1
	}
	t.current += delta
	t.previousRaw = raw
	if t.current < t.minimum {
		t.minimum = t.current
	}
	if t.current > t.maximum {
		t.maximum = t.current
	}
	if delta > 0 && (t.minimumStep == 0 || delta < t.minimumStep) {
		t.minimumStep = delta
	}
}

func rewriteTimestamps(payload []byte, offset int64) []byte {
	out := append([]byte(nil), payload...)
	if offset == 0 {
		return out
	}
	visitTransportTimestamps(out, func(_ uint16, kind byte, raw int64, encoded []byte) {
		shifted := (raw + offset) & timestampMask
		if kind == 'R' {
			writePCRBase(encoded, shifted)
			return
		}
		writePESTimestamp(encoded, shifted)
	})
	return out
}

func visitTransportTimestamps(payload []byte, visit func(pid uint16, kind byte, raw int64, encoded []byte)) {
	for offset := 0; offset+transportPacketSize <= len(payload); offset += transportPacketSize {
		packet := payload[offset : offset+transportPacketSize]
		if packet[0] != 0x47 {
			continue
		}
		pid := PID(packet)
		visitPCR(packet, pid, visit)
		visitPESTimestamps(packet, pid, visit)
	}
}

func visitPCR(packet []byte, pid uint16, visit func(pid uint16, kind byte, raw int64, encoded []byte)) {
	adaptationControl := (packet[3] >> 4) & 0x03
	if adaptationControl != 0x02 && adaptationControl != 0x03 {
		return
	}
	if packet[4] < 7 || packet[5]&0x10 == 0 {
		return
	}
	encoded := packet[6:12]
	visit(pid, 'R', readPCRBase(encoded), encoded)
}

func visitPESTimestamps(packet []byte, pid uint16, visit func(pid uint16, kind byte, raw int64, encoded []byte)) {
	if packet[1]&0x40 == 0 {
		return
	}
	payloadStart, ok := payloadOffset(packet)
	if !ok || payloadStart+14 > len(packet) {
		return
	}
	pes := packet[payloadStart:]
	if pes[0] != 0x00 || pes[1] != 0x00 || pes[2] != 0x01 {
		return
	}
	flags := (pes[7] >> 6) & 0x03
	if flags != 0x02 && flags != 0x03 {
		return
	}
	pts := pes[9:14]
	visit(pid, 'P', readPESTimestamp(pts), pts)
	if flags == 0x03 && len(pes) >= 19 {
		dts := pes[14:19]
		visit(pid, 'D', readPESTimestamp(dts), dts)
	}
}

func readPCRBase(encoded []byte) int64 {
	return int64(encoded[0])<<25 |
		int64(encoded[1])<<17 |
		int64(encoded[2])<<9 |
		int64(encoded[3])<<1 |
		int64(encoded[4]>>7)
}

func writePCRBase(encoded []byte, value int64) {
	encoded[0] = byte(value >> 25)
	encoded[1] = byte(value >> 17)
	encoded[2] = byte(value >> 9)
	encoded[3] = byte(value >> 1)
	encoded[4] = (encoded[4] & 0x7F) | byte(value&1)<<7
}

func readPESTimestamp(encoded []byte) int64 {
	return int64(encoded[0]&0x0E)<<29 |
		int64(encoded[1])<<22 |
		int64(encoded[2]&0xFE)<<14 |
		int64(encoded[3])<<7 |
		int64(encoded[4]>>1)
}

func writePESTimestamp(encoded []byte, value int64) {
	prefix := encoded[0] & 0xF0
	encoded[0] = prefix | byte((value>>29)&0x0E) | 0x01
	encoded[1] = byte(value >> 22)
	encoded[2] = byte((value>>14)&0xFE) | 0x01
	encoded[3] = byte(value >> 7)
	encoded[4] = byte(value<<1) | 0x01
}
