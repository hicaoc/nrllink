package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type wsRoomState struct {
	RoomKey    string      `json:"room_key"`
	RoomID     int         `json:"room_id"`
	RoomName   string      `json:"room_name"`
	RoomType   int         `json:"room_type"`
	Callsign   string      `json:"callsign"`
	SSID       byte        `json:"ssid"`
	Speakers   []wsSpeaker `json:"speakers,omitempty"`
	Active     bool        `json:"active"`
	UpdatedAt  string      `json:"updated_at"`
	LastActive int64       `json:"last_active"`
}

type wsSpeaker struct {
	Callsign string `json:"callsign"`
	SSID     byte   `json:"ssid"`
}

type wsCallRecord struct {
	CallID         string `json:"-"`
	RoomKey        string `json:"room_key"`
	RoomID         int    `json:"room_id"`
	RoomName       string `json:"room_name"`
	Callsign       string `json:"callsign"`
	SSID           byte   `json:"ssid"`
	StartedAt      string `json:"started_at"`
	DurationMS     int64  `json:"duration_ms"`
	DurationText   string `json:"duration_text"`
	DurationSecond int64  `json:"duration_second"`
	Active         bool   `json:"active"`
}

type wsCallCommand struct {
	Action   string   `json:"action"`
	RoomKeys []string `json:"room_keys"`
}

type wsCallMessage struct {
	Type             string         `json:"type"`
	Rooms            []wsRoomState  `json:"rooms,omitempty"`
	Room             *wsRoomState   `json:"room,omitempty"`
	RecentCalls      []wsCallRecord `json:"recent_calls,omitempty"`
	Subscriptions    []string       `json:"subscriptions,omitempty"`
	Message          string         `json:"message,omitempty"`
	TotalSubs        int            `json:"total_subs"`
	ConnectedClients int            `json:"connected_clients"`
	OnlineDevices    int            `json:"online_devices"`
}

type roomStateEntry struct {
	wsRoomState
	lastActivity time.Time
}

type activeCallEntry struct {
	record    wsCallRecord
	startedAt time.Time
	promoted  bool
}

type wsCallHub struct {
	mu                sync.RWMutex
	clients           map[*wsCallClient]struct{}
	roomStates        map[string]*roomStateEntry
	activeCalls       map[string]activeCallEntry
	recent            []wsCallRecord
	statsNotify       chan struct{}
	lastOnlineDevices int
}

type wsCallClient struct {
	hub           *wsCallHub
	ws            *websocket.Conn
	user          *userinfo
	done          chan struct{}
	sendMu        sync.Mutex
	mu            sync.Mutex
	closed        bool
	lastSeen      time.Time
	subscriptions map[string]bool
	audioBuffers  map[string][]byte
}

const wsCallAudioFrameSize = 160
const wsCallMaxBufferedBytes = 500 * 50
const wsCallClientTimeout = 25 * time.Second

var wsCallSilenceALaw = Linear2Alaw(0)

var callWSHub = newWSCallHub()

func newWSCallHub() *wsCallHub {
	return &wsCallHub{
		clients:           make(map[*wsCallClient]struct{}),
		roomStates:        make(map[string]*roomStateEntry),
		activeCalls:       make(map[string]activeCallEntry),
		recent:            make([]wsCallRecord, 0, 20),
		statsNotify:       make(chan struct{}, 1),
		lastOnlineDevices: -1,
	}
}

func (h *wsCallHub) requestStatsBroadcast() {
	select {
	case h.statsNotify <- struct{}{}:
	default:
	}
}

