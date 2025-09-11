import { DashboardCard } from "../types/dashboard";
import PlaybackMethodsCard from "../components/PlaybackMethodsCard";
import MovieStatsCard from "../components/MovieStatsCard";
import SeriesStatsCard from "../components/SeriesStatsCard";
import TopUsers from "../components/TopUsers";
import TopItems from "../components/TopItems";

export const CARD_REGISTRY: Record<string, Omit<DashboardCard, "isVisible" | "position">> = {
  "playback-methods": {
    id: "playback-methods",
    title: "Playback Methods",
    description: "Distribution of playback methods used",
    category: "stats",
    component: PlaybackMethodsCard,
  },
  "movie-stats": {
    id: "movie-stats",
    title: "Movie Statistics",
    description: "Movie library and playback statistics",
    category: "stats",
    component: MovieStatsCard,
  },
  "series-stats": {
    id: "series-stats",
    title: "Series Statistics",
    description: "TV series library and playback statistics",
    category: "stats",
    component: SeriesStatsCard,
  },
  "top-users": {
    id: "top-users",
    title: "Top Users",
    description: "Most active users by playtime",
    category: "activity",
    component: TopUsers,
  },
  "top-items": {
    id: "top-items",
    title: "Top Items",
    description: "Most played content items",
    category: "activity",
    component: TopItems,
  },
};

export const DEFAULT_CARD_ORDER = [
  "playback-methods",
  "movie-stats",
  "series-stats",
  "top-users",
  "top-items",
];

export function getDefaultLayout(): DashboardCard[] {
  return DEFAULT_CARD_ORDER.map((cardId, index) => ({
    ...CARD_REGISTRY[cardId],
    isVisible: true,
    position: index,
  }));
}

export function getAllAvailableCards(): DashboardCard[] {
  return Object.values(CARD_REGISTRY).map((card, index) => ({
    ...card,
    isVisible: false,
    position: index,
  }));
}
