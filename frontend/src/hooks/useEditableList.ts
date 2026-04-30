import { useState } from 'react';

export function useEditableList<T extends Record<string, unknown>>() {
  const [edits, setEdits] = useState<Record<string, Partial<T>>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  function handleEdit(id: string, field: keyof T, value: T[keyof T]) {
    setEdits(prev => ({ ...prev, [id]: { ...prev[id], [field]: value } as Partial<T> }));
    setDirty(prev => ({ ...prev, [id]: true }));
  }

  function toggleExpand(id: string) {
    setExpanded(prev => ({ ...prev, [id]: !prev[id] }));
  }

  function resetDirty(id?: string) {
    if (id) {
      setDirty(prev => ({ ...prev, [id]: false }));
    } else {
      setDirty({});
    }
  }

  const getEdit = (id: string) => edits[id] ?? ({} as Partial<T>);
  const isDirty = (id: string) => dirty[id] ?? false;
  const isExpanded = (id: string) => expanded[id] ?? false;

  return {
    edits,
    setEdits,
    dirty,
    expanded,
    handleEdit,
    toggleExpand,
    resetDirty,
    getEdit,
    isDirty,
    isExpanded,
  };
}
