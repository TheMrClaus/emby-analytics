// app/src/types.ts
export type UsageRow = { day: string; user: string; hours: number };

export type TopUser = { user_id?: string; name: string; hours: number };

export type UserDetail = {
  user_id: string;
  user_name: string;
  total_hours: number;
  plays: number;
  total_movies: number;
  total_series_finished: number;
  total_episodes: number;
  top_items: UserTopItem[];
  recent_activity: UserActivity[];
  last_seen_movies: UserTopItem[];
  last_seen_episodes: UserTopItem[];
  finished_series: UserTopItem[];
};

export type UserTopItem = {
  item_id: string;
  name: string;
  type: string;
  hours: number;
};

export type UserActivity = {
  timestamp: number;
  item_id: string;
  item_name: string;
  item_type: string;
  pos_hours: number;
};
// Compatible with backend 'TopUser' and your previous UI 'TopUser' shape.

export type TopItem = {
  item_id: string;
  name: string;
  type: string;
  hours: number;
  display?: string;
};

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
  is_paused?: boolean;
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

// Lightweight Now Playing header summary
export type NowPlayingSummary = {
  outbound_mbps: number;
  active_streams: number;
  active_transcodes: number;
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

export type GenreStats = {
  genre: string;
  count: number;
};

export type MovieStats = {
  total_movies: number;
  largest_movie_gb: number;
  largest_movie_name: string;
  longest_runtime_minutes: number;
  longest_movie_name: string;
  shortest_runtime_minutes: number;
  shortest_movie_name: string;
  newest_movie: {
    name: string;
    date: string;
  };
  most_watched_movie: {
    name: string;
    hours: number;
  };
  total_runtime_hours: number;
  popular_genres: GenreStats[];
  movies_added_this_month: number;
};

export type SeriesStats = {
  total_series: number;
  total_episodes: number;
  largest_series_name: string;
  largest_series_total_gb: number;
  largest_episode_name: string;
  largest_episode_gb: number;
  longest_series_name: string;
  longest_series_runtime_minutes: number;
  most_watched_series: { name: string; hours: number };
  total_episode_runtime_hours: number;
  newest_series: { name: string; date: string };
  episodes_added_this_month: number;
  popular_genres: GenreStats[];
};