func (h *wsCallHub) connectedClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *wsCallHub) totalSubscriptions() int {
	h.mu.RLock()
	clients := make([]*wsCallClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	total := 0
	for _, client := range clients {
		client.mu.Lock()
		total += len(client.subscriptions)
		client.mu.Unlock()
	}
	return total
}

func currentOnlineDeviceCount() int {
	if totalstats.OnlineDevNumber > 0 {
		return totalstats.OnlineDevNumber
	}
	return len(onlinedevMap)
}

func (h *wsCallHub) broadcastStats() {
	connectedClients := h.connectedClientCount()
	totalSubs := h.totalSubscriptions()
	onlineDevices := currentOnlineDeviceCount()
	h.mu.RLock()
	clients := make([]*wsCallClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	msg := wsCallMessage{
		Type:             "stats",
		TotalSubs:        totalSubs,
		ConnectedClients: connectedClients,
		OnlineDevices:    onlineDevices,
	}
	for _, client := range clients {
		if err := client.sendJSON(msg); err != nil {
			client.close()
		}
	}
}

func formatCallDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int64(d / time.Second)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (h *wsCallHub) updateRecentDurationLocked(callID string, d time.Duration) bool {
	durationMS := d.Milliseconds()
	durationText := formatCallDuration(d)
	durationSecond := int64(d / time.Second)

	for i := range h.recent {
		if h.recent[i].CallID != callID {
			continue
		}
		if h.recent[i].DurationSecond == durationSecond {
			return false
		}
		h.recent[i].DurationMS = durationMS
		h.recent[i].DurationText = durationText
		h.recent[i].DurationSecond = durationSecond
		return true
	}
	return false
}

func (h *wsCallHub) insertRecentLocked(record wsCallRecord) {
	h.recent = append([]wsCallRecord{record}, h.recent...)
	if len(h.recent) > 20 {
		h.recent = h.recent[:20]
	}
}

func (h *wsCallHub) promoteActiveCallLocked(roomKey string, duration time.Duration) bool {
	activeCall, ok := h.activeCalls[roomKey]
	if !ok {
		return false
	}
	if activeCall.promoted {
		return h.updateRecentDurationLocked(activeCall.record.CallID, duration)
	}
	if duration < time.Second {
		return false
	}

	activeCall.record.DurationMS = duration.Milliseconds()
	activeCall.record.DurationText = formatCallDuration(duration)
	activeCall.record.DurationSecond = int64(duration / time.Second)
	activeCall.promoted = true
	h.activeCalls[roomKey] = activeCall
	h.insertRecentLocked(activeCall.record)
	return true
}

func (h *wsCallHub) run() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.expireInactiveRooms()
			h.expireInactiveClients()
			currentOnlineDevices := currentOnlineDeviceCount()
			if currentOnlineDevices != h.lastOnlineDevices {
				h.lastOnlineDevices = currentOnlineDevices
				h.broadcastStats()
			}
		case <-h.statsNotify:
			h.broadcastStats()
		}
	}
}

func (h *wsCallHub) expireInactiveClients() {
	now := time.Now()

	h.mu.RLock()
	clients := make([]*wsCallClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		client.mu.Lock()
		lastSeen := client.lastSeen
		closed := client.closed
		client.mu.Unlock()

		if closed {
			continue
		}
		if !lastSeen.IsZero() && now.Sub(lastSeen) > wsCallClientTimeout {
			log.Printf("calls-ws: closing stale client last_seen=%s", lastSeen.Format(time.RFC3339))
			client.close()
		}
	}
}

func (h *wsCallHub) expireInactiveRooms() {
	now := time.Now()
	updates := make([]wsRoomState, 0)
	recentChanged := false

	h.mu.Lock()
	for _, state := range h.roomStates {
		if !state.Active {
			continue
		}
		if now.Sub(state.lastActivity) <= 450*time.Millisecond {
			continue
		}
		if activeCall, ok := h.activeCalls[state.RoomKey]; ok {
			recentChanged = h.promoteActiveCallLocked(state.RoomKey, state.lastActivity.Sub(activeCall.startedAt)) || recentChanged
			delete(h.activeCalls, state.RoomKey)
		}
		state.Active = false
		state.Callsign = ""
		state.SSID = 0
		state.Speakers = nil
		state.UpdatedAt = now.Format("2006-01-02 15:04:05")
		state.LastActive = now.UnixMilli()
		updates = append(updates, state.wsRoomState)
	}
	h.mu.Unlock()

	for _, state := range updates {
		h.broadcastRoomState(state)
	}
	if recentChanged {
		h.broadcastRecentCalls()
	}
}

func roomKeyFromGroup(gp *group) string {
	if gp == nil {
		return ""
	}
	if gp.ID > 0 && gp.ID <= 3 && gp.OwerCallsign != "" && gp.OwerCallsign != "default" {
		return fmt.Sprintf("private:%s:%d", strings.ToUpper(gp.OwerCallsign), gp.ID)
	}
	return fmt.Sprintf("public:%d", gp.ID)
}

