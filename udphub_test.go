package main

import (
	"bytes"
	"errors"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConnPoolEnsureDeviceReplacesOldNATAddress(t *testing.T) {
	pool := &currentConnPool{devConnMap: make(map[string]*deviceInfo)}
	devA := &deviceInfo{CallSign: "BG1AAA", SSID: 1}
	devB := &deviceInfo{CallSign: "BG1BBB", SSID: 1}

	if !pool.ensureDevice("192.0.2.1:10001", devA) {
		t.Fatal("first registration of devA should return true")
	}
	if !pool.ensureDevice("192.0.2.2:10002", devB) {
		t.Fatal("first registration of devB should return true")
	}
	if pool.count() != 2 {
		t.Fatalf("count = %d, want 2", pool.count())
	}

	before := pool.devices()

	// devA changes NAT source port; old entry must be evicted immediately.
	if !pool.ensureDevice("192.0.2.1:20001", devA) {
		t.Fatal("re-registration with new port should return true")
	}
	if pool.count() != 2 {
		t.Fatalf("count after NAT port change = %d, want 2", pool.count())
	}
	if _, ok := pool.getDevice("192.0.2.1:10001"); ok {
		t.Fatal("stale NAT address was not removed")
	}
	if got, ok := pool.getDevice("192.0.2.1:20001"); !ok || got != devA {
		t.Fatal("new NAT address did not map to devA")
	}
	// Idempotent re-registration should return false.
	if pool.ensureDevice("192.0.2.1:20001", devA) {
		t.Fatal("duplicate registration should return false")
	}
	// COW snapshot taken before the update must remain intact.
	if len(before) != 2 {
		t.Fatalf("previous snapshot length changed to %d", len(before))
	}
}

func TestConnPoolAcquireCtlAtomicWindow(t *testing.T) {
	pool := &currentConnPool{}
	start := time.Now()
	addrA := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5001}
	addrB := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5002}

	if !pool.acquireCtl(addrA, addrA.String(), start) {
		t.Fatal("first controller did not acquire ctl")
	}
	if pool.acquireCtl(addrB, addrB.String(), start.Add(199*time.Millisecond)) {
		t.Fatal("second controller preempted ctl within 200ms")
	}
	if !pool.acquireCtl(addrA, addrA.String(), start.Add(150*time.Millisecond)) {
		t.Fatal("current controller could not refresh ctl")
	}
	if !pool.acquireCtl(addrB, addrB.String(), start.Add(350*time.Millisecond)) {
		t.Fatal("second controller could not acquire ctl after 200ms idle")
	}
}

