# Tracearr Analytics-Focused Competitive Analysis

**Date:** 2026-04-01  
**Project:** emby-analytics  
**Focus:** Media consumption analytics, library insights, and data visualization  
**Competitor:** Tracearr (https://github.com/connorgallopo/Tracearr)

---

## Executive Summary

This analysis focuses on **Tracearr's analytics capabilities** for media consumption, library management, and data visualization — ignoring sharing detection features.

### Key Analytics Gaps in emby-analytics

1. ❌ **Library Storage Analytics** - Growth predictions, stale content, ROI analysis
2. ❌ **Library Watch Analytics** - Engagement metrics, completion rates, binge detection
3. ❌ **Advanced Quality Analytics** - Quality trending over time, codec evolution
4. ❌ **Interactive Stream Map** - Geographic visualization of viewership
5. ❌ **Duplicate Detection** - Find same content across servers
6. ❌ **Viewing Patterns** - Heatmaps by hour/day/month
7. ❌ **Content ROI** - Watch hours per GB stored
8. ❌ **Live TV & Music Analytics** - Non-movie/TV content tracking
9. ❌ **Multi-Server Analytics** - Aggregate stats across servers
10. ❌ **TimescaleDB Performance** - Pre-computed aggregates for fast dashboards

---

## What Tracearr Does Better (Analytics Focus)

### 1. Four Dedicated Library Analytics Pages ⭐

#### a) Library Overview
**What they show:**
- Item counts by type (Movies, Series, Episodes)
- Storage usage with growth charts over time
- Collection growth timeline (items added per week/month)
- Total size trending

**What we have:**
- Basic item counts
- No growth tracking
- No timeline visualization

**Implementation needed:**
- Track library snapshots over time
- Store `library_snapshots` table with daily/weekly counts
- Chart: items added per time period
- Chart: storage growth over time

---

#### b) Library Quality
**What they show:**
- Resolution distribution (4K/1080p/720p/SD) **with trending**
- Codec distribution **with trending**
- Quality ratio changes over time (e.g., "4K content increased 15% this month")
- Video/audio codec evolution charts

**What we have:**
- Static quality breakdown (current state only)
- Static codec distribution
- No historical trending

**Implementation needed:**
- Store quality/codec snapshots over time
- Chart: 4K vs 1080p ratio trending (line chart)
- Chart: New codec adoption (AV1, HEVC growth)
- Show percentage changes month-over-month

---

#### c) Library Storage ⭐ HIGH VALUE
**What they show:**
- **Storage predictions** - ML-based growth forecasting
  - "At current rate, you'll reach 10TB in 6 months"
  - Linear regression on historical growth
- **Duplicate detection** - Same content across servers
  - "Movie X exists on Server A and Server B"
  - File hash or filename matching
- **Stale content identification** - Never watched items
  - "427 items have zero plays"
  - "Remove these to free 2.3TB"
- **ROI analysis** - Watch hours per GB stored
  - "Movie X: 243 watch hours, 15GB = 16.2 hours/GB"
  - "Movie Y: 0 watch hours, 45GB = 0 hours/GB (candidate for removal)"

**What we have:**
- Basic storage totals
- No predictions
- No duplicate detection
- No stale content tracking
- No ROI metrics

**Implementation needed:**
- Historical storage tracking (daily snapshots)
- Linear regression for predictions
- Duplicate detection: `GROUP BY normalized_filename HAVING COUNT(*) > 1`
- Stale content: `LEFT JOIN play_sessions ... WHERE play_sessions.id IS NULL`
- ROI calculation: `SUM(watch_seconds) / file_size_gb`
- Admin page: "Stale Content" with sortable ROI table

---

#### d) Library Watch ⭐ HIGH VALUE
**What they show:**
- **Engagement metrics**
  - Completion rates per title (% who finished vs started)
  - Average watch percentage per item
  - "Episode 1 has 95% completion, Episode 8 has 40% (people drop off)"
- **Viewing patterns** - Heatmaps
  - Watch activity by hour of day (0-23)
  - Watch activity by day of week (Mon-Sun)
  - Watch activity by month (Jan-Dec)
  - Peak viewing hours highlighted
- **Binge detection**
  - "User watched 8 episodes of Series X in 12 hours"
  - Marathon viewing sessions (3+ episodes, same series, <24h)
- **Popular content with trends**
  - Most-watched items (7-day, 30-day windows)
  - Trending indicators: "↑ 45% vs last week"
  - "Rising stars" - new content gaining traction

**What we have:**
- Basic "top items" (all-time)
- No completion rates
- No viewing pattern heatmaps
- No binge detection
- No trending indicators

**Implementation needed:**
- Completion rate calculation:
  ```sql
  SELECT item_id,
         COUNT(DISTINCT user_id) as total_viewers,
         COUNT(DISTINCT CASE WHEN watch_percentage > 90 THEN user_id END) as completions,
         (completions * 100.0 / total_viewers) as completion_rate
  FROM play_sessions
  GROUP BY item_id
  ```
- Heatmap data:
  ```sql
  SELECT strftime('%H', started_at) as hour,
         COUNT(*) as play_count
  FROM play_sessions
  GROUP BY hour
  ```
- Binge detection:
  ```sql
  SELECT user_id, series_id, COUNT(*) as episode_count,
         MIN(started_at) as first_watch,
         MAX(started_at) as last_watch
  FROM play_sessions
  WHERE item_type = 'Episode'
  GROUP BY user_id, series_id, DATE(started_at)
  HAVING episode_count >= 3
  ```
- Trending calculation: Compare last 7 days vs previous 7 days

---

### 2. Multi-Server Analytics

**What they do:**
- Aggregate stats across Plex + Jellyfin + Emby servers
- Cross-server duplicate detection
- Unified "top content" across all servers
- Server comparison charts ("Server A: 45% 4K, Server B: 20% 4K")

**Our limitation:**
- Single server only

**Implementation (if you add multi-server later):**
- Add `server_id` to all tables
- Aggregate queries: `GROUP BY server_id`
- Server selector dropdown in header
- "All Servers" view with combined stats

---

### 3. TimescaleDB for Performance ⭐ IMPORTANT

**Why they use it:**
- PostgreSQL extension optimized for time-series data
- **Hypertables** - Automatic partitioning by time (old data in compressed partitions)
- **Continuous aggregates** - Pre-computed rollups refresh incrementally
  - Dashboard stats computed once, updated in background
  - Instead of `SELECT COUNT(*) FROM play_sessions` on every page load
  - They query pre-computed `daily_stats` table

**Example continuous aggregate:**
```sql
CREATE MATERIALIZED VIEW daily_stats
WITH (timescaledb.continuous) AS
SELECT 
  time_bucket('1 day', started_at) as day,
  COUNT(*) as total_plays,
  COUNT(DISTINCT user_id) as unique_users,
  SUM(watch_seconds) as total_watch_seconds
FROM play_sessions
GROUP BY day;

-- Query takes milliseconds instead of seconds
SELECT * FROM daily_stats WHERE day > NOW() - INTERVAL '30 days';
```

**Our limitation:**
- SQLite = full table scans every time
- No automatic partitioning
- Slow queries as data grows

**When to migrate:**
- Users report slow dashboard loads
- Dataset > 100k sessions
- Query times > 2 seconds

**Migration effort:** 10-15 days (high complexity)

---

### 4. Live TV & Music Analytics

**What they track:**
- Live TV sessions with channel info
- Music playback with album/artist metadata
- Separate analytics pages:
  - "Top Music Artists"
  - "Top Live TV Channels"
  - "Music listening patterns"

**Our limitation:**
- Movies and TV episodes only
- `MediaType` filtering ignores LiveTV/Music

**Implementation needed:**
- Detect `MediaType` from Emby API
- Add `media_type` column to `library_item`
- Separate stats endpoints:
  - `GET /stats/music/top-artists`
  - `GET /stats/livetv/top-channels`
- UI pages: `/music`, `/livetv`

**Effort:** 5-6 days

---

### 5. Enhanced Chart Visualizations

**What they use: Highcharts**
- Interactive, professional charts
- Built-in tooltips, zoom, export
- Better performance for large datasets

**What we use: Nivo**
- Open source, React-native
- Good for basic charts
- Can be slow with 1000+ data points

**Comparison:**

| Feature | Nivo | Highcharts |
|---------|------|------------|
| License | MIT (free) | Commercial ($$$) |
| Performance | Good (<1000 points) | Excellent (10k+ points) |
| Interactivity | Basic | Advanced |
| Chart types | 20+ | 50+ |
| Time series | Manual | Built-in |

**Recommendation:** **Stick with Nivo** unless performance becomes an issue. Highcharts requires paid license for commercial use.

---

### 6. Data Import from Competitors

**What they support:**
- Import watch history from **Tautulli** (Plex)
- Import watch history from **Jellystat** (Jellyfin/Emby)
- Users don't lose historical data when switching

**Our limitation:**
- No import capability
- Users start from scratch

**Implementation:**
- Parse Tautulli SQLite database
- Parse Jellystat database
- Map schemas to emby-analytics tables
- Admin page: "Import Data" with file upload + progress bar

**Effort:** 4-5 days

**Value:** Easier onboarding, users keep their history

---

### 7. Bulk Actions for Data Management

**What they support:**
- Multi-select in tables
- Bulk delete sessions
- Bulk mark sessions as "exclude from stats"
- Bulk re-sync metadata

**Our limitation:**
- Individual row actions only

**Implementation:**
- Checkbox column in all tables
- "Select All" / "Deselect All" buttons
- Bulk action dropdown: Delete, Exclude, Re-sync
- Confirmation dialog with count

**Effort:** 3-4 days

**Value:** Admin efficiency, especially for cleanup

---

## Analytics Implementation Roadmap

### Phase 1: Storage & ROI Analytics (v0.5.0) - 1 WEEK ⭐

**1.1 Library Storage Analytics Page** (4-5 days)
- **Stale content identification:**
  ```sql
  SELECT li.*, 0 as play_count
  FROM library_item li
  LEFT JOIN play_sessions ps ON li.id = ps.library_item_id
  WHERE ps.id IS NULL
  ORDER BY li.file_size DESC
  ```
  - Table: Item name, size, added date, "Never watched"
  - Sort by size (biggest first)
  - Total reclaimable space
  
- **ROI analysis:**
  ```sql
  SELECT 
    li.*,
    COUNT(ps.id) as play_count,
    SUM(ps.watch_seconds) / 3600.0 as total_watch_hours,
    li.file_size / 1073741824.0 as size_gb,
    (SUM(ps.watch_seconds) / 3600.0) / (li.file_size / 1073741824.0) as hours_per_gb
  FROM library_item li
  LEFT JOIN play_sessions ps ON li.id = ps.library_item_id
  WHERE li.file_size > 0
  GROUP BY li.id
  ORDER BY hours_per_gb DESC
  ```
  - Table: Item, watch hours, size, ROI (hours/GB)
  - Color-code: Green (high ROI), Red (low ROI)
  - Filter: Show items with 0 plays
  
- **Duplicate detection:**
  ```sql
  -- Option 1: Normalized filename
  SELECT 
    LOWER(REPLACE(REPLACE(file_path, '\', '/'), '//', '/')) as normalized_path,
    COUNT(*) as duplicate_count,
    GROUP_CONCAT(id) as item_ids
  FROM library_item
  WHERE file_path IS NOT NULL
  GROUP BY normalized_path
  HAVING duplicate_count > 1
  ```
  - Table: Filename, duplicate count, size wasted
  - "Remove duplicates" action
  
- **Storage predictions:**
  - Track daily snapshots in new table:
  ```sql
  CREATE TABLE library_snapshots (
    id INTEGER PRIMARY KEY,
    date DATE NOT NULL,
    total_items INTEGER,
    total_size_bytes INTEGER,
    movie_count INTEGER,
    series_count INTEGER,
    episode_count INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  ```
  - Background job: Insert snapshot daily
  - Linear regression:
  ```javascript
  const snapshots = await db.query('SELECT date, total_size_bytes FROM library_snapshots ORDER BY date');
  const regression = calculateLinearRegression(snapshots);
  const predictedSize = regression.predict(futureDate);
  ```
  - Chart: Storage over time + predicted line
  - Text: "At current rate, 10TB reached on [date]"

**Impact:** Help admins optimize storage, identify content to remove

---

### Phase 2: Engagement Analytics (v0.6.0) - 1 WEEK ⭐

**2.1 Library Watch Analytics Page** (3-4 days)

- **Completion rates:**
  - Calculate per-item completion percentage
  - Bar chart: Items sorted by completion rate
  - Filter: Movies vs Episodes
  - Insight: "Episode 5 has 60% completion (people drop off here)"

- **Viewing pattern heatmaps:**
  - Hour-of-day heatmap (0-23 hours)
  - Day-of-week chart (Mon-Sun)
  - Month chart (Jan-Dec)
  - Use Nivo's heatmap component
  ```javascript
  const hourData = sessions.reduce((acc, s) => {
    const hour = new Date(s.started_at).getHours();
    acc[hour] = (acc[hour] || 0) + 1;
    return acc;
  }, {});
  ```

- **Binge detection dashboard:**
  - Table: User, series, episode count, time span
  - Filter: Min 3 episodes in <24h
  - Chart: Binge sessions over time

**2.2 Trending Indicators** (2 days)
- Add 7-day and 30-day windows to "Top Items"
- Calculate % change vs previous period
- Show ↑/↓ arrows with percentage
- "Rising stars" section: New content trending up

**Impact:** Understand what content engages users

---

### Phase 3: Historical Trending (v0.7.0) - 1 WEEK

**3.1 Quality/Codec Trending** (3-4 days)
- Snapshot quality distribution weekly
- Chart: 4K vs 1080p ratio over time (line chart)
- Chart: Codec adoption (HEVC, AV1 growth)
- Text insights: "4K content up 12% this month"

**3.2 Growth Timeline** (2-3 days)
- Chart: Items added per week/month
- Chart: Storage added per week/month
- "Biggest growth periods" - identify library expansion dates

**Impact:** Track library evolution over time

---

### Phase 4: Extended Support (v0.8.0) - 1 WEEK

**4.1 Live TV & Music Support** (5-6 days)
- Detect MediaType from Emby API
- Add dedicated analytics pages
- Track channels, artists, albums
- Music listening patterns

**4.2 Bulk Actions** (3-4 days)
- Multi-select in session tables
- Bulk delete, exclude from stats
- Confirmation dialogs

**Impact:** Support full Emby feature set

---

### Phase 5: Performance Optimization (v0.9.0+) - LATER

**5.1 Redis Caching** (3-4 days)
- Cache overview stats (5 min TTL)
- Cache top users/items (10 min TTL)
- Cache active sessions (30 sec TTL)

**5.2 TimescaleDB Migration** (10-15 days)
- ONLY if users report slow queries
- Requires PostgreSQL deployment
- Pre-computed aggregates for fast dashboards

**5.3 Data Import** (4-5 days)
- Tautulli import
- Jellystat import

---

## Recommended Priority Order

### Immediate (Next 2-3 weeks)

1. **Storage Analytics** (Phase 1) - 1 week ⭐
   - ROI analysis
   - Stale content detection
   - Duplicate detection
   - Storage predictions
   - **Why first:** Helps admins make decisions NOW

2. **Engagement Analytics** (Phase 2) - 1 week ⭐
   - Completion rates
   - Viewing patterns
   - Binge detection
   - Trending indicators
   - **Why second:** Most requested analytics feature

3. **Historical Trending** (Phase 3) - 1 week
   - Quality/codec evolution
   - Growth timeline
   - **Why third:** Nice-to-have context

### Medium-term (1-2 months)

4. **Live TV & Music** (Phase 4) - 1 week
   - Full Emby feature support
   - Music analytics

5. **Bulk Actions** (Phase 4) - 3-4 days
   - Admin efficiency

### Long-term (3+ months)

6. **Performance** (Phase 5) - DEFER
   - Redis caching
   - TimescaleDB migration (only if needed)

7. **Data Import** (Phase 5) - 4-5 days
   - Tautulli/Jellystat import

---

## Database Schema Changes

### New Tables

```sql
-- Daily library snapshots
CREATE TABLE library_snapshots (
  id INTEGER PRIMARY KEY,
  snapshot_date DATE NOT NULL UNIQUE,
  total_items INTEGER NOT NULL,
  total_size_bytes INTEGER NOT NULL,
  movie_count INTEGER,
  series_count INTEGER,
  episode_count INTEGER,
  video_4k_count INTEGER,
  video_1080p_count INTEGER,
  video_720p_count INTEGER,
  video_sd_count INTEGER,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Track codec evolution
CREATE TABLE codec_snapshots (
  id INTEGER PRIMARY KEY,
  snapshot_date DATE NOT NULL,
  video_codec TEXT NOT NULL,
  item_count INTEGER NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(snapshot_date, video_codec)
);

-- Optional: Mark sessions as excluded from stats
ALTER TABLE play_sessions ADD COLUMN excluded_from_stats BOOLEAN DEFAULT FALSE;
```

### Indexes

```sql
CREATE INDEX idx_library_snapshots_date ON library_snapshots(snapshot_date);
CREATE INDEX idx_codec_snapshots_date_codec ON codec_snapshots(snapshot_date, video_codec);
CREATE INDEX idx_play_sessions_started_at ON play_sessions(started_at);
CREATE INDEX idx_play_sessions_item_id ON play_sessions(library_item_id);
```

---

## Technical Considerations

### Storage Predictions Algorithm

**Linear Regression Implementation:**

```javascript
function calculateLinearRegression(dataPoints) {
  // dataPoints = [{x: Date, y: size_bytes}, ...]
  const n = dataPoints.length;
  const sumX = dataPoints.reduce((sum, p) => sum + p.x.getTime(), 0);
  const sumY = dataPoints.reduce((sum, p) => sum + p.y, 0);
  const sumXY = dataPoints.reduce((sum, p) => sum + p.x.getTime() * p.y, 0);
  const sumX2 = dataPoints.reduce((sum, p) => sum + p.x.getTime() ** 2, 0);
  
  const slope = (n * sumXY - sumX * sumY) / (n * sumX2 - sumX ** 2);
  const intercept = (sumY - slope * sumX) / n;
  
  return {
    predict: (date) => slope * date.getTime() + intercept,
    slope,
    intercept
  };
}

// Usage
const snapshots = await getLibrarySnapshots(); // Last 90 days
const regression = calculateLinearRegression(snapshots);
const in6Months = new Date();
in6Months.setMonth(in6Months.getMonth() + 6);
const predictedSize = regression.predict(in6Months);
console.log(`Predicted size in 6 months: ${predictedSize / 1e12} TB`);
```

---

### Completion Rate Calculation

```sql
-- Episode completion rates (detect drop-off points)
SELECT 
  s.name as series_name,
  e.season_number,
  e.episode_number,
  COUNT(DISTINCT ps.user_id) as total_viewers,
  COUNT(DISTINCT CASE WHEN ps.watch_percentage > 90 THEN ps.user_id END) as completions,
  CAST(COUNT(DISTINCT CASE WHEN ps.watch_percentage > 90 THEN ps.user_id END) AS FLOAT) * 100 / 
    COUNT(DISTINCT ps.user_id) as completion_rate
FROM library_item e
JOIN library_item s ON e.parent_id = s.id
JOIN play_sessions ps ON e.id = ps.library_item_id
WHERE e.item_type = 'Episode'
GROUP BY s.id, e.season_number, e.episode_number
HAVING total_viewers > 5  -- Min viewers threshold
ORDER BY s.name, e.season_number, e.episode_number;
```

**Use case:** Identify episodes where viewers drop off

---

### Viewing Pattern Heatmap Data

```sql
-- Hour-of-day heatmap
SELECT 
  CAST(strftime('%H', started_at) AS INTEGER) as hour,
  COUNT(*) as play_count,
  SUM(watch_seconds) / 3600.0 as total_hours
FROM play_sessions
WHERE started_at > datetime('now', '-30 days')
GROUP BY hour
ORDER BY hour;

-- Day-of-week heatmap
SELECT 
  CASE CAST(strftime('%w', started_at) AS INTEGER)
    WHEN 0 THEN 'Sunday'
    WHEN 1 THEN 'Monday'
    WHEN 2 THEN 'Tuesday'
    WHEN 3 THEN 'Wednesday'
    WHEN 4 THEN 'Thursday'
    WHEN 5 THEN 'Friday'
    WHEN 6 THEN 'Saturday'
  END as day_of_week,
  COUNT(*) as play_count
FROM play_sessions
WHERE started_at > datetime('now', '-90 days')
GROUP BY strftime('%w', started_at)
ORDER BY CAST(strftime('%w', started_at) AS INTEGER);
```

---

## Timeline Summary

| Phase | Duration | Features | Impact |
|-------|----------|----------|--------|
| **Phase 1** | 1 week | Storage analytics, ROI, stale content, predictions | ⭐⭐⭐ HIGH |
| **Phase 2** | 1 week | Engagement, completion rates, binge detection | ⭐⭐⭐ HIGH |
| **Phase 3** | 1 week | Historical trending, quality evolution | ⭐⭐ MEDIUM |
| **Phase 4** | 1 week | Live TV/Music, bulk actions | ⭐ LOW |
| **Phase 5** | Later | Performance, caching, imports | 🔮 FUTURE |

**Total to analytics parity:** 3-4 weeks of focused development

---

## Open Questions

1. **Storage predictions:** How far into future? (3 months? 6 months? 1 year?)

2. **Stale content threshold:** What counts as "stale"?
   - Never watched? Or
   - Not watched in last 12 months?

3. **ROI calculation:** Should we weight by:
   - Total watch hours? (current approach)
   - Unique viewers?
   - Recent views vs old views?

4. **Duplicate detection:** How to identify duplicates?
   - Filename matching? (simple, fast)
   - File hash? (accurate, slow)
   - TMDB/TVDB ID? (requires metadata)

5. **Binge threshold:** What counts as "binge"?
   - 3+ episodes in 24h? (current)
   - 2+ episodes in 12h?
   - Configurable by admin?

6. **Historical data:** Start tracking snapshots immediately, or backfill?
   - Immediate: Only future predictions work
   - Backfill: Approximate past data from current state

---

## Conclusion

**Focus on what Tracearr does for analytics:**

### Top 3 Priorities (Analytics-Focused)

1. **Storage Analytics** (Phase 1) - 1 week
   - ROI analysis, stale content, predictions
   - **Why:** Actionable insights for admins NOW

2. **Engagement Analytics** (Phase 2) - 1 week
   - Completion rates, viewing patterns
   - **Why:** Most requested feature, shows what content works

3. **Historical Trending** (Phase 3) - 1 week
   - Quality evolution, growth timeline
   - **Why:** Context on library changes

**Skip for now:**
- ❌ Sharing detection (not your focus)
- ❌ Trust scores (not relevant)
- ❌ Geolocation/maps (not needed for analytics)
- ❌ Mobile app (defer)

**Recommended Start:** Phase 1 (Storage Analytics) - highest value, helps admins make decisions today.