func roomStateFromGroup(gp *group) wsRoomState {
	if gp == nil {
		return wsRoomState{}
	}
	return wsRoomState{
		RoomKey:  roomKeyFromGroup(gp),
		RoomID:   gp.ID,
		RoomName: gp.Name,
		RoomType: gp.Type,
	}
}

func normalizeSpeakers(speakers []wsSpeaker) []wsSpeaker {
	if len(speakers) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(speakers))
	result := make([]wsSpeaker, 0, len(speakers))
	for _, speaker := range speakers {
		callsign := strings.ToUpper(strings.TrimSpace(speaker.Callsign))
		if callsign == "" {
			continue
		}
		normalized := wsSpeaker{Callsign: callsign, SSID: speaker.SSID}
		key := fmt.Sprintf("%s-%d", normalized.Callsign, normalized.SSID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Callsign == result[j].Callsign {
			return result[i].SSID < result[j].SSID
		}
		return result[i].Callsign < result[j].Callsign
	})
	return result
}

func sameSpeakers(a, b []wsSpeaker) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func allowedRoomCallsigns(gp *group) map[string]bool {
	if gp == nil || len(gp.AllowCALLSSIDList) == 0 {
		return nil
	}

	allowed := make(map[string]bool, len(gp.AllowCALLSSIDList))
	for _, entry := range gp.AllowCALLSSIDList {
		entry = strings.ToUpper(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if callsign, _, ok := strings.Cut(entry, "-"); ok {
			entry = callsign
		}
		if entry != "" {
			allowed[entry] = true
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func userCallsign(u *userinfo) string {
	if u == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(u.CallSign))
}

func isPrivateRoom(gp *group) bool {
	if gp == nil {
		return false
	}
	return gp.Type == 8 || (gp.ID > 0 && gp.ID <= 3 && gp.OwerCallsign != "" && gp.OwerCallsign != "default")
}

func canUserAccessGroup(u *userinfo, gp *group) bool {
	if gp == nil {
		return false
	}

	callsign := userCallsign(u)
	if isPrivateRoom(gp) {
		owner := strings.ToUpper(strings.TrimSpace(gp.OwerCallsign))
		return callsign != "" && owner != "" && owner != "DEFAULT" && callsign == owner
	}

	allowed := allowedRoomCallsigns(gp)
	if len(allowed) > 0 {
		return callsign != "" && allowed[callsign]
	}
	return true
}

func accessibleRooms(u *userinfo) map[string]*group {
	rooms := make(map[string]*group, len(publicGroupMap)+3)

	for _, gp := range publicGroupMap {
		if !canUserAccessGroup(u, gp) {
			continue
		}
		rooms[roomKeyFromGroup(gp)] = gp
	}
	if u != nil {
		for _, gp := range u.Groups {
			if !canUserAccessGroup(u, gp) {
				continue
			}
			rooms[roomKeyFromGroup(gp)] = gp
		}
	}
	return rooms
}

func sortRoomStates(items []wsRoomState) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].RoomID == items[j].RoomID {
			return items[i].RoomKey < items[j].RoomKey
		}
		return items[i].RoomID < items[j].RoomID
	})
}

func (h *wsCallHub) roomsForUser(u *userinfo) []wsRoomState {
	rooms := accessibleRooms(u)
	items := make([]wsRoomState, 0, len(rooms))

	h.mu.RLock()
	for key, gp := range rooms {
		state := roomStateFromGroup(gp)
		if current, ok := h.roomStates[key]; ok {
			state.Callsign = current.Callsign
			state.SSID = current.SSID
			state.Active = current.Active
			state.UpdatedAt = current.UpdatedAt
			state.LastActive = current.LastActive
		}
		items = append(items, state)
	}
	h.mu.RUnlock()

	sortRoomStates(items)
	return items
}

