package main

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pion/opus"
)

const (
	opusBrowserSampleRate = 8000
	opusBrowserChannels   = 1
	// Opus permits packets up to 120 ms. At 8 kHz that is 960 samples.
	opusMaxOutputSamples = opusBrowserSampleRate * 120 / 1000
	// Bound decoder state retained for speakers multiplexed by one relay device.
	opusMaxStreamsPerDevice = 256
)

// nrlOpusDecoder owns the state and scratch space for one logical Opus stream.
// Opus decoding is stateful, so packets from different speakers must not share it.
type nrlOpusDecoder struct {
	mu      sync.Mutex
	decoder opus.Decoder
	pcm     []int16
	lastUse time.Time
}

func newNRLOpusDecoder() (*nrlOpusDecoder, error) {
	decoder, err := opus.NewDecoderWithOutput(opusBrowserSampleRate, opusBrowserChannels)
	if err != nil {
		return nil, err
	}
	return &nrlOpusDecoder{
		decoder: decoder,
		pcm:     make([]int16, opusMaxOutputSamples),
	}, nil
}

func (d *nrlOpusDecoder) decodePCM(packet []byte, timestamp time.Time) ([]int16, error) {
	if len(packet) == 0 {
		return nil, errors.New("empty Opus packet")
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.lastUse.IsZero() && timestamp.Sub(d.lastUse) > time.Second {
		if err := d.decoder.Init(opusBrowserSampleRate, opusBrowserChannels); err != nil {
			return nil, err
		}
	}
	d.lastUse = timestamp

	sampleCount, err := d.decoder.DecodeToInt16(packet, d.pcm)
	if err != nil {
		return nil, err
	}
	if sampleCount <= 0 || sampleCount > len(d.pcm) {
		return nil, fmt.Errorf("invalid decoded sample count %d", sampleCount)
	}

	return append([]int16(nil), d.pcm[:sampleCount]...), nil
}

func (d *deviceInfo) opusDecoderFor(streamKey string) (*nrlOpusDecoder, error) {
	d.opusMu.Lock()
	defer d.opusMu.Unlock()

	if decoder := d.opusDecoders[streamKey]; decoder != nil {
		return decoder, nil
	}
	decoder, err := newNRLOpusDecoder()
	if err != nil {
		return nil, err
	}
	if d.opusDecoders == nil {
		d.opusDecoders = make(map[string]*nrlOpusDecoder)
	}
	if len(d.opusDecoders) >= opusMaxStreamsPerDevice {
		for key := range d.opusDecoders {
			delete(d.opusDecoders, key)
			break
		}
	}
	d.opusDecoders[streamKey] = decoder
	return decoder, nil
}

func opusStreamKey(nrl *NRL21packet) string {
	if nrl.OriginalCallsign != "" {
		return nrl.OriginalCallsign + "-" + strconv.Itoa(int(nrl.OriginalSSID))
	}
	return nrl.CallSign + "-" + strconv.Itoa(int(nrl.SSID))
}

// nrlVoiceToPCM returns 8 kHz mono PCM without changing the original NRL
// payload. Type 1 A-law is decoded; type 8 is decoded and resampled directly
// to the mixer's sample rate by the Opus decoder.
func nrlVoiceToPCM(dev *deviceInfo, nrl *NRL21packet) ([]int16, error) {
	if dev == nil || nrl == nil {
		return nil, errors.New("missing device or NRL packet")
	}
	switch nrl.Type {
	case 1:
		return g711ToPCM(nrl.DATA), nil
	case 8:
		decoder, err := dev.opusDecoderFor(opusStreamKey(nrl))
		if err != nil {
			return nil, err
		}
		return decoder.decodePCM(nrl.DATA, nrl.timeStamp)
	default:
		return nil, fmt.Errorf("unsupported voice type %d", nrl.Type)
	}
}

func g711ToPCM(g711 []byte) []int16 {
	pcm := make([]int16, len(g711))
	for i, sample := range g711 {
		pcm[i] = alaw2linear(sample)
	}
	return pcm
}

func pcmToG711(pcm []int16) []byte {
	g711 := make([]byte, len(pcm))
	for i, sample := range pcm {
		g711[i] = Linear2Alaw(sample)
	}
	return g711
}
