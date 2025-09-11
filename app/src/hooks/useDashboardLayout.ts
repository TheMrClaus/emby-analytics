import { useState, useEffect, useCallback } from "react";
import { DashboardCard, DashboardSettings, EditModeContextType } from "../types/dashboard";
import { getDefaultLayout, CARD_REGISTRY } from "../lib/cardRegistry";
import { useSettings } from "./useSettings";

const DASHBOARD_LAYOUT_KEY = "dashboard_layout";
const DASHBOARD_VERSION = 1;

function getStoredLayout(): DashboardSettings | null {
  try {
    if (typeof window !== "undefined") {
      const stored = localStorage.getItem(DASHBOARD_LAYOUT_KEY);
      if (stored) {
        const parsed = JSON.parse(stored) as DashboardSettings;
        if (parsed.version === DASHBOARD_VERSION) {
          return parsed;
        }
      }
    }
  } catch {
    /* ignore localStorage errors */
  }
  return null;
}

function storeLayout(settings: DashboardSettings) {
  try {
    if (typeof window !== "undefined") {
      localStorage.setItem(DASHBOARD_LAYOUT_KEY, JSON.stringify(settings));
    }
  } catch {
    /* ignore localStorage errors */
  }
}

export function useDashboardLayout() {
  const [isEditMode, setIsEditMode] = useState(false);
  const [cards, setCards] = useState<DashboardCard[]>(() => getDefaultLayout());
  const { updateSetting } = useSettings();

  // Initialize layout from storage on mount
  useEffect(() => {
    const stored = getStoredLayout();
    if (stored) {
      // Convert stored positions back to full card objects
      const restoredCards = stored.layout
        .map((pos) => {
          const cardDef = CARD_REGISTRY[pos.cardId];
          if (!cardDef) return null;
          return {
            ...cardDef,
            isVisible: pos.isVisible,
            position: pos.position,
          };
        })
        .filter((card): card is DashboardCard => card !== null)
        .sort((a, b) => a.position - b.position);

      if (restoredCards.length > 0) {
        setCards(restoredCards);
      }
    }
  }, []);

  const persistLayout = useCallback(
    async (newCards: DashboardCard[]) => {
      const settings: DashboardSettings = {
        layout: newCards.map((card) => ({
          cardId: card.id,
          position: card.position,
          isVisible: card.isVisible,
        })),
        version: DASHBOARD_VERSION,
      };

      // Store locally first for immediate persistence
      storeLayout(settings);

      // Try to persist to backend (optional)
      try {
        await updateSetting(DASHBOARD_LAYOUT_KEY, JSON.stringify(settings));
      } catch (error) {
        console.warn("Failed to persist dashboard layout to backend:", error);
        // Continue with local storage only - this is not a fatal error
      }
    },
    [updateSetting]
  );

  const updateCardPositions = useCallback(
    (newCards: DashboardCard[]) => {
      setCards(newCards);
      persistLayout(newCards);
    },
    [persistLayout]
  );

  const toggleEditMode = useCallback(() => {
    setIsEditMode((prev) => !prev);
  }, []);

  const reorderCards = useCallback(
    (startIndex: number, endIndex: number) => {
      setCards((current) => {
        const newCards = [...current];
        const [reorderedCard] = newCards.splice(startIndex, 1);
        newCards.splice(endIndex, 0, reorderedCard);

        // Update positions
        const updatedCards = newCards.map((card, index) => ({
          ...card,
          position: index,
        }));

        persistLayout(updatedCards);
        return updatedCards;
      });
    },
    [persistLayout]
  );

  const removeCard = useCallback(
    (cardId: string) => {
      setCards((current) => {
        const updated = current.map((card) =>
          card.id === cardId ? { ...card, isVisible: false } : card
        );
        persistLayout(updated);
        return updated;
      });
    },
    [persistLayout]
  );

  const addCard = useCallback(
    (cardId: string) => {
      setCards((current) => {
        const cardExists = current.find((card) => card.id === cardId);

        if (cardExists) {
          // Make existing card visible
          const updated = current.map((card) =>
            card.id === cardId ? { ...card, isVisible: true } : card
          );
          persistLayout(updated);
          return updated;
        } else {
          // Add new card from registry
          const cardDef = CARD_REGISTRY[cardId];
          if (!cardDef) return current;

          const newCard: DashboardCard = {
            ...cardDef,
            isVisible: true,
            position: current.length,
          };

          const updated = [...current, newCard];
          persistLayout(updated);
          return updated;
        }
      });
    },
    [persistLayout]
  );

  const resetLayout = useCallback(() => {
    const defaultCards = getDefaultLayout();
    setCards(defaultCards);
    persistLayout(defaultCards);
    setIsEditMode(false);
  }, [persistLayout]);

  // Get visible cards sorted by position
  const visibleCards = cards
    .filter((card) => card.isVisible)
    .sort((a, b) => a.position - b.position);

  // Get available cards that can be added
  const availableCards = Object.values(CARD_REGISTRY).filter(
    (cardDef) => !cards.find((card) => card.id === cardDef.id && card.isVisible)
  );

  const editModeContext: EditModeContextType = {
    isEditMode,
    toggleEditMode,
    removeCard,
    addCard,
    resetLayout,
  };

  return {
    cards: visibleCards,
    availableCards,
    isEditMode,
    editModeContext,
    reorderCards,
    updateCardPositions,
  };
}