func (h *wsCallHub) recentCallsForUser(u *userinfo) []wsCallRecord {
	rooms := accessibleRooms(u)

	h.mu.RLock()
	defer h.mu.RUnlock()

	activeCallIDs := make(map[string]bool, len(h.activeCalls))
	for _, activeCall := range h.activeCalls {
		if activeCall.promoted {
			activeCallIDs[activeCall.record.CallID] = true
		}
	}

	items := make([]wsCallRecord, 0, len(h.recent))
	for _, item := range h.recent {
		if _, ok := rooms[item.RoomKey]; ok {
			item.Active = activeCallIDs[item.CallID]
			items = append(items, item)
		}
	}
	return items
}

func (h *wsCallHub) trackCallStart(gp *group, callsign string, ssid byte, ts time.Time) {
	if gp == nil || callsign == "" {
		return
	}

	state := roomStateFromGroup(gp)
	state.Callsign = strings.ToUpper(callsign)
	state.SSID = ssid
	state.Active = true
	state.UpdatedAt = ts.Format("2006-01-02 15:04:05")
	state.LastActive = ts.UnixMilli()

	record := wsCallRecord{
		CallID:         fmt.Sprintf("%s:%d", state.RoomKey, ts.UnixNano()),
		RoomKey:        state.RoomKey,
		RoomID:         state.RoomID,
		RoomName:       state.RoomName,
		Callsign:       state.Callsign,
		SSID:           ssid,
		StartedAt:      state.UpdatedAt,
		DurationMS:     0,
		DurationText:   "00:00",
		DurationSecond: 0,
		Active:         false,
	}

	h.mu.Lock()
	h.roomStates[state.RoomKey] = &roomStateEntry{
		wsRoomState:  state,
		lastActivity: ts,
	}
	h.activeCalls[state.RoomKey] = activeCallEntry{
		record:    record,
		startedAt: ts,
		promoted:  false,
	}
	h.mu.Unlock()

	h.broadcastRoomState(state)
}

func (h *wsCallHub) touchRoomActivity(gp *group, callsign string, ssid byte, ts time.Time, speakers []wsSpeaker) {
	if gp == nil {
		return
	}

	key := roomKeyFromGroup(gp)
	shouldBroadcast := false
	var state wsRoomState
	speakers = normalizeSpeakers(speakers)

	h.mu.Lock()
	entry, ok := h.roomStates[key]
	if !ok {
		entry = &roomStateEntry{wsRoomState: roomStateFromGroup(gp)}
		h.roomStates[key] = entry
	}
	wasActive := entry.Active
	entry.RoomName = gp.Name
	entry.RoomID = gp.ID
	entry.RoomType = gp.Type
	entry.Active = true
	entry.lastActivity = ts
	entry.UpdatedAt = ts.Format("2006-01-02 15:04:05")
	entry.LastActive = ts.UnixMilli()
	recentChanged := false
	if activeCall, ok := h.activeCalls[key]; ok {
		recentChanged = h.promoteActiveCallLocked(key, ts.Sub(activeCall.startedAt))
	}

	if len(speakers) == 0 && callsign != "" {
		speakers = []wsSpeaker{{Callsign: callsign, SSID: ssid}}
	}
	if !sameSpeakers(entry.Speakers, speakers) {
		entry.Speakers = append([]wsSpeaker(nil), speakers...)
		shouldBroadcast = true
	}

	if len(speakers) > 0 {
		primary := speakers[0]
		if entry.Callsign != primary.Callsign || entry.SSID != primary.SSID || !wasActive {
			shouldBroadcast = true
		}
		entry.Callsign = primary.Callsign
		entry.SSID = primary.SSID
	} else if callsign != "" {
		callsign = strings.ToUpper(callsign)
		if entry.Callsign != callsign || entry.SSID != ssid || !wasActive {
			shouldBroadcast = true
		}
		entry.Callsign = callsign
		entry.SSID = ssid
	} else if gp.Type == 7 {
		if entry.Callsign != "" || entry.SSID != 0 || !wasActive {
			shouldBroadcast = true
		}
		entry.Callsign = ""
		entry.SSID = 0
	}

	state = entry.wsRoomState
	h.mu.Unlock()

	if shouldBroadcast {
		h.broadcastRoomState(state)
	}
	if recentChanged {
		h.broadcastRecentCalls()
	}
}

