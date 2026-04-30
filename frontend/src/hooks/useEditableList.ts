import { useState, useCallback } from 'react';

export function useEditableList<T extends Record<string, unknown>>(
  _items: T[] | undefined,
  _idKey: keyof T = 'id' as keyof T,
) {
  const [edits, setEdits] = useState<Record<string, Partial<T>>>({});
  const [dirty, setDirty] = useState<Record<string, boolean>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  const handleEdit = useCallback(
    (id: string, field: keyof T, value: T[keyof T]) => {
      setEdits(prev => ({ ...prev, [id]: { ...prev[id], [field]: value } as Partial<T> }));
      setDirty(prev => ({ ...prev, [id]: true }));
    },
    [],
  );

  const toggleExpand = useCallback((id: string) => {
    setExpanded(prev => ({ ...prev, [id]: !prev[id] }));
  }, []);

  const resetDirty = useCallback((id?: string) => {
    if (id) {
      setDirty(prev => ({ ...prev, [id]: false }));
    } else {
      setDirty({});
    }
  }, []);

  const getEdit = useCallback(
    (id: string) => edits[id] ?? ({} as Partial<T>),
    [edits],
  );

  const isDirty = useCallback((id: string) => dirty[id] ?? false, [dirty]);

  const isExpanded = useCallback((id: string) => expanded[id] ?? false, [expanded]);

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
