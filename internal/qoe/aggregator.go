package qoe

import (
	"sort"
	"sync"
	"time"

	"philos-video/internal/models"
	"philos-video/internal/repository"
)

// qualityBitrates maps quality label to video bitrate in kbps (from encoding ladder).
var qualityBitrates = map[string]float64{
	"720p": 2500,
	"480p": 1000,
	"360p": 400,
}

type timedEvent struct {
	at    time.Time
	event models.PlaybackEvent
}

// DashboardMetrics is the aggregated QoE snapshot.
type DashboardMetrics struct {
	Timestamp             time.Time          `json:"timestamp"`
	ActiveSessions        int                `json:"active_sessions"`
	TotalSessionsLast5m   int                `json:"total_sessions_5m"`
	TTFFMedianMs          int                `json:"ttff_median_ms"`
	TTFFP95Ms             int                `json:"ttff_p95_ms"`
	RebufferRate          float64            `json:"rebuffer_rate"`
	AvgRebufferDurationMs int                `json:"avg_rebuffer_duration_ms"`
	AvgBitrateKbps        float64            `json:"avg_bitrate_kbps"`
	QualityDistribution   map[string]float64 `json:"quality_distribution"`
	QualitySwitchesPerMin float64            `json:"quality_switches_per_min"`
	AvgThroughputMbps     float64            `json:"avg_throughput_mbps"`
	P10ThroughputMbps     float64            `json:"p10_throughput_mbps"`
	PerVideo              []VideoMetrics     `json:"per_video"`
	ActiveLiveStreams      int                `json:"active_live_streams"`
}

// VideoMetrics is the per-video breakdown within DashboardMetrics.
type VideoMetrics struct {
	VideoID        string  `json:"video_id"`
	Title          string  `json:"title"`
	ActiveSessions int     `json:"active_sessions"`
	AvgBitrateKbps float64 `json:"avg_bitrate_kbps"`
	RebufferRate   float64 `json:"rebuffer_rate"`
}

// LiveCounter is implemented by the live.Manager to report active stream count.
type LiveCounter interface {
	ActiveCount() int
}

// Aggregator maintains a sliding window of playback metrics in memory.
type Aggregator struct {
	windowDuration time.Duration

	mu             sync.RWMutex
	activeSessions map[string]time.Time // session_id → last heartbeat
	sessionToVideo map[string]string    // session_id → video_id
	videoTitles    map[string]string    // video_id → title (cache)
	recentEvents   []timedEvent
	currentMetrics *DashboardMetrics

	subMu       sync.Mutex
	subscribers map[chan *DashboardMetrics]struct{}

	videoRepo   *repository.VideoRepo
	liveCounter LiveCounter
}

// SetLiveCounter lets main.go wire in the live manager after construction.
func (a *Aggregator) SetLiveCounter(lc LiveCounter) {
	a.mu.Lock()
	a.liveCounter = lc
	a.mu.Unlock()
}

// New creates a new Aggregator and starts its background loop.
func New(videoRepo *repository.VideoRepo) *Aggregator {
	a := &Aggregator{
		windowDuration: 5 * time.Minute,
		activeSessions: make(map[string]time.Time),
		sessionToVideo: make(map[string]string),
		videoTitles:    make(map[string]string),
		currentMetrics: emptyMetrics(),
		subscribers:    make(map[chan *DashboardMetrics]struct{}),
		videoRepo:      videoRepo,
	}
	go a.loop()
	return a
}

// Ingest processes a batch of events from the telemetry handler.
func (a *Aggregator) Ingest(events []models.PlaybackEvent) {
	now := time.Now()
	a.mu.Lock()
	for _, e := range events {
		a.recentEvents = append(a.recentEvents, timedEvent{at: now, event: e})
		// Track session → video mapping
		if e.SessionID != "" && e.VideoID != "" {
			a.sessionToVideo[e.SessionID] = e.VideoID
		}
		// Update active session on heartbeat
		if e.EventType == "heartbeat" && e.SessionID != "" {
			a.activeSessions[e.SessionID] = now
		}
		// Lazy title fetch
		if e.VideoID != "" {
			if _, ok := a.videoTitles[e.VideoID]; !ok {
				a.videoTitles[e.VideoID] = "" // placeholder so we only fetch once
				go a.fetchTitle(e.VideoID)
			}
		}
	}
	a.mu.Unlock()
}

// GetMetrics returns the latest pre-computed metrics snapshot.
func (a *Aggregator) GetMetrics() *DashboardMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentMetrics
}

