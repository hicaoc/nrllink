package main

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

//dev model:
/*
 { id: 100, name: 'NRL-微信小程序' },
  { id: 101, name: 'NRL-安卓' },
  { id: 102, name: 'NRL-IOS' },
  { id: 103, name: 'NRL-Win' },
  { id: 105, name: 'NRL-浏览器' },
  { id: 106, name: 'NRL-救援' },

  { id: 200, name: 'NRL-Server' },
  { id: 201, name: 'NRL-BM' },
  { id: 250, name: 'NRL-保姆' }
  { id: 255, name: 'FULL NET' }

*/

const (
	udpSocketBufferSize      = 4 * 1024 * 1024
	udpReadBufferSize        = 2048
	decodeErrLogInterval     = time.Minute
	decodeErrMaxTrackedAddrs = 1024
	udpShardQueueSize        = 2048
	newDevRegisterQueueSize  = 256
)

var udpPacketBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, udpReadBufferSize)
		return &buf
	},
}

func getUDPPacketBuf() *[]byte {
	return udpPacketBufPool.Get().(*[]byte)
}

func putUDPPacketBuf(bufPtr *[]byte) {
	if bufPtr == nil {
		return
	}
	if cap(*bufPtr) < udpReadBufferSize {
		return
	}
	*bufPtr = (*bufPtr)[:udpReadBufferSize]
	udpPacketBufPool.Put(bufPtr)
}

type udpPacketTask struct {
	nrl    *NRL21packet
	bufPtr *[]byte
	length int
	dev    *deviceInfo
	conn   *net.UDPConn
}

type udpRoomDispatcher struct {
	shards    []chan udpPacketTask
	dropCount atomic.Uint64
}

func newUDPRoomDispatcher(workerCount int) *udpRoomDispatcher {
	if workerCount <= 0 {
		workerCount = runtime.GOMAXPROCS(0)
	}
	if workerCount < 4 {
		workerCount = 4
	}
	if workerCount > 16 {
		workerCount = 16
	}
	d := &udpRoomDispatcher{
		shards: make([]chan udpPacketTask, workerCount),
	}
	for i := range d.shards {
		ch := make(chan udpPacketTask, udpShardQueueSize)
		d.shards[i] = ch
		go d.runWorker(ch)
	}
	return d
}

func (d *udpRoomDispatcher) shardIndex(dev *deviceInfo, groupID int) int {
	if isPrivateGroupID(groupID) && dev != nil {
		h := fnv.New32a()
		_, _ = h.Write([]byte(dev.CallSign))
		return int((uint64(h.Sum32()) + uint64(groupID)) % uint64(len(d.shards)))
	}
	if groupID < 0 {
		groupID = -groupID
	}
	return groupID % len(d.shards)
}

func (d *udpRoomDispatcher) dispatch(task udpPacketTask) {
	idx := d.shardIndex(task.dev, task.dev.getGroupID())
	select {
	case d.shards[idx] <- task:
	default:
		putUDPPacketBuf(task.bufPtr)
		if dropped := d.dropCount.Add(1); dropped == 1 || dropped%1000 == 0 {
			log.Printf("udp worker shard %d queue full, dropped=%d", idx, dropped)
		}
	}
}

func (d *udpRoomDispatcher) runWorker(ch <-chan udpPacketTask) {
	for task := range ch {
		handleKnownDevicePacket(task.nrl, (*task.bufPtr)[:task.length], task.dev, task.conn)
		putUDPPacketBuf(task.bufPtr)
	}
}

func (d *udpRoomDispatcher) close() {
	for _, ch := range d.shards {
		close(ch)
	}
}

type newDeviceTask struct {
	callsignSSID string
	nrl          *NRL21packet
	bufPtr       *[]byte
	length       int
	conn         *net.UDPConn
}

type newDeviceRegistrar struct {
	mu         sync.Mutex
	pending    map[string]struct{}
	queue      chan newDeviceTask
	dispatcher *udpRoomDispatcher
	dropCount  atomic.Uint64
}

func newNewDeviceRegistrar(dispatcher *udpRoomDispatcher) *newDeviceRegistrar {
	r := &newDeviceRegistrar{
		pending:    make(map[string]struct{}),
		queue:      make(chan newDeviceTask, newDevRegisterQueueSize),
		dispatcher: dispatcher,
	}
	go r.run()
	return r
}

func (r *newDeviceRegistrar) enqueue(task newDeviceTask) bool {
	r.mu.Lock()
	if _, exists := r.pending[task.callsignSSID]; exists {
		r.mu.Unlock()
		putUDPPacketBuf(task.bufPtr)
		return false
	}
	r.pending[task.callsignSSID] = struct{}{}
	r.mu.Unlock()

	select {
	case r.queue <- task:
		return true
	default:
		r.mu.Lock()
		delete(r.pending, task.callsignSSID)
		r.mu.Unlock()
		putUDPPacketBuf(task.bufPtr)
		if dropped := r.dropCount.Add(1); dropped == 1 || dropped%1000 == 0 {
			log.Printf("new device register queue full, dropped=%d", dropped)
		}
		return false
	}
}

func (r *newDeviceRegistrar) run() {
	for task := range r.queue {
		r.processTask(task)
	}
}

