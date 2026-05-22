package rtsp

import "testing"

func TestRTPSenderSkipAdvancesSequenceAndTimestamp(t *testing.T) {
	sender := NewRTPSender()

	sender.Skip()

	if sender.sequence != 1 {
		t.Fatalf("sequence after skip: got %d want 1", sender.sequence)
	}
	if sender.timestamp != 90000/25 {
		t.Fatalf("timestamp after skip: got %d want %d", sender.timestamp, 90000/25)
	}
}