func TestForwardVoiceDoesNotMutateMutedPeerTimestampAndDecodesLazily(t *testing.T) {
	serverConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer serverConn.Close()

	peerConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer peerConn.Close()

	senderAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 41001}
	mutedAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 41002}
	peerAddr := peerConn.LocalAddr().(*net.UDPAddr)

	sender := &deviceInfo{CallSign: "BG1SND", SSID: 1, DevModel: 101, Priority: 100}
	sender.setUDPAddr(senderAddr)

	origMutedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	muted := &deviceInfo{CallSign: "BG1MUT", SSID: 1, DevModel: 101, Status: 2, LastVoiceEndTime: origMutedTime}
	muted.setUDPAddr(mutedAddr)

	peer := &deviceInfo{CallSign: "BG1RCV", SSID: 1, DevModel: 101}
	peer.setUDPAddr(peerAddr)

	gp := &group{
		ID:       1001,
		Name:     "test-room",
		Type:     0,
		connPool: &currentConnPool{devConnMap: make(map[string]*deviceInfo)},
	}
	gp.connPool.ensureDevice(senderAddr.String(), sender)
	gp.connPool.ensureDevice(mutedAddr.String(), muted)
	gp.connPool.ensureDevice(peerAddr.String(), peer)

	rawPacket := encodeNRL21(sender.CallSign, sender.SSID, 8, 101, 12345, []byte{9 << 3})
	nrl, err := newNRL21packet(senderAddr, rawPacket)
	if err != nil {
		t.Fatal(err)
	}

	forwardVoice(nrl, sender, rawPacket, serverConn, gp)

	// No WS audio subscriber and no legacy G.711 recipient -> Opus must not be decoded.
	if nrl.webPCMDecoded {
		t.Fatal("Opus packet was eagerly decoded despite having no PCM consumer")
	}
	// Muted peer's LastVoiceEndTime must not be overwritten.
	if !muted.LastVoiceEndTime.Equal(origMutedTime) {
		t.Fatalf("muted peer LastVoiceEndTime changed to %v, want %v", muted.LastVoiceEndTime, origMutedTime)
	}

	buf := make([]byte, 2048)
	_ = peerConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	n, _, err := peerConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("peer did not receive forwarded packet: %v", err)
	}
	if !bytes.Equal(buf[:n], rawPacket) {
		t.Fatal("forwarded Opus packet payload differed from original")
	}

	// Now switch peer to a legacy G.711 device and verify lazy decoding triggers transcoding.
	peer.DevModel = 17
	nrlLegacy, err := newNRL21packet(senderAddr, rawPacket)
	if err != nil {
		t.Fatal(err)
	}
	forwardVoice(nrlLegacy, sender, rawPacket, serverConn, gp)

	if !nrlLegacy.webPCMDecoded {
		t.Fatal("Opus packet was not decoded for legacy G.711 recipient")
	}
	_ = peerConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	n, _, err = peerConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("legacy peer did not receive transcoded packet: %v", err)
	}
	if buf[20] != 1 {
		t.Fatalf("legacy recipient packet type = %d, want 1 (G.711)", buf[20])
	}
}

func TestEnqueueCallLogUsesSnapshotAndDoesNotBlock(t *testing.T) {
	oldBuf := logbuffer
	defer func() { logbuffer = oldBuf }()

	logbuffer = make(chan callLogEntry, 1)
	ts := time.Date(2026, 11, 21, 10, 20, 30, 0, time.UTC)
	dev := &deviceInfo{
		ID:                42,
		CallSign:          "BG1LOG",
		SSID:              7,
		GroupID:           1000,
		LastVoiceEndTime:  ts,
		LastVoiceDuration: 3500,
	}

	enqueueCallLog(dev)
	// Mutate device after enqueue; snapshot must not change.
	dev.GroupID = 999
	dev.LastVoiceDuration = 99000

	// Second enqueue on a full buffer must return immediately without blocking.
	enqueueCallLog(dev)

	entry := <-logbuffer
	if entry.GroupID != 1000 || entry.DurationMS != 3500 {
		t.Fatalf("snapshot mutated: got group=%d duration=%d", entry.GroupID, entry.DurationMS)
	}
	want := "2026-11-21 10:20:30,BG1LOG-7,42,1000,4"
	if got := entry.String(); got != want {
		t.Fatalf("entry.String() = %q, want %q", got, want)
	}
}

func TestAppendPacketSuffixDoesNotOverwriteBackingArray(t *testing.T) {
	backing := make([]byte, 8, 32)
	copy(backing, []byte("NRL2TEST"))
	sentinel := backing[:16]
	copy(sentinel[8:], []byte("SENTINEL"))

	resp := appendPacketSuffix(backing, "1000,ok")
	if string(resp) != "NRL2TEST1000,ok" {
		t.Fatalf("resp = %q, want %q", resp, "NRL2TEST1000,ok")
	}
	if string(sentinel[8:]) != "SENTINEL" {
		t.Fatalf("backing buffer was overwritten: %q", sentinel[8:])
	}
}