func (r *newDeviceRegistrar) processTask(task newDeviceTask) {
	defer func() {
		r.mu.Lock()
		delete(r.pending, task.callsignSSID)
		r.mu.Unlock()
	}()

	// 若在排队期间已被其他路径注册，直接走常规分发
	if existing, ok := getDevByCallsignSSID(task.callsignSSID); ok {
		r.forwardRegisteredPacket(existing, task)
		return
	}

	nrl := task.nrl
	dd := &deviceInfo{
		CallSignSSID: task.callsignSSID,
		CallSign:     nrl.CallSign,
		SSID:         nrl.SSID,
		//DMRID:        nrl.DMRID, // 过渡期，暂时不从nrl报文中获取DMRID，盒子不支持，当前从服务器配置中获取
		DevModel: nrl.DevModel,
		Priority: 100,
		ChanName: make([]string, 16),
	}

	if nrl.DevModel == 255 {
		dd.GroupID = 999
	}

	if err := addDevice(dd); err != nil {
		log.Println("add dev failed, ", err, '\n', nrl)
	}

	d, err := getDevice(nrl.CallSign, nrl.SSID)
	if err != nil {
		log.Printf("Failed to get device for server %v-%d: %v", nrl.CallSign, nrl.SSID, err)
		putUDPPacketBuf(task.bufPtr)
		return
	}

	log.Println("add dev ok", dd.CallSign, dd.SSID, d.GroupID, "groupid:", dd.GroupID, dd.CallSign, dd.SSID, d)

	d.pcmBuffer = make([]int, 1000)

	setDevByCallsignSSID(task.callsignSSID, d)
	if nrl.Type == 2 && refreshDeviceAccountExpired(d, nrl.timeStamp) {
		log.Printf("billing: expired account new device heartbeat ignored: %v-%v", d.CallSign, d.SSID)
		putUDPPacketBuf(task.bufPtr)
		return
	}

	if p, ok := getPublicGroup(d.GroupID); ok {
		p.setDevice(d.ID, d)
	} else if pg0, ok := getPublicGroup(0); ok {
		d.setGroupID(0)
		pg0.setDevice(d.ID, d)
	}

	r.forwardRegisteredPacket(d, task)
}

func (r *newDeviceRegistrar) forwardRegisteredPacket(dev *deviceInfo, task newDeviceTask) {
	if r.dispatcher != nil {
		r.dispatcher.dispatch(udpPacketTask{
			nrl:    task.nrl,
			bufPtr: task.bufPtr,
			length: task.length,
			dev:    dev,
			conn:   task.conn,
		})
		return
	}
	handleKnownDevicePacket(task.nrl, (*task.bufPtr)[:task.length], dev, task.conn)
	putUDPPacketBuf(task.bufPtr)
}

type decodeErrLogState struct {
	lastLog    time.Time
	suppressed uint64
}

var decodeErrLogs = make(map[string]*decodeErrLogState)

func logDecodeErr(remoteaddr *net.UDPAddr, err error, raw []byte, now time.Time) {
	key := ""
	if remoteaddr != nil {
		key = remoteaddr.IP.String()
	}

	st, ok := decodeErrLogs[key]
	if !ok {
		if len(decodeErrLogs) >= decodeErrMaxTrackedAddrs {
			for k, v := range decodeErrLogs {
				if now.Sub(v.lastLog) >= decodeErrLogInterval {
					delete(decodeErrLogs, k)
				}
			}
			if len(decodeErrLogs) >= decodeErrMaxTrackedAddrs {
				clear(decodeErrLogs)
			}
		}
		decodeErrLogs[key] = &decodeErrLogState{lastLog: now}
		log.Printf("from %v, decode err %v  % X:", remoteaddr, err, raw)
		return
	}

	if now.Sub(st.lastLog) < decodeErrLogInterval {
		st.suppressed++
		return
	}

	suppressed := st.suppressed
	st.lastLog = now
	st.suppressed = 0
	if suppressed > 0 {
		log.Printf("from %v, decode err %v (suppressed %d in last %v)  % X:", remoteaddr, err, suppressed, decodeErrLogInterval, raw)
	} else {
		log.Printf("from %v, decode err %v  % X:", remoteaddr, err, raw)
	}
}

var userlist sync.Map // callid ,userinfo

var mdcidmap sync.Map //key mdcid, value callsign

var dmridmap sync.Map //key dmrid, value callsign

var (
	devCallsignSSIDMap = make(map[string]*deviceInfo, 1000) //key : callsign+ssid 在线设备列表
	devMapMu           sync.RWMutex
)

var (
	onlinedevMap = make(map[int]*deviceInfo, 1000) //所有设备列表
	onlineDevPtr atomic.Pointer[map[int]*deviceInfo]
)

var (
	ServerMap   = make(map[string]*deviceInfo) //呼号对应的服务器设备
	serverMapMu sync.RWMutex
)

var globelconn *net.UDPConn

func getDevByCallsignSSID(key string) (*deviceInfo, bool) {
	devMapMu.RLock()
	dev, ok := devCallsignSSIDMap[key]
	devMapMu.RUnlock()
	return dev, ok
}

func setDevByCallsignSSID(key string, dev *deviceInfo) {
	devMapMu.Lock()
	devCallsignSSIDMap[key] = dev
	devMapMu.Unlock()
}

func deleteDevByCallsignSSID(key string) {
	devMapMu.Lock()
	delete(devCallsignSSIDMap, key)
	devMapMu.Unlock()
}

func countDevCallsignSSID() int {
	devMapMu.RLock()
	n := len(devCallsignSSIDMap)
	devMapMu.RUnlock()
	return n
}

func snapshotDevCallsignSSID() map[string]*deviceInfo {
	devMapMu.RLock()
	res := make(map[string]*deviceInfo, len(devCallsignSSIDMap))
	for k, v := range devCallsignSSIDMap {
		res[k] = v
	}
	devMapMu.RUnlock()
	return res
}

func forEachDev(fn func(key string, dev *deviceInfo) bool) {
	devMapMu.RLock()
	defer devMapMu.RUnlock()
	for k, v := range devCallsignSSIDMap {
		if !fn(k, v) {
			return
		}
	}
}

func setOnlineDevMap(m map[int]*deviceInfo) {
	onlineDevPtr.Store(&m)
}

func getOnlineDevMap() map[int]*deviceInfo {
	if ptr := onlineDevPtr.Load(); ptr != nil {
		return *ptr
	}
	return onlinedevMap
}

func setServerDevice(callsign string, dev *deviceInfo) {
	serverMapMu.Lock()
	ServerMap[callsign] = dev
	serverMapMu.Unlock()
}

func udpConnOrGlobal(conn *net.UDPConn) *net.UDPConn {
	if conn != nil {
		return conn
	}
	return globelconn
}

type qth struct {
	QTH          string    `json:"qth"`
	CallSignSSID string    `json:"callsign_ssid"`
	JoinTime     time.Time `json:"join_time"`
	Name         string    `json:"name"`
}