// Subscribe returns a channel that receives metric updates every second.
func (a *Aggregator) Subscribe() chan *DashboardMetrics {
	ch := make(chan *DashboardMetrics, 2)
	a.subMu.Lock()
	a.subscribers[ch] = struct{}{}
	a.subMu.Unlock()
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func (a *Aggregator) Unsubscribe(ch chan *DashboardMetrics) {
	a.subMu.Lock()
	delete(a.subscribers, ch)
	a.subMu.Unlock()
	close(ch)
}

func (a *Aggregator) loop() {
	tickMetrics := time.NewTicker(time.Second)
	tickPrune := time.NewTicker(10 * time.Second)
	tickSessions := time.NewTicker(30 * time.Second)
	defer tickMetrics.Stop()
	defer tickPrune.Stop()
	defer tickSessions.Stop()

	for {
		select {
		case <-tickMetrics.C:
			m := a.recalculate()
			a.mu.Lock()
			a.currentMetrics = m
			a.mu.Unlock()
			a.broadcast(m)

		case <-tickPrune.C:
			a.pruneEvents()

		case <-tickSessions.C:
			a.pruneSessions()
		}
	}
}

func (a *Aggregator) broadcast(m *DashboardMetrics) {
	a.subMu.Lock()
	defer a.subMu.Unlock()
	for ch := range a.subscribers {
		select {
		case ch <- m:
		default: // drop if subscriber is slow
		}
	}
}

func (a *Aggregator) pruneEvents() {
	cutoff := time.Now().Add(-a.windowDuration)
	a.mu.Lock()
	defer a.mu.Unlock()
	i := 0
	for i < len(a.recentEvents) && a.recentEvents[i].at.Before(cutoff) {
		i++
	}
	a.recentEvents = a.recentEvents[i:]
}

func (a *Aggregator) pruneSessions() {
	threshold := time.Now().Add(-60 * time.Second)
	a.mu.Lock()
	defer a.mu.Unlock()
	for sessID, lastSeen := range a.activeSessions {
		if lastSeen.Before(threshold) {
			delete(a.activeSessions, sessID)
			delete(a.sessionToVideo, sessID)
		}
	}
	// Remove video title cache entries no longer referenced by any session.
	referenced := make(map[string]struct{}, len(a.sessionToVideo))
	for _, vid := range a.sessionToVideo {
		referenced[vid] = struct{}{}
	}
	for vid := range a.videoTitles {
		if _, ok := referenced[vid]; !ok {
			delete(a.videoTitles, vid)
		}
	}
}

func (a *Aggregator) recalculate() *DashboardMetrics {
	a.mu.RLock()
	liveCounter := a.liveCounter
	events := make([]timedEvent, len(a.recentEvents))
	copy(events, a.recentEvents)
	activeSessions := make(map[string]time.Time, len(a.activeSessions))
	for k, v := range a.activeSessions {
		activeSessions[k] = v
	}
	sessionToVideo := make(map[string]string, len(a.sessionToVideo))
	for k, v := range a.sessionToVideo {
		sessionToVideo[k] = v
	}
	titles := make(map[string]string, len(a.videoTitles))
	for k, v := range a.videoTitles {
		titles[k] = v
	}
	a.mu.RUnlock()

	now := time.Now()
	m := emptyMetrics()
	m.Timestamp = now

	// Active sessions (heartbeat within 60s)
	for _, t := range activeSessions {
		if now.Sub(t) < 60*time.Second {
			m.ActiveSessions++
		}
	}

	// Process events
	sessionSet := make(map[string]struct{})
	rebufferSessions := make(map[string]struct{})
	var ttffValues []int
	var rebufferDurations []int
	qualityCounts := make(map[string]int)
	totalQualityObs := 0
	var throughputValues []float64
	var qualitySwitches int

	// Per-video accumulators
	type vidStats struct {
		qualityObs  map[string]int
		totalObs    int
		rebufSess   map[string]struct{}
		totalSess   map[string]struct{}
	}
	videoStats := make(map[string]*vidStats)

	ensureVideo := func(vid string) {
		if _, ok := videoStats[vid]; !ok {
			videoStats[vid] = &vidStats{
				qualityObs: make(map[string]int),
				rebufSess:  make(map[string]struct{}),
				totalSess:  make(map[string]struct{}),
			}
		}
	}

	for _, te := range events {
		e := te.event
		sessionSet[e.SessionID] = struct{}{}

		if e.VideoID != "" {
			ensureVideo(e.VideoID)
			videoStats[e.VideoID].totalSess[e.SessionID] = struct{}{}
		}

		switch e.EventType {
		case "playback_start":
			if e.DownloadTimeMs != nil && *e.DownloadTimeMs > 0 {
				ttffValues = append(ttffValues, *e.DownloadTimeMs)
			}
		case "rebuffer_start":
			rebufferSessions[e.SessionID] = struct{}{}
			if e.VideoID != "" {
				ensureVideo(e.VideoID)
				videoStats[e.VideoID].rebufSess[e.SessionID] = struct{}{}
			}
		case "rebuffer_end":
			if e.RebufferDurationMs != nil {
				rebufferDurations = append(rebufferDurations, *e.RebufferDurationMs)
			}
		case "heartbeat":
			if e.CurrentQuality != "" {
				qualityCounts[e.CurrentQuality]++
				totalQualityObs++
				if e.VideoID != "" {
					ensureVideo(e.VideoID)
					videoStats[e.VideoID].qualityObs[e.CurrentQuality]++
					videoStats[e.VideoID].totalObs++
				}
			}
		case "segment_downloaded":
			if e.ThroughputBps != nil {
				throughputValues = append(throughputValues, float64(*e.ThroughputBps)/1_000_000)
			}
		case "quality_change":
			qualitySwitches++
		}
	}

	m.TotalSessionsLast5m = len(sessionSet)

	if len(ttffValues) > 0 {
		sort.Ints(ttffValues)
		m.TTFFMedianMs = percentile(ttffValues, 0.50)
		m.TTFFP95Ms = percentile(ttffValues, 0.95)
	}

	if len(sessionSet) > 0 {
		m.RebufferRate = float64(len(rebufferSessions)) / float64(len(sessionSet))
	}
	if len(rebufferDurations) > 0 {
		sum := 0
		for _, d := range rebufferDurations {
			sum += d
		}
		m.AvgRebufferDurationMs = sum / len(rebufferDurations)
	}

	if totalQualityObs > 0 {
		var weightedBitrate float64
		for q, count := range qualityCounts {
			pct := float64(count) / float64(totalQualityObs)
			m.QualityDistribution[q] = pct
			if br, ok := qualityBitrates[q]; ok {
				weightedBitrate += pct * br
			}
		}
		m.AvgBitrateKbps = weightedBitrate
	}

	if len(events) > 0 {
		windowMin := a.windowDuration.Minutes()
		m.QualitySwitchesPerMin = float64(qualitySwitches) / windowMin
	}

	if len(throughputValues) > 0 {
		sum := 0.0
		for _, t := range throughputValues {
			sum += t
		}
		m.AvgThroughputMbps = sum / float64(len(throughputValues))
		sorted := make([]float64, len(throughputValues))
		copy(sorted, throughputValues)
		sort.Float64s(sorted)
		m.P10ThroughputMbps = sorted[int(float64(len(sorted)-1)*0.10)]
	}

	// Per-video breakdown — active sessions per video
	videoActiveCounts := make(map[string]int)
	for sessID, lastSeen := range activeSessions {
		if now.Sub(lastSeen) < 60*time.Second {
			if vid, ok := sessionToVideo[sessID]; ok {
				videoActiveCounts[vid]++
			}
		}
	}

	var perVideo []VideoMetrics
	for vid, active := range videoActiveCounts {
		vm := VideoMetrics{
			VideoID:        vid,
			Title:          titles[vid],
			ActiveSessions: active,
		}
		if vs, ok := videoStats[vid]; ok && vs.totalObs > 0 {
			var wb float64
			for q, cnt := range vs.qualityObs {
				if br, ok := qualityBitrates[q]; ok {
					wb += float64(cnt) / float64(vs.totalObs) * br
				}
			}
			vm.AvgBitrateKbps = wb
			if len(vs.totalSess) > 0 {
				vm.RebufferRate = float64(len(vs.rebufSess)) / float64(len(vs.totalSess))
			}
		}
		perVideo = append(perVideo, vm)
	}
	sort.Slice(perVideo, func(i, j int) bool {
		return perVideo[i].ActiveSessions > perVideo[j].ActiveSessions
	})
	if len(perVideo) > 10 {
		perVideo = perVideo[:10]
	}
	m.PerVideo = perVideo

	if liveCounter != nil {
		m.ActiveLiveStreams = liveCounter.ActiveCount()
	}

	return m
}

func (a *Aggregator) fetchTitle(videoID string) {
	if a.videoRepo == nil {
		return
	}
	if vid, _ := a.videoRepo.GetByID(videoID); vid != nil {
		a.mu.Lock()
		a.videoTitles[videoID] = vid.Title
		a.mu.Unlock()
	}
}

func emptyMetrics() *DashboardMetrics {
	return &DashboardMetrics{
		QualityDistribution: make(map[string]float64),
		PerVideo:            []VideoMetrics{},
	}
}

func percentile(sorted []int, pct float64) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * pct)
	return sorted[idx]
}

