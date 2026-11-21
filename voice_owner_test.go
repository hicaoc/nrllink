package main

import (
	"testing"
	"time"
)

func TestAcquireVoiceSamePriority(t *testing.T) {
	pool := &currentConnPool{}
	start := time.Now()

	if !pool.acquireVoice("A-1", start, 100) {
		t.Fatal("first speaker did not acquire voice")
	}
	if pool.acquireVoice("B-1", start.Add(150*time.Millisecond), 100) {
		t.Fatal("same-priority speaker acquired voice during the owner's grace period")
	}
	if pool.acquireVoice("B-1", start.Add(399*time.Millisecond), 100) {
		t.Fatal("same-priority speaker acquired voice before 400ms")
	}
	if !pool.acquireVoice("B-1", start.Add(400*time.Millisecond), 100) {
		t.Fatal("same-priority speaker did not acquire voice after the owner was silent for 400ms")
	}
	if pool.acquireVoice("C-1", start.Add(401*time.Millisecond), 100) {
		t.Fatal("third speaker acquired voice immediately after the owner changed")
	}
}

func TestAcquireVoicePriority(t *testing.T) {
	pool := &currentConnPool{}
	start := time.Now()

	if !pool.acquireVoice("A-1", start, 100) {
		t.Fatal("first speaker did not acquire voice")
	}
	if !pool.acquireVoice("B-1", start.Add(20*time.Millisecond), 101) {
		t.Fatal("higher-priority speaker did not preempt immediately")
	}
	if pool.acquireVoice("A-1", start.Add(40*time.Millisecond), 100) {
		t.Fatal("lower-priority speaker preempted the current owner")
	}
	if !pool.acquireVoice("B-1", start.Add(60*time.Millisecond), 101) {
		t.Fatal("current owner could not continue speaking")
	}
}

func TestVoiceArbitrationAcrossLocalAndServerVoice(t *testing.T) {
	pool := &currentConnPool{}
	start := time.Now()

	localPacket := &NRL21packet{Type: 1, DevModel: 101}
	localDevice := &deviceInfo{CallSign: "LOCAL", SSID: 1, Priority: 100}
	relayDevice := &deviceInfo{CallSign: "RELAY", SSID: 200, Priority: 100}
	remoteA := &NRL21packet{
		Type: 1, DevModel: 200, UDPAddrStr: "192.0.2.1:6000",
		OriginalCallsign: "REMOTE", OriginalSSID: 1,
	}
	remoteB := &NRL21packet{
		Type: 1, DevModel: 200, UDPAddrStr: "192.0.2.1:6000",
		OriginalCallsign: "OTHER", OriginalSSID: 2,
	}

	if !pool.acquireVoice(voiceOwnerID(localPacket, localDevice), start, localDevice.Priority) {
		t.Fatal("local speaker did not acquire voice")
	}
	if pool.acquireVoice(voiceOwnerID(remoteA, relayDevice), start.Add(20*time.Millisecond), relayDevice.Priority) {
		t.Fatal("server voice preempted local voice during the grace period")
	}
	if pool.acquireVoice(voiceOwnerID(remoteA, relayDevice), start.Add(200*time.Millisecond), relayDevice.Priority) {
		t.Fatal("server voice acquired voice before 400ms")
	}
	if !pool.acquireVoice(voiceOwnerID(remoteA, relayDevice), start.Add(400*time.Millisecond), relayDevice.Priority) {
		t.Fatal("server voice did not acquire voice after 400ms")
	}
	if pool.acquireVoice(voiceOwnerID(remoteB, relayDevice), start.Add(420*time.Millisecond), relayDevice.Priority) {
		t.Fatal("different original speaker from the same relay bypassed arbitration")
	}

	remoteA.UDPAddrStr = "192.0.2.1:6001"
	if !pool.acquireVoice(voiceOwnerID(remoteA, relayDevice), start.Add(440*time.Millisecond), relayDevice.Priority) {
		t.Fatal("original speaker lost ownership after the relay UDP port changed")
	}
}

func TestVoiceOwnerIDFallsBackToRelayAddress(t *testing.T) {
	dev := &deviceInfo{CallSign: "RELAY", SSID: 200}
	packet := &NRL21packet{Type: 9, DevModel: 200, UDPAddrStr: "192.0.2.1:6000"}

	if got, want := voiceOwnerID(packet, dev), "relay:192.0.2.1:6000"; got != want {
		t.Fatalf("voice owner ID = %q, want %q", got, want)
	}
}
