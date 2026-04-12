# Tracearr Competitive Analysis & Implementation Plan

**Date:** 2026-04-01  
**Project:** emby-analytics  
**Competitor:** Tracearr (https://github.com/connorgallopo/Tracearr)  
**Stars:** 1.7k | **Active Development:** Yes | **License:** AGPL-3.0

---

## Executive Summary

Tracearr is a comprehensive monitoring platform for Plex, Jellyfin, and Emby with **1.7k stars** and active development. Their key differentiator is **account sharing detection** with sophisticated rule-based systems that emby-analytics currently lacks.

### Critical Gaps in emby-analytics

1. ❌ **No sharing detection** - Tracearr's killer feature
2. ❌ **Single server only** - Tracearr supports multi-server management  
3. ❌ **No real-time alerts** - Tracearr has Discord webhooks + 80+ notification services
4. ❌ **Limited analytics** - Tracearr has 4 dedicated library analytics pages
5. ❌ **No mobile app** - Tracearr has iOS/Android apps (in beta)
6. ❌ **No public API** - Tracearr has Swagger-documented REST API
7. ❌ **No geolocation** - Tracearr maps streams globally with ASN/continent/postal data
8. ❌ **No trust scoring** - Automated user reputation system
9. ❌ **Single database** - Tracearr uses TimescaleDB for time-series optimization

---

## Tracearr's Key Features We Lack

### 1. Account Sharing Detection ⭐ CRITICAL GAP

**Six Rule Types:**

| Rule Type | Logic | Example | Implementation |
|-----------|-------|---------|----------------|
| **Impossible Travel** | Location A → B in impossible time | NYC → London in 30min | Haversine distance ÷ time delta > 800 km/h |
| **Simultaneous Locations** | Same user, multiple locations at once | Paris + Tokyo simultaneously | Active sessions geo-distance > 100km |
| **Device Velocity** | Too many IPs in short window | 10 IPs in 24h from 10 cities | Sliding window: unique IPs per period |
| **Concurrent Streams** | Per-user stream limits | 3 active sessions, limit is 2 | COUNT(active_sessions) > limit |
| **Geo Restrictions** | Block specific countries | Block all streams from blacklist | IP → Country, check blacklist |
| **Account Inactivity** | Alert when dormant | No streams in 90 days | NOW() - MAX(session_end) > threshold |

**Trust Scoring System:**
- Violations reduce user trust score automatically
- Low severity: -5 points | Medium: -15 | High: -30
- Admin can manually adjust scores
- Color-coded badges in UI
- Filter "suspicious users" by low trust score

---

### 2. Real-Time Alerts & Notifications

**Notification Channels:**
- **Discord Webhooks** - Rich embeds with violation details
- **Apprise Integration** - 80+ services (Telegram, Slack, Pushover, Email, SMS)
- **Custom Webhooks** - Generic HTTP POST for third-party integrations

**Trigger Events:**
- Rule violations (sharing detection)
- New user detected
- Server offline
- Transcoder overload

---

### 3. Interactive Stream Map

**Features:**
- Leaflet-based world map
- Real-time stream markers by geolocation
- Filter by user/server/time period
- Click markers → session details popup
- **Enhanced geolocation data:**
  - ASN (ISP/hosting provider)
  - Continent
  - Postal code
  - City + Country

---

### 4. Advanced Library Analytics (4 Pages)

#### a) Library Overview
- Item counts by type (Movies/Series/Episodes)
- Storage usage with growth charts over time

#### b) Library Quality  
- Resolution distribution (4K/1080p/720p/SD)
- Codec distribution analysis
- Quality ratio trending

#### c) Library Storage
- **Storage predictions** - ML-based growth forecasting
- **Duplicate detection** - Same file across servers
- **Stale content** - Never watched items
- **ROI analysis** - Watch hours per GB

#### d) Library Watch
- **Engagement metrics** - Completion rates per title
- **Viewing patterns** - Heatmaps by hour/day/month
- **Binge detection** - Marathon viewing sessions
- **Popular content** - Most-watched with trends

---

### 5. Multi-Server Support

- Single dashboard for Plex + Jellyfin + Emby
- Centralized user management across servers
- Library analytics aggregated
- Duplicate detection across servers

---

### 6. Public REST API

- Swagger UI at `/api-docs`
- Read-only endpoints for third-party integrations
- API key generation in Settings
- **Integrations:** Homarr, Home Assistant, custom dashboards

---

### 7. TimescaleDB for Performance

**Why they use it:**
- PostgreSQL extension for time-series data
- **Hypertables** - Automatic partitioning by time
- **Continuous aggregates** - Pre-computed rollups
- Fast queries even with millions of rows

**Our limitation:** SQLite struggles with large datasets over time

---

### 8. Mobile App (Beta)

- React Native (iOS + Android)
- Push notifications for violations
- Remote session control (pause/stop)

---

### 9. Data Import

- Import from Tautulli (Plex)
- Import from Jellystat (Jellyfin/Emby)
- Users don't lose historical data

---

### 10. Bulk Actions

- Multi-select operations across tables
- Bulk acknowledge/dismiss violations
- Bulk reset trust scores
- Bulk delete session history

---

## Technology Stack Comparison

| Layer | emby-analytics | Tracearr | Winner |
|-------|----------------|----------|--------|
| Backend | Go + Fiber v3 | Node.js + Fastify | **Tie** |
| Frontend | Next.js 15 + React 19 | React 19 + TypeScript | **Tie** |
| Database | SQLite | TimescaleDB | **Tracearr** |
| Cache | None | Redis | **Tracearr** |
| Real-time | WebSocket | Socket.io | **Tie** |
| Charts | Nivo | Highcharts | Preference |
| Maps | None | Leaflet | **Tracearr** |
| Mobile | None | React Native (beta) | **Tracearr** |

---

## Implementation Roadmap

### Phase 1: Foundation (v0.5.0) - 2 WEEKS

**1.1 Geolocation Infrastructure** (2-3 days)
- Add `ip_address` to `play_sessions`
- New table: `play_session_locations`
  - IP, country, city, lat/lon, ASN, ISP, continent, postal
- Integrate MaxMind GeoLite2 or ip-api.com
- Backfill existing sessions

**1.2 Sharing Detection Rules Engine** (5-7 days)
- New tables: `sharing_rules`, `sharing_violations`
- Background job: `detectSharingViolations()` every 60s
- Implement 6 rule types:
  - Impossible Travel (Haversine distance)
  - Simultaneous Locations
  - Device Velocity
  - Concurrent Streams
  - Geo Restrictions
  - Account Inactivity
- Admin UI: Rules config page
- Admin UI: Violations dashboard

**1.3 Trust Scoring System** (2 days)
- Add `trust_score` to `emby_user` (default 100)
- New table: `trust_score_history` (audit trail)
- Auto-deduct points on violations
- Admin manual adjustment UI
- Color-coded trust badges

**Total:** 9-12 days | **Impact:** KILLER FEATURE

---

### Phase 2: Alerts & Visual (v0.6.0) - 2 WEEKS

**2.1 Discord Webhooks** (2 days)
- New table: `notification_channels`
- Discord embed builder
- Send on: violations, new user, server offline

**2.2 Apprise Integration** (3 days)
- Integrate Go Apprise library
- Admin UI: Configure endpoints
- Support 80+ services

**2.3 Custom Webhooks** (1 day)
- Generic HTTP POST with JSON payload
- Retry logic for failures

**2.4 Interactive Stream Map** (3-4 days)
- Add Leaflet.js to frontend
- New page: `/map`
- Active + historical sessions as markers
- Filters: user, server, time range

**Total:** 9-10 days | **Impact:** Complete monitoring

---

### Phase 3: Advanced Analytics (v0.7.0) - 2 WEEKS

**3.1 Library Storage Analytics** (4-5 days)
- Storage predictions (linear regression)
- Duplicate detection
- Stale content identification
- ROI analysis (watch hours / GB)

**3.2 Library Watch Analytics** (3-4 days)
- Completion rates
- Viewing patterns heatmap
- Binge detection
- Popular content with trends

**3.3 Bulk Actions** (3-4 days)
- Multi-select in tables
- Bulk operations on violations/users/sessions

**Total:** 10-13 days | **Impact:** Advanced insights

---

### Phase 4: Ecosystem (v0.8.0) - 1 MONTH

**4.1 Multi-Server Support** (8-10 days)
- Add `server_id` FK to all tables
- Multi-server selector
- Cross-server analytics

**4.2 Public REST API** (5-6 days)
- Swagger/OpenAPI docs
- API key generation
- Rate limiting

**4.3 Redis Caching** (3-4 days)
- Cache overview stats (5 min TTL)
- Cache active sessions (30 sec TTL)

**4.4 Data Import** (4-5 days)
- Tautulli import
- Jellystat import

**Total:** 20-25 days | **Impact:** Scalability

---

### Phase 5: Future (v1.0.0+)

- **TimescaleDB Migration** (10-15 days)
- **Mobile App** (20-30 days)
- **Live TV & Music** (5-6 days)

---

## Database Schema Changes

### New Tables

```sql
-- Geolocation
CREATE TABLE play_session_locations (
  id INTEGER PRIMARY KEY,
  session_id INTEGER NOT NULL REFERENCES play_sessions(id) ON DELETE CASCADE,
  ip TEXT NOT NULL,
  country_code TEXT,
  country_name TEXT,
  city TEXT,
  latitude REAL,
  longitude REAL,
  asn INTEGER,
  isp TEXT,
  continent TEXT,
  postal_code TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Sharing rules
CREATE TABLE sharing_rules (
  id INTEGER PRIMARY KEY,
  rule_type TEXT NOT NULL,
  enabled BOOLEAN DEFAULT TRUE,
  threshold REAL NOT NULL,
  config_json TEXT,
  server_id TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Violations
CREATE TABLE sharing_violations (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES emby_user(id),
  rule_id INTEGER NOT NULL REFERENCES sharing_rules(id),
  session_id INTEGER REFERENCES play_sessions(id),
  severity TEXT NOT NULL,
  details_json TEXT NOT NULL,
  acknowledged BOOLEAN DEFAULT FALSE,
  acknowledged_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Trust score history
CREATE TABLE trust_score_history (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES emby_user(id),
  old_score INTEGER NOT NULL,
  new_score INTEGER NOT NULL,
  reason TEXT NOT NULL,
  reference_id INTEGER,
  admin_user TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Notifications
CREATE TABLE notification_channels (
  id INTEGER PRIMARY KEY,
  type TEXT NOT NULL,
  name TEXT NOT NULL,
  config_json TEXT NOT NULL,
  enabled BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Column Additions

```sql
ALTER TABLE emby_user ADD COLUMN trust_score INTEGER DEFAULT 100;
ALTER TABLE play_sessions ADD COLUMN ip_address TEXT;
```

---

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Geolocation API rate limits | Medium | Use MaxMind GeoLite2 (self-hosted, free) |
| False positives (VPN users) | Medium | Allow manual whitelisting, adjustable thresholds |
| Notification spam | Medium | Rate limiting, configurable cooldowns |
| Multi-server schema changes | High | Thorough testing, migration scripts |
| TimescaleDB migration | High | Defer until users request it |

---

## Open Questions

1. **Priority:** Which features matter most?
   - Sharing detection? (killer feature)
   - Multi-server support? (do you have multiple Emby servers?)
   - Mobile app? (would you use it?)

2. **Database:** Migrate SQLite → PostgreSQL?
   - Pro: Better performance for large datasets
   - Con: Adds deployment complexity

3. **Notifications:** Which services?
   - Discord, Telegram, Email, Pushover?

4. **Geolocation:** Privacy concerns with IP tracking?
   - Required for sharing detection

5. **Budget:** Paid geolocation API or free tier?
   - MaxMind GeoLite2 (free, less accurate)
   - ip-api.com (45 req/min free, unlimited paid)

---

## Timeline to Feature Parity

- **Phase 1 (v0.5.0):** 2 weeks - Sharing detection ⭐
- **Phase 2 (v0.6.0):** 2 weeks - Alerts + map ⭐
- **Phase 3 (v0.7.0):** 2 weeks - Advanced analytics
- **Phase 4 (v0.8.0):** 1 month - Multi-server + API

**TOTAL:** 3-4 months to full feature parity

---

## Recommendation

**Start with Phase 1 (Sharing Detection) immediately.**

This is Tracearr's killer feature and the biggest gap in emby-analytics. Implementing geolocation + 6 rule types + trust scoring in 2 weeks would:

- ✅ Differentiate from basic monitoring tools
- ✅ Attract users concerned about account sharing
- ✅ Establish foundation for all future alert features
- ✅ Provide immediate competitive advantage

**Next steps:**
1. Confirm priorities with user
2. Design geolocation integration (MaxMind vs API)
3. Design sharing rules UI/UX
4. Create migration for new schema
5. Start implementation