var (
	QTHmapNew = make(map[string]qth) // callsign+ssid
	QTHmap    = make(map[string]string)
	qthMu     sync.RWMutex
)

func setQTH(callsignSSID, oldQTH string, newQTH qth) {
	qthMu.Lock()
	defer qthMu.Unlock()

	QTHmap[callsignSSID] = oldQTH
	QTHmapNew[callsignSSID] = newQTH
}

func getQTHEntry(callsignSSID string) (qth, bool) {
	qthMu.RLock()
	defer qthMu.RUnlock()

	q, ok := QTHmapNew[callsignSSID]
	return q, ok
}

func snapshotQTHMap() map[string]string {
	qthMu.RLock()
	defer qthMu.RUnlock()

	res := make(map[string]string, len(QTHmap))
	for k, v := range QTHmap {
		res[k] = v
	}
	return res
}

func snapshotQTHMapNew() map[string]qth {
	qthMu.RLock()
	defer qthMu.RUnlock()

	res := make(map[string]qth, len(QTHmapNew))
	for k, v := range QTHmapNew {
		res[k] = v
	}
	return res
}

type currentConnPool struct {
	mu                  sync.RWMutex
	voiceOwnerID        string
	ctlUDPAddr          *net.UDPAddr // 控制通道占用者地址，与语音话权相互独立
	ctlUDPAddrStr       string
	lastOwnerPacketTime time.Time // 当前占用者本人最近一个被接受的包时间（用于抢话判定）
	lastCtlTime         time.Time
	lastPriority        int

	//allowCALLSSID []string
	devConnMap    map[string]*deviceInfo //key udpaddr
	devAddrByDev  map[*deviceInfo]string //反向索引：NAT端口变化时立即替换旧addr，避免幽灵条目
	devConnList   []*deviceInfo
	devListAtomic atomic.Pointer[[]*deviceInfo]
}

func (p *currentConnPool) count() int {
	if ptr := p.devListAtomic.Load(); ptr != nil {
		return len(*ptr)
	}
	p.mu.RLock()
	n := len(p.devConnMap)
	p.mu.RUnlock()
	return n
}

func (p *currentConnPool) ensureDevice(addr string, dev *deviceInfo) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.devConnMap == nil {
		p.devConnMap = make(map[string]*deviceInfo)
	}
	if p.devAddrByDev == nil {
		p.devAddrByDev = make(map[*deviceInfo]string)
	}

	if existing, ok := p.devConnMap[addr]; ok && existing == dev {
		return false
	}

	// 若同一台设备因 NAT 重连更换了源端口，立刻移除旧地址，避免房间人数膨胀或重复转发。
	if oldAddr, ok := p.devAddrByDev[dev]; ok && oldAddr != addr {
		if p.devConnMap[oldAddr] == dev {
			delete(p.devConnMap, oldAddr)
		}
	}

	p.devConnMap[addr] = dev
	p.devAddrByDev[dev] = addr
	p.rebuildListLocked()
	return true
}

func (p *currentConnPool) snapshotMap() map[string]*deviceInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	res := make(map[string]*deviceInfo, len(p.devConnMap))
	for k, v := range p.devConnMap {
		res[k] = v
	}
	return res
}

// devices returns a read-only COW slice of devices currently in the pool
// without locking or allocating on the forwarding hot path. Callers must not
// mutate the returned slice.
func (p *currentConnPool) devices() []*deviceInfo {
	if ptr := p.devListAtomic.Load(); ptr != nil {
		return *ptr
	}

	p.mu.RLock()
	list := make([]*deviceInfo, 0, len(p.devConnMap))
	for _, dev := range p.devConnMap {
		list = append(list, dev)
	}
	p.mu.RUnlock()
	return list
}

func (p *currentConnPool) snapshotList() []*deviceInfo {
	return p.devices()
}

func (p *currentConnPool) getDevice(addr string) (*deviceInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	dev, ok := p.devConnMap[addr]
	return dev, ok
}

func voiceOwnerID(nrl *NRL21packet, dev *deviceInfo) string {
	serverVoice := nrl.Type == 9 || nrl.DevModel == 200 || nrl.DevModel == 255 || nrl.SSID == 200 || nrl.SSID == 255
	if serverVoice {
		if nrl.OriginalCallsign != "" {
			return getCallsignSSID(nrl.OriginalCallsign, nrl.OriginalSSID)
		}
		return "relay:" + nrl.UDPAddrStr
	}
	return getCallsignSSID(dev.CallSign, dev.SSID)
}

func (p *currentConnPool) setVoiceStateLocked(ownerID string, ts time.Time, priority int) {
	p.voiceOwnerID = ownerID
	p.lastOwnerPacketTime = ts
	p.lastPriority = priority
}

func (p *currentConnPool) setVoiceState(ownerID string, ts time.Time, priority int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.setVoiceStateLocked(ownerID, ts, priority)
}

func (p *currentConnPool) acquireVoice(ownerID string, ts time.Time, priority int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.voiceOwnerID != "" && p.voiceOwnerID != ownerID &&
		priority <= p.lastPriority && ts.Sub(p.lastOwnerPacketTime) < 400*time.Millisecond {
		return false
	}

	p.setVoiceStateLocked(ownerID, ts, priority)
	return true
}

func (p *currentConnPool) ctlState() (*net.UDPAddr, time.Time) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ctlUDPAddr, p.lastCtlTime
}

