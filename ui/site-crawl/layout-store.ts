import { useSyncExternalStore } from "react";

// View-layout state (user-dragged column widths keyed per tab, the Overview
// panel's width/visibility, the detail pane's height/visibility). Module-scope
// store + localStorage, the crumb-store pattern — NOT stored in Go: an async
// read would let the grid paint default widths first and snap a frame later.

const STORAGE_KEY = "sitecrawl.layout.v1";

export const DEFAULT_SIDE_W = 260;
export const DEFAULT_PANE_H = 260;

interface LayoutState {
  /** tabId → colId → width px. Only user-dragged overrides live here. */
  widths: Record<string, Record<string, number>>;
  sideW: number;
  sideOpen: boolean;
  paneH: number;
  /** The detail pane defaults CLOSED — the 800px window budget demands it. */
  paneOpen: boolean;
}

function load(): LayoutState {
  const def: LayoutState = {
    widths: {},
    sideW: DEFAULT_SIDE_W,
    sideOpen: true,
    paneH: DEFAULT_PANE_H,
    paneOpen: false,
  };
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<LayoutState>;
      if (parsed && typeof parsed.widths === "object" && parsed.widths !== null) {
        return { ...def, ...parsed };
      }
    }
  } catch {
    // Corrupt storage falls back to defaults.
  }
  return def;
}

let state: LayoutState = load();
const listeners = new Set<() => void>();

function emit() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // Quota errors just lose persistence, not the session.
  }
  for (const l of listeners) l();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

const EMPTY: Record<string, number> = {};

/** Width overrides for one tab (stable reference until that tab changes). */
export function useColumnWidths(tabId: string): Record<string, number> {
  return useSyncExternalStore(subscribe, () => state.widths[tabId] ?? EMPTY);
}

export function setColumnWidth(tabId: string, colId: string, width: number) {
  state = {
    ...state,
    widths: {
      ...state.widths,
      [tabId]: { ...(state.widths[tabId] ?? {}), [colId]: Math.round(width) },
    },
  };
  emit();
}

export function useLayout() {
  return useSyncExternalStore(subscribe, () => state);
}

export function setLayout(patch: Partial<Omit<LayoutState, "widths">>) {
  state = { ...state, ...patch };
  emit();
}
