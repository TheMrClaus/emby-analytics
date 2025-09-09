export interface DashboardCard {
  id: string;
  title: string;
  description: string;
  category: 'stats' | 'admin' | 'activity';
  component: React.ComponentType;
  isVisible: boolean;
  position: number;
}

export interface DashboardLayout {
  cards: DashboardCard[];
  editMode: boolean;
  lastUpdated: Date;
}

export interface CardPosition {
  cardId: string;
  position: number;
  isVisible: boolean;
}

export interface DashboardSettings {
  layout: CardPosition[];
  version: number;
}

export interface EditModeContextType {
  isEditMode: boolean;
  toggleEditMode: () => void;
  removeCard: (cardId: string) => void;
  addCard: (cardId: string) => void;
  resetLayout: () => void;
}