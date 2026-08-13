package main

import (
	"bytes"
	"net"
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

func TestVoiceEchoOnlyEnabledForParrotRoom(t *testing.T) {
	tests := []struct {
		name string
		gp   *group
		want bool
	}{
		{name: "normal room", gp: &group{ID: 100, Type: 0}, want: false},
		{name: "relay room", gp: &group{ID: 100, Type: 1}, want: false},
		{name: "full network room", gp: &group{ID: 999, Type: 0}, want: false},
		{name: "parrot room", gp: &group{ID: voiceEchoRoomID, Type: 0}, want: true},
		{name: "missing room", gp: nil, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := voiceEchoRoomEnabled(test.gp); got != test.want {
				t.Fatalf("voiceEchoRoomEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestVoiceEchoRoomIsPublicBuiltInRoom(t *testing.T) {
	for _, groupID := range []int{0, voiceEchoRoomID, 999, 1000} {
		if !isPublicGroupID(groupID) {
			t.Errorf("room %d was not classified as public", groupID)
		}
	}
	for _, groupID := range []int{privateGroupMinID, 2, privateGroupMaxID} {
		if isPublicGroupID(groupID) {
			t.Errorf("private room %d was classified as public", groupID)
		}
		if !isPrivateGroupID(groupID) {
			t.Errorf("room %d was not classified as private", groupID)
		}
	}
	for _, groupID := range []int{0, voiceEchoRoomID, 999} {
		if isPrivateGroupID(groupID) {
			t.Errorf("public room %d was classified as private", groupID)
		}
	}
	for _, groupID := range []int{reservedGroupMinID, reservedGroupMaxID} {
		if !isReservedGroupID(groupID) {
			t.Errorf("room %d was not classified as reserved", groupID)
		}
		if isPrivateGroupID(groupID) || isPublicGroupID(groupID) {
			t.Errorf("reserved room %d was assigned to another range", groupID)
		}
	}
	if isPrivateGroupID(997) || isReservedGroupID(997) || isPublicGroupID(997) {
		t.Fatal("unassigned room 997 was classified into a room range")
	}
}

func TestVoiceEchoPlaybackAllowsEachDeviceInMultiDeviceParrotRoom(t *testing.T) {
	addrA := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 10001}
	addrB := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 10002}
	devA := &deviceInfo{}
	devB := &deviceInfo{}
	gp := &group{
		ID: voiceEchoRoomID,
		connPool: &currentConnPool{devConnMap: map[string]*deviceInfo{
			addrA.String(): devA,
			addrB.String(): devB,
		}},
	}

	if !voiceEchoPlaybackAllowed(gp, devA, addrA) {
		t.Fatal("first device was not allowed to replay with multiple devices online")
	}
	if !voiceEchoPlaybackAllowed(gp, devB, addrB) {
		t.Fatal("second device was not allowed to replay with multiple devices online")
	}
	if voiceEchoPlaybackAllowed(gp, devA, addrB) {
		t.Fatal("device was allowed to replay to another device's address")
	}
	gp.ID = 1000
	if voiceEchoPlaybackAllowed(gp, devA, addrA) {
		t.Fatal("normal room allowed recorded playback")
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