func (h *wsCallHub) publishVoiceFrame(gp *group, callsign string, ssid byte, g711 []byte, ts time.Time, speakers ...wsSpeaker) {
	if gp == nil || len(g711) == 0 {
		return
	}

	h.touchRoomActivity(gp, callsign, ssid, ts, speakers)

	h.mu.RLock()
	clients := make([]*wsCallClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	roomKey := roomKeyFromGroup(gp)
	for _, client := range clients {
		client.enqueueAudio(roomKey, g711)
	}
}

func (h *wsCallHub) addClient(client *wsCallClient) {
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	h.requestStatsBroadcast()
}

func (h *wsCallHub) removeClient(client *wsCallClient) {
	h.mu.Lock()
	delete(h.clients, client)
	h.mu.Unlock()
	h.requestStatsBroadcast()
}

func (h *wsCallHub) broadcastRoomState(state wsRoomState) {
	h.mu.RLock()
	clients := make([]*wsCallClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		if !client.canAccessRoom(state.RoomKey) {
			continue
		}
		if err := client.sendJSON(wsCallMessage{
			Type: "room_state",
			Room: &state,
		}); err != nil {
			client.close()
		}
	}
}

func (h *wsCallHub) broadcastRecentCalls() {
	h.mu.RLock()
	clients := make([]*wsCallClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		if err := client.sendJSON(wsCallMessage{
			Type:        "recent_calls",
			RecentCalls: h.recentCallsForUser(client.user),
		}); err != nil {
			client.close()
		}
	}
}

func newWSCallClient(h *wsCallHub, ws *websocket.Conn, user *userinfo) *wsCallClient {
	return &wsCallClient{
		hub:           h,
		ws:            ws,
		user:          user,
		done:          make(chan struct{}),
		lastSeen:      time.Now(),
		subscriptions: make(map[string]bool),
		audioBuffers:  make(map[string][]byte),
	}
}

func (c *wsCallClient) accessibleRoomKeys() map[string]bool {
	rooms := accessibleRooms(c.user)
	res := make(map[string]bool, len(rooms))
	for key := range rooms {
		res[key] = true
	}
	return res
}

func (c *wsCallClient) canAccessRoom(roomKey string) bool {
	_, ok := accessibleRooms(c.user)[roomKey]
	return ok
}

func (c *wsCallClient) snapshotSubscriptions() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := make([]string, 0, len(c.subscriptions))
	for key := range c.subscriptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (c *wsCallClient) sendJSON(message wsCallMessage) error {
	payload, err := jsonextra.Marshal(message)
	if err != nil {
		return err
	}

	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	return websocket.Message.Send(c.ws, string(payload))
}

func (c *wsCallClient) sendBinary(data []byte) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	return websocket.Message.Send(c.ws, data)
}

func (c *wsCallClient) enqueueAudio(roomKey string, g711 []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed || !c.subscriptions[roomKey] || len(g711) == 0 {
		return
	}

	buf := append(c.audioBuffers[roomKey], g711...)
	if len(buf) > wsCallMaxBufferedBytes {
		buf = buf[len(buf)-wsCallMaxBufferedBytes:]
	}
	c.audioBuffers[roomKey] = buf
}

func (c *wsCallClient) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.done)
	c.mu.Unlock()

	c.hub.removeClient(c)
	c.ws.Close()
}

func (c *wsCallClient) sendSnapshot() error {
	return c.sendJSON(wsCallMessage{
		Type:             "snapshot",
		Rooms:            c.hub.roomsForUser(c.user),
		RecentCalls:      c.hub.recentCallsForUser(c.user),
		Subscriptions:    c.snapshotSubscriptions(),
		TotalSubs:        c.hub.totalSubscriptions(),
		ConnectedClients: c.hub.connectedClientCount(),
		OnlineDevices:    currentOnlineDeviceCount(),
	})
}

func (c *wsCallClient) updateSubscriptions(action string, roomKeys []string) {
	allowed := c.accessibleRoomKeys()

	c.mu.Lock()
	switch action {
	case "set_subscriptions":
		newSubs := make(map[string]bool)
		for _, key := range roomKeys {
			if allowed[key] {
				newSubs[key] = true
			}
		}
		c.subscriptions = newSubs
		for key := range c.audioBuffers {
			if !newSubs[key] {
				delete(c.audioBuffers, key)
			}
		}
	case "subscribe":
		for _, key := range roomKeys {
			if allowed[key] {
				c.subscriptions[key] = true
			}
		}
	case "unsubscribe":
		for _, key := range roomKeys {
			delete(c.subscriptions, key)
			delete(c.audioBuffers, key)
		}
	}
	c.mu.Unlock()

	c.hub.requestStatsBroadcast()
}