func TestDecodeNRL21TrimsSpacePaddedCallsign(t *testing.T) {
	// Exact wire bytes from the reported heartbeat with space-padded 5-char callsign "TY2TM ".
	raw := []byte{
		0x4E, 0x52, 0x4C, 0x32, 0x00, 0x30, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x02, 0x01, 0x09, 0x17,
		0x54, 0x59, 0x32, 0x54, 0x4D, 0x20, // "TY2TM "
		0xC8, 0xC8,
		0x42, 0x47, 0x31, 0x41, 0x42, 0x20, // OrigCallsign "BG1AB "
		0x01, 0x7F, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00,
	}

	addr := &net.UDPAddr{IP: net.IPv4(58, 240, 17, 226), Port: 47428}
	pkt, err := newNRL21packet(addr, raw)
	if err != nil {
		t.Fatalf("newNRL21packet() error = %v", err)
	}
	if pkt.CallSign != "TY2TM" {
		t.Fatalf("CallSign = %q, want %q", pkt.CallSign, "TY2TM")
	}
	if pkt.OriginalCallsign != "BG1AB" {
		t.Fatalf("OriginalCallsign = %q, want %q", pkt.OriginalCallsign, "BG1AB")
	}

	// All-space callsign must still be rejected.
	blank := append([]byte(nil), raw...)
	copy(blank[24:30], []byte("      "))
	if _, err := newNRL21packet(addr, blank); err == nil {
		t.Fatal("newNRL21packet() accepted an all-space callsign")
	}
}

func TestLogDecodeErrRateLimitsPerIP(t *testing.T) {
	oldMap := decodeErrLogs
	oldOut := log.Writer()
	defer func() {
		decodeErrLogs = oldMap
		log.SetOutput(oldOut)
	}()

	decodeErrLogs = make(map[string]*decodeErrLogState)
	var buf bytes.Buffer
	log.SetOutput(&buf)

	addr := &net.UDPAddr{IP: net.IPv4(121, 227, 174, 212), Port: 60708}
	t0 := time.Date(2026, 11, 21, 21, 48, 44, 0, time.UTC)

	logDecodeErr(addr, errors.New("callsign error"), []byte{0x4E, 0x52}, t0)
	logDecodeErr(addr, errors.New("callsign error"), []byte{0x4E, 0x52}, t0.Add(1*time.Second))
	logDecodeErr(addr, errors.New("callsign error"), []byte{0x4E, 0x52}, t0.Add(2*time.Second))

	if lines := strings.Count(buf.String(), "\n"); lines != 1 {
		t.Fatalf("expected 1 log line within interval, got %d: %q", lines, buf.String())
	}

	logDecodeErr(addr, errors.New("callsign error"), []byte{0x4E, 0x52}, t0.Add(decodeErrLogInterval))
	out := buf.String()
	if lines := strings.Count(out, "\n"); lines != 2 {
		t.Fatalf("expected 2 log lines after interval, got %d: %q", lines, out)
	}
	if !strings.Contains(out, "suppressed 2") {
		t.Fatalf("expected suppressed count in second log line, got %q", out)
	}
}

