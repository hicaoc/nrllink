package main

import (
	"bytes"
	"testing"
)

func TestNRLVoiceToPCMDecodesALaw(t *testing.T) {
	want := bytes.Repeat([]byte{0xd5}, 160)
	nrl := &NRL21packet{Type: 1, DATA: want}

	got, err := nrlVoiceToPCM(&deviceInfo{}, nrl)
	if err != nil {
		t.Fatalf("nrlVoiceToPCM() error = %v", err)
	}
	if encoded := pcmToG711(got); !bytes.Equal(encoded, want) {
		t.Fatal("G.711 payload changed after PCM round trip")
	}
}

func TestNRLVoiceToPCMDecodes20msOpus(t *testing.T) {
	// A one-byte SILK wideband, mono, 20 ms Opus packet. The range decoder
	// treats the omitted payload bits as zero, producing a valid quiet frame.
	nrl := &NRL21packet{
		Type:     8,
		CallSign: "N0CALL",
		SSID:     1,
		DATA:     []byte{9 << 3},
	}

	got, err := nrlVoiceToPCM(&deviceInfo{}, nrl)
	if err != nil {
		t.Fatalf("nrlVoiceToPCM() error = %v", err)
	}
	if len(got) != wsCallAudioFrameSize {
		t.Fatalf("decoded PCM frame length = %d, want %d", len(got), wsCallAudioFrameSize)
	}
}

func TestOpusRelaySpeakersUseSeparateDecoders(t *testing.T) {
	dev := &deviceInfo{}
	first := &NRL21packet{Type: 8, OriginalCallsign: "N0AAA", OriginalSSID: 1, DATA: []byte{9 << 3}}
	second := &NRL21packet{Type: 8, OriginalCallsign: "N0BBB", OriginalSSID: 1, DATA: []byte{9 << 3}}

	if _, err := nrlVoiceToPCM(dev, first); err != nil {
		t.Fatalf("first speaker decode error = %v", err)
	}
	if _, err := nrlVoiceToPCM(dev, second); err != nil {
		t.Fatalf("second speaker decode error = %v", err)
	}
	if len(dev.opusDecoders) != 2 {
		t.Fatalf("decoder count = %d, want 2", len(dev.opusDecoders))
	}
}

func TestNRLVoiceToPCMRejectsEmptyOpus(t *testing.T) {
	nrl := &NRL21packet{Type: 8, CallSign: "N0CALL", SSID: 1}
	if _, err := nrlVoiceToPCM(&deviceInfo{}, nrl); err == nil {
		t.Fatal("nrlVoiceToPCM() accepted an empty Opus packet")
	}
}

func TestWSMixesPCMThenEncodesOnce(t *testing.T) {
	client := &wsCallClient{
		subscriptions: map[string]bool{"room-a": true, "room-b": true},
		audioBuffers: map[string][]int16{
			"room-a": make([]int16, wsCallAudioFrameSize),
			"room-b": make([]int16, wsCallAudioFrameSize),
		},
	}
	for i := 0; i < wsCallAudioFrameSize; i++ {
		client.audioBuffers["room-a"][i] = 1000
		client.audioBuffers["room-b"][i] = 2000
	}

	got := client.nextMixedFrame()
	if len(got) != wsCallAudioFrameSize {
		t.Fatalf("mixed frame length = %d, want %d", len(got), wsCallAudioFrameSize)
	}
	if got[0] != Linear2Alaw(3000) {
		t.Fatalf("mixed sample = %#x, want %#x", got[0], Linear2Alaw(3000))
	}
}
