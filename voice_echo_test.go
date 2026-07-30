package main

import (
	"bytes"
	"testing"
	"time"
)

func TestVoiceEchoWaitsForEndThenReplaysInSequence(t *testing.T) {
	recorder := voiceEchoRecorder{silence: 30 * time.Millisecond}
	received := make(chan struct {
		packet []byte
		at     time.Time
	}, 2)
	send := func(packet []byte) bool {
		received <- struct {
			packet []byte
			at     time.Time
		}{append([]byte(nil), packet...), time.Now()}
		return true
	}

	recorder.enqueue([]byte{1}, 20*time.Millisecond, send)
	time.Sleep(10 * time.Millisecond)
	recorder.enqueue([]byte{2}, 20*time.Millisecond, send)

	select {
	case <-received:
		t.Fatal("voice echo started before the transmission ended")
	case <-time.After(15 * time.Millisecond):
	}

	var first, second struct {
		packet []byte
		at     time.Time
	}
	select {
	case first = <-received:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for first recorded packet")
	}
	select {
	case second = <-received:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for second recorded packet")
	}

	if !bytes.Equal(first.packet, []byte{1}) || !bytes.Equal(second.packet, []byte{2}) {
		t.Fatalf("playback order = %v, %v", first.packet, second.packet)
	}
	if spacing := second.at.Sub(first.at); spacing < 15*time.Millisecond {
		t.Fatalf("playback spacing = %v, want about 20ms", spacing)
	}
}

func TestVoiceEchoPacketDuration(t *testing.T) {
	if got := voiceEchoPacketDuration(&NRL21packet{Type: 8, DATA: []byte{1}}); got != 20*time.Millisecond {
		t.Fatalf("Opus duration = %v, want 20ms", got)
	}

	for _, test := range []struct {
		payloadBytes int
		want         time.Duration
	}{
		{payloadBytes: 1, want: 125 * time.Microsecond},
		{payloadBytes: 160, want: 20 * time.Millisecond},
		{payloadBytes: 320, want: 40 * time.Millisecond},
		{payloadBytes: 500, want: 62500 * time.Microsecond},
		{payloadBytes: 800, want: 100 * time.Millisecond},
	} {
		nrl := &NRL21packet{Type: 1, DATA: make([]byte, test.payloadBytes)}
		if got := voiceEchoPacketDuration(nrl); got != test.want {
			t.Errorf("G.711 %d-byte duration = %v, want %v", test.payloadBytes, got, test.want)
		}
	}
}

func TestConnPoolIsOnlyDeviceUsesPoolMembership(t *testing.T) {
	dev := &deviceInfo{ISOnline: false}
	pool := &currentConnPool{
		devConnMap:  map[string]*deviceInfo{"device": dev},
		devConnList: []*deviceInfo{dev},
	}

	if !pool.isOnlyDevice(dev) {
		t.Fatal("single connection-pool member was not treated as the only device")
	}
	pool.devConnMap["other"] = &deviceInfo{}
	if pool.isOnlyDevice(dev) {
		t.Fatal("device was treated as alone with two connection-pool members")
	}
}

func TestSingleDeviceVoiceEchoRoomExceptions(t *testing.T) {
	tests := []struct {
		name string
		gp   *group
		want bool
	}{
		{name: "normal room", gp: &group{ID: 100, Type: 0}, want: true},
		{name: "relay room", gp: &group{ID: 100, Type: 1}, want: false},
		{name: "full network room", gp: &group{ID: 999, Type: 0}, want: false},
		{name: "missing room", gp: nil, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := singleDeviceVoiceEchoEnabled(test.gp); got != test.want {
				t.Fatalf("singleDeviceVoiceEchoEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestVoiceEchoNewTransmissionInterruptsPlayback(t *testing.T) {
	recorder := voiceEchoRecorder{silence: 20 * time.Millisecond}
	received := make(chan byte, 3)
	send := func(packet []byte) bool {
		received <- packet[0]
		return true
	}

	recorder.enqueue([]byte{1}, 80*time.Millisecond, send)
	recorder.enqueue([]byte{2}, 80*time.Millisecond, send)
	select {
	case got := <-received:
		if got != 1 {
			t.Fatalf("first playback packet = %d, want 1", got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for playback to start")
	}

	// A new PTT transmission starts while the old recording is between frames.
	recorder.enqueue([]byte{3}, 20*time.Millisecond, send)
	select {
	case got := <-received:
		if got != 3 {
			t.Fatalf("packet after new transmission = %d, want 3", got)
		}
	case <-time.After(150 * time.Millisecond):
		t.Fatal("timed out waiting for the new recording")
	}

	select {
	case got := <-received:
		t.Fatalf("stale playback continued with packet %d", got)
	case <-time.After(30 * time.Millisecond):
	}
}