func TestForwardServerVoiceRoutingAndNonNilPacket(t *testing.T) {
	serverConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer serverConn.Close()

	listenPeer := func() (*net.UDPConn, *net.UDPAddr) {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		return c, c.LocalAddr().(*net.UDPAddr)
	}

	peerNormalConn, peerNormalAddr := listenPeer()
	peer200Conn, peer200Addr := listenPeer()
	peer255Conn, peer255Addr := listenPeer()

	peerNormal := &deviceInfo{CallSign: "BG1NOR", SSID: 1, DevModel: 101}
	peerNormal.setUDPAddr(peerNormalAddr)

	peer200 := &deviceInfo{CallSign: "BG1SRV", SSID: 200, DevModel: 200}
	peer200.setUDPAddr(peer200Addr)

	peer255 := &deviceInfo{CallSign: "BG1NET", SSID: 255, DevModel: 255}
	peer255.setUDPAddr(peer255Addr)

	senderAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42001}
	sender255 := &deviceInfo{CallSign: "BG2NET", SSID: 255, DevModel: 255, Priority: 100}
	sender255.setUDPAddr(senderAddr)

	gp := &group{
		ID:       999,
		Name:     "fullnet",
		Type:     0,
		connPool: &currentConnPool{devConnMap: make(map[string]*deviceInfo)},
	}
	gp.connPool.ensureDevice(senderAddr.String(), sender255)
	gp.connPool.ensureDevice(peerNormalAddr.String(), peerNormal)
	gp.connPool.ensureDevice(peer200Addr.String(), peer200)
	gp.connPool.ensureDevice(peer255Addr.String(), peer255)

	// Case 1: 255 voice -> forwarded only to normal peer, blocked for 255 and 200.
	base := encodeNRL21("BG2NET", 255, 1, 255, 1001, bytes.Repeat([]byte{0xd5}, 160))
	pkt255 := NRL21replace200and255dev("BG2NET", 255, 1, 255, "B1AA", 7, net.IPv4(1, 2, 3, 4).To4(), 1001, base)
	nrl255, err := newNRL21packet(senderAddr, pkt255)
	if err != nil {
		t.Fatal(err)
	}
	forwardServerVoice(nrl255, sender255, pkt255, serverConn, gp)

	buf := make([]byte, 2048)
	_ = peerNormalConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	n, _, err := peerNormalConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("normal peer did not receive 255 voice: %v", err)
	}
	decoded, err := newNRL21packet(senderAddr, buf[:n])
	if err != nil {
		t.Fatalf("decode forwarded packet failed: %v", err)
	}
	if decoded.CallSign != "B1AA" || decoded.SSID != 7 {
		t.Fatalf("forwarded header = %s-%d, want B1AA-7", decoded.CallSign, decoded.SSID)
	}

	_ = peer200Conn.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
	if _, _, err := peer200Conn.ReadFromUDP(buf); err == nil {
		t.Fatal("255 voice was unexpectedly forwarded to 200 device")
	}
	_ = peer255Conn.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
	if _, _, err := peer255Conn.ReadFromUDP(buf); err == nil {
		t.Fatal("255 voice was unexpectedly forwarded to 255 device")
	}

	// Case 2: SSID=200 with DevModel=0 (previously produced nil newpacket) ->
	// must forward to normal peer and 200 peer, and still block 255 peer.
	senderSSID200Addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42002}
	senderSSID200 := &deviceInfo{CallSign: "BG3SRV", SSID: 200, DevModel: 0, Priority: 100}
	senderSSID200.setUDPAddr(senderSSID200Addr)
	gp.connPool.ensureDevice(senderSSID200Addr.String(), senderSSID200)

	base200 := encodeNRL21("BG3SRV", 200, 1, 0, 2002, bytes.Repeat([]byte{0xd5}, 160))
	pktSSID200 := NRL21replace200and255dev("BG3SRV", 200, 1, 0, "BY1A", 3, net.IPv4(5, 6, 7, 8).To4(), 2002, base200)
	nrlSSID200, err := newNRL21packet(senderSSID200Addr, pktSSID200)
	if err != nil {
		t.Fatal(err)
	}
	// Advance timestamp past 400ms voice grace period.
	nrlSSID200.timeStamp = nrl255.timeStamp.Add(500 * time.Millisecond)
	forwardServerVoice(nrlSSID200, senderSSID200, pktSSID200, serverConn, gp)

	_ = peerNormalConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	n, _, err = peerNormalConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("normal peer did not receive SSID=200 voice: %v", err)
	}
	decoded, err = newNRL21packet(senderSSID200Addr, buf[:n])
	if err != nil {
		t.Fatalf("decode forwarded SSID=200 packet failed: %v", err)
	}
	if decoded.CallSign != "BY1A" || decoded.SSID != 3 {
		t.Fatalf("forwarded SSID=200 header = %s-%d, want BY1A-3", decoded.CallSign, decoded.SSID)
	}

	_ = peer200Conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, _, err := peer200Conn.ReadFromUDP(buf); err != nil {
		t.Fatalf("200 peer did not receive 200 voice: %v", err)
	}

	_ = peer255Conn.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
	if _, _, err := peer255Conn.ReadFromUDP(buf); err == nil {
		t.Fatal("200 voice was unexpectedly forwarded to 255 device")
	}
}