func (p *currentConnPool) setCtlState(addr *net.UDPAddr, ts time.Time) {
	addrStr := ""
	if addr != nil {
		addrStr = addr.String()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ctlUDPAddr = addr
	p.ctlUDPAddrStr = addrStr
	p.lastCtlTime = ts
}

func (p *currentConnPool) acquireCtl(addr *net.UDPAddr, addrStr string, ts time.Time) bool {
	if addrStr == "" && addr != nil {
		addrStr = addr.String()
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctlUDPAddrStr != "" && p.ctlUDPAddrStr != addrStr && ts.Sub(p.lastCtlTime) < 200*time.Millisecond {
		return false
	}

	p.ctlUDPAddr = addr
	p.ctlUDPAddrStr = addrStr
	p.lastCtlTime = ts
	return true
}

func (p *currentConnPool) removeDevice(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if dev, ok := p.devConnMap[addr]; ok {
		delete(p.devConnMap, addr)
		if p.devAddrByDev != nil && p.devAddrByDev[dev] == addr {
			delete(p.devAddrByDev, dev)
		}
		p.rebuildListLocked()
	}
}

func (p *currentConnPool) rebuildListLocked() {
	list := make([]*deviceInfo, 0, len(p.devConnMap))
	addrByDev := make(map[*deviceInfo]string, len(p.devConnMap))
	for addr, dev := range p.devConnMap {
		list = append(list, dev)
		addrByDev[dev] = addr
	}
	p.devAddrByDev = addrByDev
	p.devConnList = list
	p.devListAtomic.Store(&list)
}

func udpServer() {

	port, err := strconv.Atoi(conf.System.Port)
	if err != nil {
		fmt.Println(" udp port err:" + err.Error())
		os.Exit(1)

	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: port,
	})

	if err != nil {
		fmt.Println("read from connect failed, err:" + err.Error())
		os.Exit(1)
	}

	defer conn.Close()

	if err := conn.SetReadBuffer(udpSocketBufferSize); err != nil {
		log.Printf("udp set read buffer (%d) failed: %v", udpSocketBufferSize, err)
	}
	if err := conn.SetWriteBuffer(udpSocketBufferSize); err != nil {
		log.Printf("udp set write buffer (%d) failed: %v", udpSocketBufferSize, err)
	}

	globelconn = conn

	//启动服务器互联
	for _, v := range queryServers() {
		if v.Status == 1 {
			v.Start()
		}
	}

	log.Println("data parse server started on udp :", conf.System.Port)

	dispatcher := newUDPRoomDispatcher(0)
	registrar := newNewDeviceRegistrar(dispatcher)

	for {
		udpProcess(conn, dispatcher, registrar)
	}
}

func udpProcess(conn *net.UDPConn, dispatcher *udpRoomDispatcher, registrar *newDeviceRegistrar) {

	for {
		bufPtr := getUDPPacketBuf()
		data := *bufPtr
		n, remoteaddr, err := conn.ReadFromUDP(data)
		if err != nil {
			putUDPPacketBuf(bufPtr)
			fmt.Println("failed read udp msg, error: ", err)
			break
		}

		nrl, err := newNRL21packet(remoteaddr, data[:n])
		if err != nil {
			logDecodeErr(remoteaddr, err, data[:n], time.Now())
			putUDPPacketBuf(bufPtr)
			continue
		}

		addPacketNumberStat(1)

		callsignSSID := getCallsignSSID(nrl.CallSign, nrl.SSID)

		if dev, ok := getDevByCallsignSSID(callsignSSID); ok {
			dispatcher.dispatch(udpPacketTask{
				nrl:    nrl,
				bufPtr: bufPtr,
				length: n,
				dev:    dev,
				conn:   conn,
			})
		} else {
			registrar.enqueue(newDeviceTask{
				callsignSSID: callsignSSID,
				nrl:          nrl,
				bufPtr:       bufPtr,
				length:       n,
				conn:         conn,
			})
		}
	}
}

func handleKnownDevicePacket(nrl *NRL21packet, packet []byte, dev *deviceInfo, conn *net.UDPConn) {
	if nrl.Type == 2 && refreshDeviceAccountExpired(dev, nrl.timeStamp) {
		removeExpiredDeviceFromPool(dev, groupForDevice(dev))
		dev.expireMu.Lock()
		lastCheck := dev.LastExpireCheck
		dev.expireMu.Unlock()
		log.Printf("billing: expired account heartbeat ignored: %v-%v expire check at %v", dev.CallSign, dev.SSID, lastCheck.Format("2006-01-02 15:04:05"))
		return
	}

	pktBytes := 42 + 48 + len(nrl.DATA)
	dmrid, groupID := dev.recordPacket(nrl.timeStamp, pktBytes)
	addTrafficStat(int64(pktBytes))

	if nrl.DevModel != 200 {
		NRL21SetDevDMRID(dmrid, packet)
	}

	//  没有加入公共组的设备，使用用户内置连接池
	if isPrivateGroupID(groupID) {

		if u, okok := userlist.Load(dev.CallSign); okok {
			if privateGroup, exists := u.(*userinfo).Groups[groupID]; exists {
				NRL21parser(nrl, packet, dev, conn, privateGroup)
			}
		} else {
			fmt.Println("dev:", dev, nrl)
		}

	} else if isPublicGroupID(groupID) {

		//否则使用公共群组连接池
		if p, ok := getPublicGroup(groupID); ok {
			NRL21parser(nrl, packet, dev, conn, p)
		}
	} else {
		log.Printf("dev:%v, nrl:%v, dev.GroupID error:%v", dev, nrl, groupID)
	}
}

func groupForDevice(dev *deviceInfo) *group {
	if dev == nil {
		return nil
	}
	groupID := dev.getGroupID()
	if isPrivateGroupID(groupID) {
		if u, ok := userlist.Load(dev.CallSign); ok {
			return u.(*userinfo).Groups[groupID]
		}
	}
	g, _ := getPublicGroup(groupID)
	return g
}

func recordVoiceActivity(nrl *NRL21packet, dev *deviceInfo, gp *group, voiceDurationMS int) {
	dev.stateMu.Lock()
	td := nrl.timeStamp.Sub(dev.LastVoiceEndTime).Milliseconds()
	shouldLog := false
	callStart := false
	if td > 200 {
		if !dev.Loged && !dev.LastVoiceEndTime.IsZero() {
			shouldLog = true
		}
		callStart = true
	}
	dev.stateMu.Unlock()

	if shouldLog {
		enqueueCallLog(dev)
	}

	dev.stateMu.Lock()
	if callStart {
		dev.LastVoiceBeginTime = nrl.timeStamp
	}
	dev.Loged = false
	dev.LastVoiceDuration = int(nrl.timeStamp.Sub(dev.LastVoiceBeginTime).Milliseconds())
	dev.LastVoiceEndTime = nrl.timeStamp
	dev.LastCtlEndTime = nrl.timeStamp
	dev.VoiceTime += voiceDurationMS
	dev.stateMu.Unlock()

	if callStart {
		if nrl.DevModel == 200 || (nrl.DevModel == 255 && nrl.SSID == 255) {
			callWSHub.trackCallStart(gp, nrl.OriginalCallsign, nrl.OriginalSSID, nrl.timeStamp)
		} else {
			callWSHub.trackCallStart(gp, dev.CallSign, dev.SSID, nrl.timeStamp)
		}
	}

	addVoiceTimeStat(int64(voiceDurationMS))
}

func appendPacketSuffix(packet []byte, suffix string) []byte {
	resp := make([]byte, len(packet)+len(suffix))
	copy(resp, packet)
	copy(resp[len(packet):], suffix)
	return resp
}

func NRL21parser(nrl *NRL21packet, packet []byte, dev *deviceInfo, conn *net.UDPConn, gp *group) {

	// 报文内容完全来自 UDP 对端，畸形包不应导致进程崩溃：捕获解析 panic 并丢弃该包
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recover from packet parse panic, from %v %v-%v type %v: %v",
				nrl.UDPAddrStr, nrl.CallSign, nrl.SSID, nrl.Type, r)
		}
	}()

	switch nrl.Type {

	case 0:
		//控制指令，用户远程控制设备
		fmt.Println("recived control commond ", nrl)
	case 1, 8:
		//1 语音消息，需要转发给群组内其它设备,
		//设备状态为禁发

		if (dev.getStatus() & 1) == 1 {
			return
		}

		voiceDurationMS := 63 // legacy G.711 packets may contain about 62.5 ms
		if nrl.Type == 8 {
			voiceDurationMS = 20
		}
		recordVoiceActivity(nrl, dev, gp, voiceDurationMS)

		//来自其他服务器255的包
		if nrl.DevModel == 255 || nrl.SSID == 255 {
			dev.setOnline(true)
			forwardServerVoice(nrl, dev, packet, conn, gp)
			return
		}

		if nrl.DevModel == 200 || nrl.SSID == 200 {
			forwardServerVoice(nrl, dev, packet, conn, gp)
			return
		}

		forwardVoice(nrl, dev, packet, conn, gp)

	case 2:

		//处理服务器转发的定制心跳，不能响应回去，会循环
		if len(packet) == 52 {

			qthStr := getQTH(net.IP(packet[48:]).String())
			dev.setQTHValue(qthStr)

			log.Printf("forward dev online:%v %v %v-%v %v\n", nrl.UDPAddrStr, net.IP(packet[48:]).String(), dev.CallSign, dev.SSID, qthStr)
			return

		}

		dev.setUDPAddrWithStr(nrl.UDPAddr, nrl.UDPAddrStr)

		//心跳包，用于保存设备在线存活状态， 目前设备1s一次发送
		dev.stateMu.Lock()
		shouldLogHB := !dev.Loged && !dev.LastVoiceEndTime.IsZero() && nrl.timeStamp.Sub(dev.LastVoiceEndTime).Milliseconds() > 200
		if shouldLogHB {
			dev.Loged = true
		}
		dev.stateMu.Unlock()
		if shouldLogHB {
			enqueueCallLog(dev)
		}

		//判断设备是否已经在组内，有可能设备网络重新连接过，udp端口号变化过，需要重新加入组内
		if gp.connPool.ensureDevice(nrl.UDPAddrStr, dev) && nrl.SSID == 200 {
			setServerDevice(nrl.CallSign, dev)
		}

		//如果是服务器自己发出的和其他服务器连接的心跳包，则更新在线状态，不能继续转发
		// dev.udpSocket 这个值只有发出心跳包的设备用到
		if dev.udpSocket == nil {

			if dev.getDeviceParm() == nil && dev.getDevModel() < 100 {
				//发送查询设备参数
				conn.WriteToUDP(encodeNRL21(dev.CallSign, dev.SSID, 3, 0, 0, []byte{0x01}), nrl.UDPAddr)

			} else {
				conn.WriteToUDP(packet, nrl.UDPAddr)
			}

		}

		// 心跳包里带了设备型号时，持续同步，避免首次上线拿到 0 或旧值后一直不刷新。
		if nrl.DevModel != 0 {
			dev.setDevModel(nrl.DevModel)
		}

		if !dev.isOnline() {

			//查询设备qth信息

			qthStr := getQTH(nrl.UDPAddr.IP.String())
			dev.setQTHValue(qthStr)
			setQTH(dev.CallSignSSID, qthStr, qth{qthStr, dev.CallSignSSID, time.Now(), dev.Name})

			log.Printf("dev online:%v %v-%v %v  group %v \n", nrl.UDPAddrStr, dev.CallSign, dev.SSID, qthStr, gp.ID)

			if dev.getDevModel() != 200 {
				at := &ATcommand{CallSign: dev.CallSign, SSID: dev.SSID, Type: 0x01, ATcommand: "AT+READ", Data: "123"}
				DeviceAT(at)

			}

			dev.setOnline(true)

		}

		//原样回复心跳，ssid小于100的设备，尝试获取设备的配置参数

	case 3:
		//读取设备的配置参数

		dev.setDeviceParm(decodeControlPacket(nrl.DATA))

	case 4:

	case 5: //文本消息,其他多媒体

		/**
			[text]adsfkafdaasdfasfd
		    [img]https://xxx.oss.aliyun.com/adafasdfasdfasdfasfd
		    [video]https://asfdfasdfasfdasdfasdfasdf
		    [file]https://adfasldfasdfasdfas//*
		*/

		forwardMsg(nrl, packet, dev, conn, gp.connPool)

	case 6, 10: //设备到设备控制通道

		if (dev.getStatus() & 1) == 1 {

			return
		}

		dev.stateMu.Lock()
		if nrl.timeStamp.Sub(dev.LastCtlEndTime).Milliseconds() > 200 {
			dev.LastCtlBeginTime = nrl.timeStamp
		}
		dev.LastCtlDuration = int(nrl.timeStamp.Sub(dev.LastCtlBeginTime).Milliseconds())
		dev.LastCtlEndTime = nrl.timeStamp
		dev.CtlTime = dev.CtlTime + 63
		dev.stateMu.Unlock()

		forwardCtl(nrl, dev, packet, conn, gp)

	case 7: //设备端操作指令

		var remoteIP net.IP
		if nrl.UDPAddr != nil {
			remoteIP = nrl.UDPAddr.IP
		}

		// 报文不含子指令字节（DATA 为空）时直接丢弃，防止 packet[48] 越界
		if len(packet) <= 48 {
			log.Printf("dev %v:%v-%v group cmd(type 7) missing sub-command, drop it\n", dev.CallSign, dev.SSID, remoteIP)
			return
		}

		// 关闭群组查询/切换功能时，直接丢弃 7 号指令
		if conf.System.DisableGroupCmd {
			log.Printf("dev %v:%v-%v group cmd(type 7-%v) disabled, drop it\n", dev.CallSign, dev.SSID, remoteIP, packet[48])
			return
		}

		t := packet[48]

		switch t {

		case 1: //切换组指令

			// 切组指令需要 DATA 至少 5 字节（子指令 + 4 字节组号）
			if len(packet) < 53 {
				log.Printf("dev %v:%v-%v change group packet too short, drop it\n", dev.CallSign, dev.SSID, remoteIP)
				return
			}

			groupid := int(binary.BigEndian.Uint32(packet[49:53]))

			log.Printf("dev %v:%v-%v change group to %v to %v, data:  % X \n", dev.CallSign, dev.SSID, remoteIP, dev.getGroupID(), groupid, packet)
			str, err := changeDevGroup(dev, groupid)

			if err != nil {
				log.Println("change group err:", err)
				conn.WriteToUDP(appendPacketSuffix(packet, strconv.Itoa(groupid)+",error"), nrl.UDPAddr)
			} else {
				enqueueUpdateDeviceGroupID(dev.ID, dev.getGroupID())
				conn.WriteToUDP(appendPacketSuffix(packet, str), nrl.UDPAddr)
			}

		case 2: //获取组列表

			resp := getGroupListForDevice(packet)

			conn.WriteToUDP(resp, nrl.UDPAddr)
			log.Printf("dev %v:%v-%v download grouplist. size: %v\n", dev.CallSign, dev.SSID, remoteIP, len(resp))

		}
		//服务器互联
	case 9:
		if (dev.getStatus() & 1) == 1 {
			return
		}

		recordVoiceActivity(nrl, dev, gp, 20)
		forwardServerVoice(nrl, dev, packet, conn, gp)

	case 11: //AT  01 读取，02写入多条 0x0d,0x0a分割，03读取所有，

		at := decodeATPacket(dev.CallSign, dev.SSID, nrl.DATA)
		//fmt.Println(at)
		dev.setLastATCommand(at)

	case 12: //COM 透传
		forwardCOM(nrl, dev, packet, conn, gp)

	default:
		fmt.Println("unknow data:", nrl.Type, nrl)
		//conn.WriteToUDP(packet, n.Addr)

	}

}

