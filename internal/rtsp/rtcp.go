package rtsp

import (
	"encoding/binary"
	"fmt"
)

const (
	rtcpPacketTypeSenderReport      = 200
	rtcpPacketTypeSourceDescription = 202
	rtcpPacketTypeApp               = 204
	rtcpAppNameSATIP                = "SES1"
)

type RTCPStatus struct {
	SourceID       int
	TunerID        int
	Level          int
	Lock           int
	Quality        int
	Frequency      int
	Polarity       string
	DeliverySystem string
	SymbolRate     int
	PIDs           string
}

type RTCPStatusSender struct {
	ssrc  uint32
	cname string
}

func NewRTCPStatusSender() *RTCPStatusSender {
	return &RTCPStatusSender{ssrc: 0x73617470, cname: "satip-lab"}
}

func (r *RTCPStatusSender) Packet(status RTCPStatus) []byte {
	packet := make([]byte, 0, 96)
	packet = append(packet, r.senderReportPacket()...)
	packet = append(packet, r.sourceDescriptionPacket()...)
	packet = append(packet, r.appStatusPacket(status)...)
	return packet
}

func (r *RTCPStatusSender) senderReportPacket() []byte {
	packet := make([]byte, 28)
	packet[0] = 0x80
	packet[1] = rtcpPacketTypeSenderReport
	binary.BigEndian.PutUint16(packet[2:4], 6)
	binary.BigEndian.PutUint32(packet[4:8], r.ssrc)
	return packet
}

func (r *RTCPStatusSender) sourceDescriptionPacket() []byte {
	cname := []byte(r.cname)
	length := padTo32Bit(4 + 4 + 2 + len(cname) + 1)
	packet := make([]byte, length)
	packet[0] = 0x81
	packet[1] = rtcpPacketTypeSourceDescription
	binary.BigEndian.PutUint16(packet[2:4], uint16(length/4-1))
	binary.BigEndian.PutUint32(packet[4:8], r.ssrc)
	packet[8] = 1
	packet[9] = byte(len(cname))
	copy(packet[10:], cname)
	return packet
}

func (r *RTCPStatusSender) appStatusPacket(status RTCPStatus) []byte {
	statusString := status.String()
	length := padTo32Bit(16 + len(statusString))
	packet := make([]byte, length)
	packet[0] = 0x80
	packet[1] = rtcpPacketTypeApp
	binary.BigEndian.PutUint16(packet[2:4], uint16(length/4-1))
	binary.BigEndian.PutUint32(packet[4:8], r.ssrc)
	copy(packet[8:12], rtcpAppNameSATIP)
	binary.BigEndian.PutUint16(packet[14:16], uint16(len(statusString)))
	copy(packet[16:], statusString)
	return packet
}

func (s RTCPStatus) String() string {
	return fmt.Sprintf(
		"ver=1.2;src=%d;tuner=%d,%d,%d,%d,%d,%s,%s,%d;pids=%s",
		s.SourceID,
		s.TunerID,
		s.Level,
		s.Lock,
		s.Quality,
		s.Frequency,
		s.Polarity,
		s.DeliverySystem,
		s.SymbolRate,
		s.PIDs,
	)
}

func padTo32Bit(length int) int {
	if length%4 == 0 {
		return length
	}
	return length + 4 - length%4
}
