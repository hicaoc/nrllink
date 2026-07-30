package main

import (
	"bytes"
	"encoding/binary"
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

func TestRequiresG711Voice(t *testing.T) {
	legacy := map[byte]bool{1: true, 2: true, 3: true, 8: true, 9: true, 17: true, 26: true, 35: true}
	for model := range byte(40) {
		if got := requiresG711Voice(model); got != legacy[model] {
			t.Errorf("requiresG711Voice(%d) = %v, want %v", model, got, legacy[model])
		}
	}
}

func TestVoicePacketForLegacyRecipientConvertsOpusToG711(t *testing.T) {
	pcm := make([]int16, 160)
	for i := range pcm {
		pcm[i] = int16(i*200 - 16000)
	}
	original := encodeNRL21("N0CALL", 4, 8, 101, 12345, []byte{0x48, 0x83, 0x92})
	original[21] = 7
	binary.BigEndian.PutUint16(original[22:24], 321)
	original[32] = 0xA5

	nrl := &NRL21packet{Type: 8, webPCM: pcm}
	converted, err := voicePacketForRecipient(nrl, original, &deviceInfo{DevModel: 17})
	if err != nil {
		t.Fatal(err)
	}
	if converted[20] != 1 {
		t.Fatalf("packet type = %d, want G.711 type 1", converted[20])
	}
	if got := int(binary.BigEndian.Uint16(converted[4:6])); got != len(converted) {
		t.Fatalf("packet length field = %d, actual %d", got, len(converted))
	}
	if converted[21] != original[21] || !bytes.Equal(converted[22:48], original[22:48]) {
		t.Fatal("transcoding changed NRL routing/header metadata")
	}
	if got, want := converted[48:], pcmToG711(pcm); !bytes.Equal(got, want) {
		t.Fatal("converted G.711 payload does not match decoded Opus PCM")
	}
	if original[20] != 8 || !bytes.Equal(original[48:], []byte{0x48, 0x83, 0x92}) {
		t.Fatal("transcoding mutated the original Opus packet")
	}
}

func TestVoicePacketForOpusRecipientKeepsOriginalPacket(t *testing.T) {
	original := encodeNRL21("N0CALL", 4, 8, 101, 12345, []byte{0x48})
	nrl := &NRL21packet{Type: 8, webPCM: make([]int16, 160)}
	got, err := voicePacketForRecipient(nrl, original, &deviceInfo{DevModel: 101})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || &got[0] != &original[0] {
		t.Fatal("Opus-capable recipient did not receive the original packet")
	}
}

func TestVoicePacketForLegacyRecipientRequiresDecodedPCM(t *testing.T) {
	original := encodeNRL21("N0CALL", 4, 8, 101, 12345, []byte{0x48})
	if _, err := voicePacketForRecipient(
		&NRL21packet{Type: 8},
		original,
		&deviceInfo{DevModel: 1},
	); err == nil {
		t.Fatal("legacy Opus forwarding succeeded without decoded PCM")
	}
}

func TestVoicePacketSelectorReusesLegacyConversion(t *testing.T) {
	original := encodeNRL21("N0CALL", 4, 8, 101, 12345, []byte{0x48})
	nrl := &NRL21packet{Type: 8, webPCM: make([]int16, 160)}
	selectPacket := voicePacketSelector(nrl, original)
	first, err := selectPacket(&deviceInfo{DevModel: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := selectPacket(&deviceInfo{DevModel: 35})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || &first[0] != &second[0] {
		t.Fatal("legacy recipients did not share the cached converted packet")
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