func update200QTH(Originalcallsignssid string, nrl *NRL21packet, dev *deviceInfo) {

	callsignssid := getCallsignSSID(nrl.CallSign, nrl.SSID)
	originalqth := ""
	if len(nrl.OriginalIP) == net.IPv4len || len(nrl.OriginalIP) == net.IPv6len {
		originalqth = getQTH(nrl.OriginalIP.String())
	}

	setQTH(Originalcallsignssid, callsignssid+"-"+originalqth, qth{
		originalqth,
		Originalcallsignssid,
		time.Now(),
		callsignssid + "-" + dev.Name,
	})

}

func FullNetOutput(nrl *NRL21packet, dev *deviceInfo, packet []byte, conn *net.UDPConn) {

	if conf.APRS.CallSign == "" {
		return
	}

	outConn := udpConnOrGlobal(conn)
	if outConn == nil {
		return
	}

	newpacket := NRL21replace200and255dev(conf.APRS.CallSign, 255, nrl.Type, 255, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), dev.getDMRID(), packet)

	for _, v := range getPlatformList() {

		if v.udpAddr != nil && v.Host != conf.APRS.SelfAddress {
			outConn.WriteToUDP(newpacket, v.udpAddr)
		}

	}

}

func forwardCOM(nrl *NRL21packet, dev *deviceInfo, packet []byte, conn *net.UDPConn, gp *group) {

	if gp.ID > 3 || gp.ID == 0 {
		return
	}

	if dev != nil {
		dev.stateMu.Lock()
		dev.LastVoiceEndTime = nrl.timeStamp
		dev.stateMu.Unlock()
	}

	outConn := udpConnOrGlobal(conn)
	if outConn == nil {
		return
	}

	for _, vv := range gp.connPool.devices() {
		if vv == dev {
			continue
		}
		addr, addrStr := vv.getUDPAddr()
		//报文转发给其它设备，不包含自己
		if addr != nil && nrl.UDPAddrStr != addrStr {
			if vv.getDevModel() == 200 {
				continue
			}
			outConn.WriteToUDP(packet, addr)
		}
	}

}

