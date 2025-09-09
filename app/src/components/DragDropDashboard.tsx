'use client';

import { DragDropContext, Droppable, Draggable, DropResult } from '@hello-pangea/dnd';
import { useDashboardLayout } from '../hooks/useDashboardLayout';
import { DashboardCard } from '../types/dashboard';
import { ErrorBoundary } from './ErrorBoundary';
import { X, Plus, RotateCcw, GripVertical } from 'lucide-react';
import { useState } from 'react';

interface DraggableCardWrapperProps {
  card: DashboardCard;
  index: number;
  isEditMode: boolean;
  onRemove: (cardId: string) => void;
}

function DraggableCardWrapper({ card, index, isEditMode, onRemove }: DraggableCardWrapperProps) {
  const CardComponent = card.component;

  return (
    <Draggable draggableId={card.id} index={index} isDragDisabled={!isEditMode}>
      {(provided, snapshot) => (
        <div
          ref={provided.innerRef}
          {...provided.draggableProps}
          className={`${
            isEditMode ? '' : 'break-inside-avoid mb-4'
          } transition-all duration-200 ${
            snapshot.isDragging ? 'opacity-75 scale-105 z-50' : ''
          } ${isEditMode ? 'ring-2 ring-blue-500/30 ring-offset-2 ring-offset-neutral-900' : ''}`}
        >
          <div className="relative group">
            {isEditMode && (
              <>
                {/* Drag Handle */}
                <div
                  {...provided.dragHandleProps}
                  className="absolute -top-2 -left-2 z-10 bg-blue-500 text-white p-1 rounded-full opacity-0 group-hover:opacity-100 transition-opacity duration-200 cursor-grab active:cursor-grabbing"
                >
                  <GripVertical className="w-4 h-4" />
                </div>
                
                {/* Remove Button */}
                <button
                  onClick={() => onRemove(card.id)}
                  className="absolute -top-2 -right-2 z-10 bg-red-500 hover:bg-red-600 text-white p-1 rounded-full opacity-0 group-hover:opacity-100 transition-opacity duration-200"
                >
                  <X className="w-4 h-4" />
                </button>
              </>
            )}
            
            <ErrorBoundary>
              <CardComponent />
            </ErrorBoundary>
          </div>
        </div>
      )}
    </Draggable>
  );
}

interface AddCardDropdownProps {
  availableCards: Array<Omit<DashboardCard, 'isVisible' | 'position'>>;
  onAddCard: (cardId: string) => void;
}

function AddCardDropdown({ availableCards, onAddCard }: AddCardDropdownProps) {
  const [isOpen, setIsOpen] = useState(false);

  if (availableCards.length === 0) {
    return null;
  }

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
      >
        <Plus className="w-4 h-4" />
        Add Card
      </button>

      {isOpen && (
        <div className="absolute top-full mt-2 left-0 bg-neutral-800 border border-neutral-700 rounded-lg shadow-xl z-20 min-w-64">
          <div className="p-2">
            <div className="text-sm text-neutral-400 px-3 py-2 border-b border-neutral-700">
              Available Cards
            </div>
            {availableCards.map((card) => (
              <button
                key={card.id}
                onClick={() => {
                  onAddCard(card.id);
                  setIsOpen(false);
                }}
                className="w-full text-left px-3 py-2 hover:bg-neutral-700 rounded-lg transition-colors"
              >
                <div className="text-sm font-medium text-white">{card.title}</div>
                <div className="text-xs text-neutral-400">{card.description}</div>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

interface EditControlsProps {
  isEditMode: boolean;
  onToggleEdit: () => void;
  onReset: () => void;
  availableCards: Array<Omit<DashboardCard, 'isVisible' | 'position'>>;
  onAddCard: (cardId: string) => void;
}

function EditControls({ 
  isEditMode, 
  onToggleEdit, 
  onReset, 
  availableCards, 
  onAddCard 
}: EditControlsProps) {
  return (
    <div className="flex items-center gap-3 mb-6">
      <button
        onClick={onToggleEdit}
        className={`px-4 py-2 rounded-lg font-medium transition-colors ${
          isEditMode
            ? 'bg-green-600 hover:bg-green-700 text-white'
            : 'bg-neutral-700 hover:bg-neutral-600 text-neutral-200'
        }`}
      >
        {isEditMode ? 'Done Editing' : 'Edit Dashboard'}
      </button>

      {isEditMode && (
        <>
          <AddCardDropdown availableCards={availableCards} onAddCard={onAddCard} />
          
          <button
            onClick={onReset}
            className="flex items-center gap-2 px-4 py-2 bg-neutral-700 hover:bg-neutral-600 text-neutral-200 rounded-lg transition-colors"
          >
            <RotateCcw className="w-4 h-4" />
            Reset
          </button>
        </>
      )}
    </div>
  );
}

export default function DragDropDashboard() {
  const { 
    cards, 
    availableCards, 
    isEditMode, 
    editModeContext, 
    reorderCards 
  } = useDashboardLayout();

  const handleDragEnd = (result: DropResult) => {
    if (!result.destination) return;
    
    if (result.source.index !== result.destination.index) {
      reorderCards(result.source.index, result.destination.index);
    }
  };

  return (
    <div>
      <EditControls
        isEditMode={isEditMode}
        onToggleEdit={editModeContext.toggleEditMode}
        onReset={editModeContext.resetLayout}
        availableCards={availableCards}
        onAddCard={editModeContext.addCard}
      />

      <DragDropContext onDragEnd={handleDragEnd}>
        <Droppable droppableId="dashboard-cards">
          {(provided, snapshot) => (
            <div
              ref={provided.innerRef}
              {...provided.droppableProps}
              className={`${
                isEditMode 
                  ? 'grid gap-6 lg:grid-cols-2' 
                  : 'columns-1 lg:columns-2 gap-4'
              } transition-all duration-200 ${
                snapshot.isDraggingOver ? 'bg-neutral-800/30 rounded-lg p-4' : ''
              }`}
              style={!isEditMode ? { columnGap: '1rem' } : undefined}
            >
              {cards.map((card, index) => (
                <DraggableCardWrapper
                  key={card.id}
                  card={card}
                  index={index}
                  isEditMode={isEditMode}
                  onRemove={editModeContext.removeCard}
                />
              ))}
              {provided.placeholder}
              
              {cards.length === 0 && (
                <div className="lg:col-span-2 text-center py-12">
                  <div className="text-neutral-400 mb-4">
                    No cards visible. Add some cards to get started!
                  </div>
                  {availableCards.length > 0 && (
                    <AddCardDropdown
                      availableCards={availableCards}
                      onAddCard={editModeContext.addCard}
                    />
                  )}
                </div>
              )}
            </div>
          )}
        </Droppable>
      </DragDropContext>
    </div>
  );
}