package rtsp

import (
	"encoding/binary"
	"net"
)

const payloadTypeMP2T = 33

type RTPSender struct {
	sequence  uint16
	timestamp uint32
	ssrc      uint32
}

func NewRTPSender() *RTPSender {
	return &RTPSender{ssrc: 0x73617470}
}

func (r *RTPSender) Send(conn *net.UDPConn, dest *net.UDPAddr, mpegTsChunk []byte) error {
	packet := make([]byte, 12+len(mpegTsChunk))
	packet[0] = 0x80
	packet[1] = payloadTypeMP2T & 0x7F
	binary.BigEndian.PutUint16(packet[2:4], r.sequence)

	binary.BigEndian.PutUint32(packet[4:8], r.timestamp)

	binary.BigEndian.PutUint32(packet[8:12], r.ssrc)
	copy(packet[12:], mpegTsChunk)

	r.Skip()
	_, err := conn.WriteToUDP(packet, dest)
	return err
}

func (r *RTPSender) Skip() {
	r.sequence++
	r.timestamp = (r.timestamp + 90000/25) & 0xFFFFFFFF
}