func sendVoiceToRoomDevices(
	nrl *NRL21packet,
	dev *deviceInfo,
	packet []byte,
	conn *net.UDPConn,
	gp *group,
	packetForRecipient func(*deviceInfo) ([]byte, error),
	check200Transparent bool,
	skip255 bool,
) {
	outConn := udpConnOrGlobal(conn)
	if outConn == nil {
		return
	}

	senderDMRID := dev.getDMRID()
	for _, vv := range gp.connPool.devices() {
		if vv == dev {
			continue
		}
		vvStatus := vv.getStatus()
		if (vvStatus & 2) == 2 {
			continue
		}
		addr, addrStr := vv.getUDPAddr()
		if addr == nil || nrl.UDPAddrStr == addrStr {
			continue
		}

		vvModel := vv.getDevModel()
		if vvModel == 200 && (!check200Transparent || (vvStatus&4) != 4) {
			//普通设备发给200设备，需要将原始呼号和SSID放到协议头
			newpacket := NRL21replace200and255dev(vv.CallSign, vv.SSID, nrl.Type, 200, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), senderDMRID, packet)
			outConn.WriteToUDP(newpacket, addr)
		} else if skip255 && (vvModel == 255 || vv.SSID == 255) {
			continue
		} else {
			//普通设备发给普通设备，老型号按需转为 G.711
			outPacket, err := packetForRecipient(vv)
			if err == nil {
				outConn.WriteToUDP(outPacket, addr)
			}
		}
	}
}

