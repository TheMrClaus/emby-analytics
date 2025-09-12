package now

import (
    "math"
    "strings"
    "sync"

    "github.com/gofiber/fiber/v3"
)

// NowPlayingSummary is a compact metrics payload for the Now Playing header.
// outbound_mbps is a 5-sample rolling average of all active session bitrates.
type NowPlayingSummary struct {
    OutboundMbps     float64 `json:"outbound_mbps"`
    ActiveStreams    int     `json:"active_streams"`
    ActiveTranscodes int     `json:"active_transcodes"`
}

// ring buffer for smoothing outbound_mbps (approx 5s window at 1s+ polling)
type mbpsRing struct {
    mu   sync.Mutex
    buf  []float64
    next int
    size int
}

func newMbpsRing(n int) *mbpsRing {
    if n <= 0 {
        n = 5
    }
    return &mbpsRing{buf: make([]float64, n), next: 0, size: 0}
}

func (r *mbpsRing) add(v float64) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.buf[r.next] = v
    r.next = (r.next + 1) % len(r.buf)
    if r.size < len(r.buf) {
        r.size++
    }
}

func (r *mbpsRing) avgOr(v float64) float64 {
    r.mu.Lock()
    defer r.mu.Unlock()
    if r.size == 0 {
        return v
    }
    sum := 0.0
    for i := 0; i < r.size; i++ {
        sum += r.buf[i]
    }
    return sum / float64(r.size)
}

var summaryRing = newMbpsRing(5)

// Summary computes the lightweight metrics for the Now Playing header.
// GET /api/now-playing/summary
func Summary(c fiber.Ctx) error {
    em, err := getEmbyClient()
    if err != nil {
        // If server is not configured, return zeros gracefully
        return c.Status(fiber.StatusOK).JSON(NowPlayingSummary{})
    }

    sessions, err := em.GetActiveSessions()
    if err != nil {
        // On upstream error, still return zeros to avoid breaking UI
        return c.Status(fiber.StatusOK).JSON(NowPlayingSummary{})
    }

    active := 0
    transcodes := 0
    var sumBps int64

    for _, s := range sessions {
        // Active stream: not paused (buffering isn't exposed; best effort)
        if s.IsPaused {
            continue
        }
        active++

        // Determine if this session is actually transcoding (re-encoding)
        // We intentionally do NOT count remux-only sessions (PlayMethod=Transcode but codecs copied)
        isTrans := false
        if strings.EqualFold(s.VideoMethod, "Transcode") || strings.EqualFold(s.AudioMethod, "Transcode") {
            isTrans = true
        } else if len(s.TransReasons) > 0 {
            // Heuristic: subtitles/burn-in indicated by reasons
            for _, r := range s.TransReasons {
                rr := strings.ToLower(r)
                if strings.Contains(rr, "subtitle") || strings.Contains(rr, "burn") {
                    isTrans = true
                    break
                }
            }
        }
        if isTrans {
            transcodes++
        }

        // Bitrate selection: prefer overall session bitrate; fallback to target A/V bitrates
        bps := s.Bitrate
        if bps <= 0 {
            if s.TransVideoBitrate > 0 || s.TransAudioBitrate > 0 {
                bps = s.TransVideoBitrate + s.TransAudioBitrate
            }
        }
        if bps > 0 {
            sumBps += bps
        }
    }

    // Convert to Mbps, round to 1 decimal
    mbps := float64(sumBps) / 1_000_000.0
    if mbps < 0 {
        mbps = 0
    }
    // Smooth over last ~5 samples
    summaryRing.add(mbps)
    avg := summaryRing.avgOr(mbps)
    avg = math.Round(avg*10) / 10

    return c.JSON(NowPlayingSummary{
        OutboundMbps:     avg,
        ActiveStreams:    active,
        ActiveTranscodes: transcodes,
    })
}