func (c *wsCallClient) sendSubscriptions() error {
	return c.sendJSON(wsCallMessage{
		Type:          "subscriptions",
		Subscriptions: c.snapshotSubscriptions(),
	})
}

func (c *wsCallClient) readLoop() error {
	for {
		var raw string
		if err := websocket.Message.Receive(c.ws, &raw); err != nil {
			return err
		}

		cmd := &wsCallCommand{}
		if err := jsonextra.Unmarshal([]byte(raw), cmd); err != nil {
			if sendErr := c.sendJSON(wsCallMessage{Type: "error", Message: "invalid command"}); sendErr != nil {
				return sendErr
			}
			continue
		}

		switch cmd.Action {
		case "ping":
			c.mu.Lock()
			c.lastSeen = time.Now()
			c.mu.Unlock()
			if err := c.sendJSON(wsCallMessage{Type: "pong"}); err != nil {
				return err
			}
		case "subscribe", "unsubscribe", "set_subscriptions":
			c.mu.Lock()
			c.lastSeen = time.Now()
			c.mu.Unlock()
			c.updateSubscriptions(cmd.Action, cmd.RoomKeys)
			if err := c.sendSubscriptions(); err != nil {
				return err
			}
		default:
			if err := c.sendJSON(wsCallMessage{Type: "error", Message: "unknown action"}); err != nil {
				return err
			}
		}
	}
}

func (c *wsCallClient) nextMixedFrame() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	frames := make([][]byte, 0, len(c.subscriptions))
	for key := range c.subscriptions {
		buf := c.audioBuffers[key]
		if len(buf) == 0 {
			continue
		}

		frame := make([]byte, wsCallAudioFrameSize)
		for i := range frame {
			frame[i] = wsCallSilenceALaw
		}

		n := wsCallAudioFrameSize
		if len(buf) < n {
			n = len(buf)
		}
		copy(frame, buf[:n])
		frames = append(frames, frame)

		if len(buf) <= wsCallAudioFrameSize {
			delete(c.audioBuffers, key)
		} else {
			c.audioBuffers[key] = buf[wsCallAudioFrameSize:]
		}
	}

	switch len(frames) {
	case 0:
		return nil
	case 1:
		return append([]byte(nil), frames[0]...)
	}

	mixedPCM := make([]int, wsCallAudioFrameSize)
	for _, frame := range frames {
		for i := 0; i < wsCallAudioFrameSize; i++ {
			sample := frame[i]
			mixedPCM[i] += int(alaw2linear(sample))
		}
	}

	for i, sample := range mixedPCM {
		if sample > 32767 {
			sample = 32767
		} else if sample < -32768 {
			sample = -32768
		}
		mixedPCM[i] = sample
	}

	return G711Encode(mixedPCM)
}

func (c *wsCallClient) audioLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("calls-ws: recovered audio loop panic: %v", r)
		}
	}()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			frame := c.nextMixedFrame()
			if len(frame) == 0 {
				continue
			}
			if err := c.sendBinary(frame); err != nil {
				c.close()
				return
			}
		case <-c.done:
			return
		}
	}
}

func (j *jsonapi) wsCallStream(ws *websocket.Conn) {
	req := ws.Request()
	if req == nil {
		ws.Close()
		return
	}

	tokenString := req.URL.Query().Get("token")
	var user *userinfo
	if tokenString != "" {
		token, err := ValidateToken(tokenString)
		if err != nil {
			ws.Close()
			return
		}

		user, err = getuser(token.Username)
		if err != nil || user.Status != 1 {
			log.Println("websocket user lookup failed:", err)
			ws.Close()
			return
		}
	}

	client := newWSCallClient(callWSHub, ws, user)
	callWSHub.addClient(client)
	defer client.close()

	if err := client.sendSnapshot(); err != nil {
		return
	}

	go client.audioLoop()

	if err := client.readLoop(); err != nil && !strings.Contains(err.Error(), "EOF") {
		log.Println("websocket read loop err:", err)
	}
}
