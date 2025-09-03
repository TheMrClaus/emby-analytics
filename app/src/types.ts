// app/src/types.ts
export type UsageRow = { day: string; user: string; hours: number };

export type TopUser = { user_id?: string; name: string; hours: number };
// Compatible with backend 'TopUser' and your previous UI 'TopUser' shape.

export type TopItem = { item_id: string; name: string; type: string; hours: number; display?: string };

export type ItemRow = { id: string; name?: string; type?: string; display?: string };

export type RefreshState = {
  running: boolean;
  imported: number;
  total?: number;
  page: number;
  error: string | null;
};

// Now Playing (UI-friendly subset)
export type NowEntry = {
  timestamp: number;
  title: string;
  user: string;
  app: string;
  device: string;
  play_method: string;
  video: string;
  audio: string;
  subs: string;
  bitrate: number;
  progress_pct: number;
  position_sec?: number;
  duration_sec?: number;
  poster: string;
  session_id: string;
  item_id: string;
  item_type?: string;
  container?: string;
  width?: number;
  height?: number;
  dolby_vision?: boolean;
  hdr10?: boolean;
};

export type PlayMethodCounts = {
  methods: {
    DirectPlay?: number;
    DirectStream?: number;
    Transcode?: number;
    Unknown?: number;
    [k: string]: number | undefined;
  };
};

// Stats responses
export type OverviewData = {
  total_users: number;
  total_items: number;
  total_plays: number;
  unique_plays: number;
};

export type QualityBuckets = {
  buckets: Record<string, { Movie: number; Episode: number }>;
};

export type CodecBuckets = {
  codecs: Record<string, { Movie: number; Episode: number }>;
};

export type ActiveUserLifetime = {
  user: string;
  days: number;
  hours: number;
  minutes: number;
};