func forwardVoice(nrl *NRL21packet, dev *deviceInfo, packet []byte, conn *net.UDPConn, gp *group) {
	packetForRecipient := voicePacketSelectorForSender(dev, nrl, packet)
	getPCM := func() ([]int16, error) {
		return ensureVoicePCM(dev, nrl)
	}
	devPriority := dev.getPriority()

	// 鹦鹉房间内每台设备只录制并回放自己的语音。录音器保存在设备上，
	// 因此多台设备同时在线时，各自的录音和回放互不影响，也不会转发给其他设备。
	if voiceEchoRoomEnabled(gp) {
		outPacket, err := packetForRecipient(dev)
		if err == nil {
			queueDeviceVoiceEcho(gp, dev, nrl, outPacket)
		}
		gp.connPool.setVoiceState(voiceOwnerID(nrl, dev), nrl.timeStamp, devPriority)
		callWSHub.publishVoicePacket(gp, dev.CallSign, dev.SSID, nrl.timeStamp, getPCM)
		return
	}

	numbs := gp.connPool.count()
	// 中继互联和全网通房间始终沿用原有单工转发逻辑。
	if gp.Type == 1 || gp.Type == 4 || gp.ID == 999 {
		numbs = 3
	}

	switch numbs {

	case 0:
		return
	case 1: //只有一个设备时不转发；录音回放仅由鹦鹉房间提供
		gp.connPool.setVoiceState(voiceOwnerID(nrl, dev), nrl.timeStamp, devPriority)
		callWSHub.publishVoicePacket(gp, dev.CallSign, dev.SSID, nrl.timeStamp, getPCM)

	case 2: //如果有2个设备，缺省为全双工通信，报文转发给对方
		callWSHub.publishVoicePacket(gp, dev.CallSign, dev.SSID, nrl.timeStamp, getPCM)
		sendVoiceToRoomDevices(nrl, dev, packet, conn, gp, packetForRecipient, true, false)

	default: //3个或3个以上设备，只允许一个设备发送语音，其它接收

		//房间类型为会议室的时候，需要将语音进行混音，语音先放入缓存，等待其他设备的语音包
		if gp.Type == 7 && (nrl.Type == 1 || nrl.Type == 8) {

			// Both codecs enter the mixer as 20 ms / 8 kHz PCM frames.
			// A failed Opus packet contributes no audio to this conference tick.
			pcm, err := ensureVoicePCM(dev, nrl)
			if err == nil && len(pcm) > 0 {
				dev.pcmMu.Lock()
				dev.pcmBuf = append(dev.pcmBuf, pcm...)
				dev.pcmMu.Unlock()
			}
			return
		}

		if !gp.connPool.acquireVoice(voiceOwnerID(nrl, dev), nrl.timeStamp, devPriority) {
			dev.stateMu.Lock()
			dev.LastVoiceEndTime = nrl.timeStamp
			dev.stateMu.Unlock()
			return
		}
		callWSHub.publishVoicePacket(gp, dev.CallSign, dev.SSID, nrl.timeStamp, getPCM)
		sendVoiceToRoomDevices(nrl, dev, packet, conn, gp, packetForRecipient, false, true)

		if gp.ID == 999 {
			FullNetOutput(nrl, dev, packet, conn)

		}

	}

}

func forwardServerVoice(nrl *NRL21packet, dev *deviceInfo, packet []byte, conn *net.UDPConn, gp *group) {

	if nrl.OriginalCallsign != "" {
		Originalcallsignssid := getCallsignSSID(nrl.OriginalCallsign, nrl.OriginalSSID)

		if q, ok := getQTHEntry(Originalcallsignssid); !ok {
			update200QTH(Originalcallsignssid, nrl, dev)

		} else if time.Since(q.JoinTime) > 10*time.Minute {
			update200QTH(Originalcallsignssid, nrl, dev)

		}
	}

	if !gp.connPool.acquireVoice(voiceOwnerID(nrl, dev), nrl.timeStamp, dev.getPriority()) {
		if k, ok := gp.connPool.getDevice(nrl.UDPAddrStr); ok {
			k.stateMu.Lock()
			k.LastVoiceEndTime = nrl.timeStamp
			k.stateMu.Unlock()
		}
		return
	}

	newpacket := packet
	if nrl.OriginalCallsign != "" {
		newpacket = NRL21replace200and255dev(
			nrl.OriginalCallsign, nrl.OriginalSSID, nrl.Type, 200,
			nrl.CallSign, nrl.SSID, nrl.OriginalIP, nrl.DMRID, packet,
		)
	}
	packetForRecipient := voicePacketSelectorForSender(dev, nrl, newpacket)
	getPCM := func() ([]int16, error) {
		return ensureVoicePCM(dev, nrl)
	}
	callWSHub.publishVoicePacket(gp, nrl.OriginalCallsign, nrl.OriginalSSID, nrl.timeStamp, getPCM)

	outConn := udpConnOrGlobal(conn)
	if outConn == nil {
		return
	}

	isFrom255 := nrl.DevModel == 255 || nrl.SSID == 255

	for _, vv := range gp.connPool.devices() {
		if vv == dev || (vv.getStatus()&2) == 2 {
			continue
		}
		addr, addrStr := vv.getUDPAddr()
		if addr == nil || nrl.UDPAddrStr == addrStr {
			continue
		}

		vvModel := vv.getDevModel()

		// 不转发给 255 设备（避免 255->255 全网通环路及 200->255 跨网转发）
		if vvModel == 255 || vv.SSID == 255 {
			continue
		}

		// 转发给 200 设备（255 与 200 不允许相互转发）
		if vvModel == 200 || vv.SSID == 200 {
			if isFrom255 {
				continue
			}
			new200packet := NRL21replace200and255dev(
				vv.CallSign, vv.SSID, nrl.Type, 200,
				nrl.OriginalCallsign, nrl.OriginalSSID, nrl.OriginalIP, nrl.DMRID, packet,
			)
			outConn.WriteToUDP(new200packet, addr)
			continue
		}

		// 转发给普通设备，需要将原始信息替换协议头信息（老型号按需转为 G.711）
		outPacket, err := packetForRecipient(vv)
		if err == nil {
			outConn.WriteToUDP(outPacket, addr)
		}
	}

}