func TestNRL21replace200and255devClearsShortCallsigns(t *testing.T) {
	orig := encodeNRL21("BG1AAA", 1, 1, 200, 123, []byte{0x01})
	copy(orig[32:38], "BG2BBB")

	out := NRL21replace200and255dev("B1AA", 2, 1, 200, "BY1A", 3, net.IPv4(10, 0, 0, 1).To4(), 456, orig)
	if !bytes.Equal(out[24:30], []byte{'B', '1', 'A', 'A', 0, 0}) {
		t.Fatalf("callsign bytes = % X, want trailing zeros cleared", out[24:30])
	}
	if !bytes.Equal(out[32:38], []byte{'B', 'Y', '1', 'A', 0, 0}) {
		t.Fatalf("original callsign bytes = % X, want trailing zeros cleared", out[32:38])
	}
}

func TestDeviceSnapshotDoesNotMutateSharedDeviceParmAndIsRaceFree(t *testing.T) {
	dev := &deviceInfo{
		ID:       10,
		CallSign: "BG1PRM",
		SSID:     1,
		GroupID:  1000,
	}
	dev.setDeviceParm(&control{CallSign: "BG1PRM"})

	hidden := dev.snapshotJSON(true)
	if hidden.DeviceParm != nil {
		t.Fatal("expected hidden snapshot DeviceParm to be nil")
	}
	if dev.getDeviceParm() == nil {
		t.Fatal("snapshotJSON(true) mutated shared deviceInfo.DeviceParm")
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			dev.recordPacket(time.Now(), 100)
			dev.setOnline(i%2 == 0)
			dev.setGroupID(1000 + i%3)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if _, err := jsonextra.Marshal(dev); err != nil {
				t.Errorf("Marshal(dev) failed: %v", err)
				return
			}
		}
	}()
	wg.Wait()
}

func TestPlatformListConcurrentReadWrite(t *testing.T) {
	oldList := getPlatformList()
	defer setPlatformList(oldList)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			setPlatformList([]Platformitem{{Name: "srv", Host: "127.0.0.1", Port: "60050", Online: i}})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			for _, item := range getPlatformList() {
				_ = item.Host
			}
		}
	}()
	wg.Wait()
}

func TestUDPRoomDispatcherShardConsistency(t *testing.T) {
	d := &udpRoomDispatcher{shards: make([]chan udpPacketTask, 8)}
	devA := &deviceInfo{CallSign: "BG1AAA", SSID: 1}
	devB := &deviceInfo{CallSign: "BG2BBB", SSID: 2}

	// Public room: all devices in room 1005 map to the same shard.
	if d.shardIndex(devA, 1005) != d.shardIndex(devB, 1005) {
		t.Fatal("devices in the same public room mapped to different shards")
	}
	// Private room: same user & room always maps to the same shard.
	if d.shardIndex(devA, 1) != d.shardIndex(&deviceInfo{CallSign: "BG1AAA", SSID: 9}, 1) {
		t.Fatal("same user's devices in the same private room mapped to different shards")
	}
}

func TestConnPoolAndDeviceAddrConcurrentAccess(t *testing.T) {
	pool := &currentConnPool{devConnMap: make(map[string]*deviceInfo)}
	dev := &deviceInfo{CallSign: "BG1CON", SSID: 1}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 30000 + id*1000 + j}
				dev.setUDPAddr(addr)
				pool.ensureDevice(addr.String(), dev)
				for _, d := range pool.devices() {
					_, _ = d.getUDPAddr()
				}
				_ = pool.count()
			}
		}(i)
	}
	wg.Wait()
}
