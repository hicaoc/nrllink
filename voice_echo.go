package main

import (
	"log"
	"net"
	"sync"
	"time"
)

const (
	voiceEchoEndSilence = 200 * time.Millisecond
	voiceEchoMaxBytes   = 8 << 20
)

type voiceEchoFrame struct {
	packet   []byte
	duration time.Duration
}

type voiceEchoSession struct {
	frames     []voiceEchoFrame
	send       func([]byte) bool
	generation uint64
}

// voiceEchoRecorder buffers one PTT transmission and plays it only after the
// sender has been silent for voiceEchoEndSilence. Its zero value is ready to use.
type voiceEchoRecorder struct {
	mu         sync.Mutex
	silence    time.Duration
	timer      *time.Timer
	generation uint64
	current    voiceEchoSession
	bytes      int
	discarding bool
	pending    []voiceEchoSession
	running    bool
}

func (r *voiceEchoRecorder) enqueue(packet []byte, duration time.Duration, send func([]byte) bool) {
	if len(packet) == 0 || send == nil {
		return
	}
	if duration <= 0 {
		duration = 20 * time.Millisecond
	}

	packetCopy := append([]byte(nil), packet...)

	r.mu.Lock()
	if !r.discarding {
		if len(r.current.frames) == 0 {
			// Starting a new PTT transmission makes queued older recordings stale.
			r.pending = nil
			r.current.send = send
		}
		if r.bytes+len(packetCopy) > voiceEchoMaxBytes {
			r.current = voiceEchoSession{}
			r.bytes = 0
			r.discarding = true
			log.Printf("voice echo recording discarded after exceeding %d bytes", voiceEchoMaxBytes)
		} else {
			r.current.frames = append(r.current.frames, voiceEchoFrame{
				packet:   packetCopy,
				duration: duration,
			})
			r.bytes += len(packetCopy)
		}
	}

	r.generation++
	generation := r.generation
	if r.timer != nil {
		r.timer.Stop()
	}
	silence := r.silence
	if silence <= 0 {
		silence = voiceEchoEndSilence
	}
	r.timer = time.AfterFunc(silence, func() {
		r.finishRecording(generation)
	})
	r.mu.Unlock()
}

func (r *voiceEchoRecorder) finishRecording(generation uint64) {
	r.mu.Lock()
	if generation != r.generation {
		r.mu.Unlock()
		return
	}
	r.timer = nil
	if r.discarding {
		r.discarding = false
	} else if len(r.current.frames) != 0 {
		r.current.generation = generation
		r.pending = append(r.pending, r.current)
	}
	r.current = voiceEchoSession{}
	r.bytes = 0
	if r.running || len(r.pending) == 0 {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	go r.play()
}

func (r *voiceEchoRecorder) play() {
	for {
		r.mu.Lock()
		// A new PTT transmission takes precedence over an old recording.
		if len(r.current.frames) != 0 || r.discarding {
			r.running = false
			r.mu.Unlock()
			return
		}
		if len(r.pending) == 0 {
			r.running = false
			r.mu.Unlock()
			return
		}
		session := r.pending[0]
		r.pending[0] = voiceEchoSession{}
		r.pending = r.pending[1:]
		r.mu.Unlock()

		for i, frame := range session.frames {
			r.mu.Lock()
			interrupted := r.generation != session.generation
			r.mu.Unlock()
			if interrupted || !session.send(frame.packet) {
				break
			}
			if i+1 < len(session.frames) {
				time.Sleep(frame.duration)
			}
		}
	}
}

func voiceEchoPacketDuration(nrl *NRL21packet) time.Duration {
	if nrl == nil {
		return 20 * time.Millisecond
	}
	if nrl.Type == 1 && len(nrl.DATA) != 0 {
		// G.711 A-law is always 8 kHz / 8 bits, so every payload byte is one
		// sample (125 us). Do not assume a fixed packet size: deployed devices
		// use both 160-byte frames and legacy 500-byte frames, among others.
		return time.Duration(len(nrl.DATA)) * time.Second / 8000
	}
	return 20 * time.Millisecond
}

// voiceEchoRoomEnabled restricts recorded echo to the built-in parrot room 998.
func voiceEchoRoomEnabled(gp *group) bool {
	return gp != nil && gp.ID == voiceEchoRoomID
}

func cloneUDPAddr(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	return &net.UDPAddr{
		IP:   append(net.IP(nil), addr.IP...),
		Port: addr.Port,
		Zone: addr.Zone,
	}
}

func voiceEchoPlaybackAllowed(gp *group, dev *deviceInfo, addr *net.UDPAddr) bool {
	if !voiceEchoRoomEnabled(gp) || dev == nil || addr == nil || gp.connPool == nil {
		return false
	}
	current, ok := gp.connPool.getDevice(addr.String())
	return ok && current == dev
}

func queueDeviceVoiceEcho(gp *group, dev *deviceInfo, nrl *NRL21packet, packet []byte) {
	if !voiceEchoRoomEnabled(gp) || dev == nil || nrl == nil {
		return
	}
	addr := cloneUDPAddr(nrl.UDPAddr)
	dev.voiceEcho.enqueue(packet, voiceEchoPacketDuration(nrl), func(recordedPacket []byte) bool {
		if !voiceEchoPlaybackAllowed(gp, dev, addr) || globelconn == nil {
			return false
		}
		if _, err := globelconn.WriteToUDP(recordedPacket, addr); err != nil {
			log.Printf("voice echo playback failed for %s-%d: %v", dev.CallSign, dev.SSID, err)
			return false
		}
		return true
	})
}