// 文本消息转发
func forwardMsg(nrl *NRL21packet, packet []byte, dev *deviceInfo, conn *net.UDPConn, connpool *currentConnPool) {

	outConn := udpConnOrGlobal(conn)
	if outConn == nil {
		return
	}

	clientAddrStr := nrl.UDPAddrStr
	devices := connpool.devices()
	devGroupID := dev.getGroupID()

	//200设备转发给其他设备
	if nrl.DevModel == 200 {

		// 999房间不接收200设备的消息
		if devGroupID == 999 {
			return
		}

		newpacket := NRL21replace200and255dev(nrl.OriginalCallsign, nrl.OriginalSSID, nrl.Type, 200, nrl.CallSign, nrl.SSID, nrl.OriginalIP, nrl.DMRID, packet)

		for _, vv := range devices {
			if vv == dev {
				continue
			}
			addr, addrStr := vv.getUDPAddr()
			if addr == nil || clientAddrStr == addrStr {
				continue
			}

			vvModel := vv.getDevModel()
			//马工3188盒子，部分盒子使用5类型控制3188信道，会干扰，临时关闭
			if vvModel == 9 || vvModel == 255 || vv.SSID == 255 {
				continue
			}
			//200设备转发给其他200设备，需要替换包头的呼号为新200设备的呼号
			if vvModel == 200 {
				new200packet := NRL21replace200and255dev(vv.CallSign, vv.SSID, nrl.Type, 200, nrl.OriginalCallsign, nrl.OriginalSSID, nrl.OriginalIP, nrl.DMRID, packet)
				outConn.WriteToUDP(new200packet, addr)
				// 200和255不允许相互转发
			} else {
				outConn.WriteToUDP(newpacket, addr)
			}
		}

		return
	}

	if nrl.DevModel == 255 || nrl.SSID == 255 {
		newpacket := NRL21replace200and255dev(nrl.OriginalCallsign, nrl.OriginalSSID, nrl.Type, 255, nrl.CallSign, nrl.SSID, nrl.OriginalIP, nrl.DMRID, packet)

		for _, vv := range devices {
			if vv == dev {
				continue
			}
			addr, addrStr := vv.getUDPAddr()
			if addr == nil || clientAddrStr == addrStr {
				continue
			}
			vvModel := vv.getDevModel()
			// 255不转发给255设备，且255和200不允许相互转发
			if vvModel == 9 || vvModel == 255 || vv.SSID == 255 || vvModel == 200 {
				continue
			}
			outConn.WriteToUDP(newpacket, addr)
		}

		return
	}

	//普通设备转发给其他设备
	for _, vv := range devices {
		if vv == dev {
			continue
		}
		addr, addrStr := vv.getUDPAddr()
		if addr == nil || clientAddrStr == addrStr {
			continue
		}

		vvModel := vv.getDevModel()
		//普通设备转发给200设备
		if vvModel == 200 {

			// 999房间不转发给200设备
			if devGroupID == 999 {
				continue
			}

			newpacket := NRL21replace200and255dev(vv.CallSign, vv.SSID, nrl.Type, 200, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), vv.getDMRID(), packet)
			outConn.WriteToUDP(newpacket, addr)

			//普通设备不转发给255设备
		} else if vvModel == 9 || vvModel == 255 || vv.SSID == 255 {
			continue

			//普通设备转发给普通设备
		} else {
			outConn.WriteToUDP(packet, addr)
		}
	}

	//普通设备转发给全网通设备，输出到各个平台
	if devGroupID == 999 {
		FullNetOutput(nrl, dev, packet, outConn)
	}

}

func sendCtlToRoomDevices(nrl *NRL21packet, dev *deviceInfo, packet []byte, conn *net.UDPConn, gp *group) {
	outConn := udpConnOrGlobal(conn)
	if outConn == nil {
		return
	}

	for _, vv := range gp.connPool.devices() {
		if vv == dev || (vv.getStatus()&2) == 2 || vv.getDevModel() == 200 {
			continue
		}
		addr, addrStr := vv.getUDPAddr()
		if addr == nil || nrl.UDPAddrStr == addrStr {
			continue
		}
		outConn.WriteToUDP(packet, addr)
	}
}

// forwardCtl forwardCtl
func forwardCtl(nrl *NRL21packet, dev *deviceInfo, packet []byte, conn *net.UDPConn, gp *group) {

	numbs := gp.connPool.count()

	//房间类型为中继互联的时候，使用不允许出现双工
	if gp.Type == 1 {
		numbs = 3
	}

	switch numbs {

	case 0:
		return
	case 1: //只有一个设备，缺省为环路测试，报文原样返回
		if outConn := udpConnOrGlobal(conn); outConn != nil {
			outConn.WriteToUDP(packet, nrl.UDPAddr)
		}

		gp.connPool.setCtlState(nrl.UDPAddr, nrl.timeStamp)

	case 2: //如果有2个设备，缺省为全双工通信，报文转发给对方
		sendCtlToRoomDevices(nrl, dev, packet, conn, gp)

	default: //3个或3个以上设备，只允许一个设备发送控制，其它接收
		if !gp.connPool.acquireCtl(nrl.UDPAddr, nrl.UDPAddrStr, nrl.timeStamp) {
			if k, ok := gp.connPool.getDevice(nrl.UDPAddrStr); ok {
				k.stateMu.Lock()
				k.LastCtlEndTime = nrl.timeStamp
				k.stateMu.Unlock()
			}
			return
		}

		sendCtlToRoomDevices(nrl, dev, packet, conn, gp)

	}

}
