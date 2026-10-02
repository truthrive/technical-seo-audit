import { useCallback, useEffect, useReducer, useRef } from "react";

import * as SiteCrawlService from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/service";
import { RowQuery } from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/models";
import type { GridRowData, GridSort } from "@/components/ui/data-grid";

// Windowed row cache for the crawl grid.
//
// The dataset NEVER materializes into a React array (design.md §6 data grid):
// rows live in a Map<chunkIndex, GridRowData[]> inside a ref, the grid reads
// through getRow(i), and the only React state is a version counter bumped
// through a 120ms coalescing window. Cost per flush is O(rows on screen),
// independent of whether the crawl found 500 or 500,000 URLs.

const CHUNK = 200;
const FLUSH_MS = 120;
/** Live refetch of the visible window trails the revision signal by this. */
const REFRESH_MS = 1200;
/** Per-view cap: 50 chunks = 10k rows ≈ 6-10MB of short strings. */
const MAX_CHUNKS = 50;
/** Views kept warm (tab/filter/sort/search combinations). */
const MAX_VIEWS = 3;

interface ViewCache {
  chunks: Map<number, GridRowData[]>;
  /** chunkIndex → touch tick, for least-recently-seen eviction. */
  touched: Map<number, number>;
  total: number; // -1 = not known yet
}

export interface CrawlGridArgs {
  runId: string | null;
  tab: string;
  filter: string;
  search: string;
  sort: GridSort | null;
  cols: string[];
  /** Bumped by Go on every committed flush (sitecrawl:progress). */
  revision: number;
  /** Crawl still writing — visible windows refresh on a trailing timer. */
  live: boolean;
}

export function useCrawlGrid(args: CrawlGridArgs) {
  const { runId, tab, filter, search, sort, cols, revision, live } = args;

  const viewKey = JSON.stringify([runId, tab, filter, search, sort?.id ?? "", sort?.desc ?? false, cols]);

  const [version, bump] = useReducer((n: number) => n + 1, 0);
  const viewsRef = useRef(new Map<string, ViewCache>());
  const inflightRef = useRef(new Set<string>());
  const flushRef = useRef<number | null>(null);
  const refreshRef = useRef<number | null>(null);
  const rangeRef = useRef<[number, number]>([0, 0]);
  const tickRef = useRef(0);
  // Bumped whenever the cache is invalidated wholesale, so responses issued
  // before that can be told apart from ones issued after.
  const genRef = useRef(0);
  // Latest values for callbacks that must not re-subscribe per keystroke.
  const argsRef = useRef({ viewKey, runId, tab, filter, search, sort, cols });
  argsRef.current = { viewKey, runId, tab, filter, search, sort, cols };

  const scheduleBump = useCallback(() => {
    if (flushRef.current !== null) return;
    flushRef.current = window.setTimeout(() => {
      flushRef.current = null;
      bump();
    }, FLUSH_MS);
  }, []);

  /** The current view's cache, created on first touch and kept LRU-fresh. */
  const view = useCallback((): ViewCache => {
    const views = viewsRef.current;
    const key = argsRef.current.viewKey;
    let v = views.get(key);
    if (v) {
      // delete+set refreshes recency (the map's insertion order is the LRU).
      views.delete(key);
      views.set(key, v);
      return v;
    }
    v = { chunks: new Map(), touched: new Map(), total: -1 };
    views.set(key, v);
    while (views.size > MAX_VIEWS) {
      const oldest = views.keys().next().value;
      if (oldest === undefined) break;
      views.delete(oldest);
    }
    return v;
  }, []);

  const fetchChunk = useCallback(
    (chunk: number, wantTotal: boolean) => {
      const a = argsRef.current;
      if (!a.runId) return;
      const key = `${a.viewKey}#${chunk}`;
      if (inflightRef.current.has(key)) return;
      inflightRef.current.add(key);
      const gen = genRef.current;

      SiteCrawlService.Rows(
        RowQuery.createFrom({
          runId: a.runId,
          tab: a.tab,
          filter: a.filter === "all" ? "" : a.filter,
          search: a.search,
          cols: a.cols,
          sort: a.sort?.id ?? "",
          desc: a.sort?.desc ?? false,
          offset: chunk * CHUNK,
          limit: CHUNK,
          wantTotal,
        }),
      )
        .then((page) => {
          // A response for a view the user already left is dropped silently —
          // Wails calls cannot abort, and 200 rows cost nothing to discard.
          if (argsRef.current.viewKey !== a.viewKey) return;
          // Same for a response issued before the cache was invalidated. The
          // viewKey does not change when a run finishes — only `live` does — so
          // without this a request sent before finalize rewrote inlinks, issues
          // and orphans lands after the clear, rebuilds the cache it was meant
          // to replace, and pins pre-finalize rows on screen for good.
          if (gen !== genRef.current) return;
          const v = view();
          v.chunks.set(chunk, (page.rows ?? []) as GridRowData[]);
          v.touched.set(chunk, ++tickRef.current);
          if (page.total >= 0) v.total = page.total;
          evict(v);
          scheduleBump();
        })
        .catch(() => {
          // A failed window fetch stays skeleton; the next scroll retries it.
        })
        .finally(() => {
          inflightRef.current.delete(key);
        });
    },
    [view, scheduleBump],
  );

  /** Drop least-recently-seen chunks outside the visible window ±5. */
  const evict = (v: ViewCache) => {
    if (v.chunks.size <= MAX_CHUNKS) return;
    const [start, end] = rangeRef.current;
    const lo = Math.floor(start / CHUNK) - 5;
    const hi = Math.floor(end / CHUNK) + 5;
    const candidates = [...v.touched.entries()]
      .filter(([c]) => c < lo || c > hi)
      .sort((a, b) => a[1] - b[1]);
    for (const [c] of candidates) {
      if (v.chunks.size <= MAX_CHUNKS) break;
      v.chunks.delete(c);
      v.touched.delete(c);
    }
  };

  /** Fetch driver, called by the grid with the visible (inclusive) window. */
  const onRange = useCallback(
    (start: number, end: number) => {
      const prevStart = rangeRef.current[0];
      rangeRef.current = [start, end];
      const v = view();
      const firstChunk = Math.floor(start / CHUNK);
      const lastChunk = Math.floor(end / CHUNK);
      for (let c = firstChunk; c <= lastChunk; c++) {
        if (!v.chunks.has(c)) fetchChunk(c, v.total < 0);
        else v.touched.set(c, ++tickRef.current);
      }
      // One chunk of read-ahead in the scroll direction.
      const ahead = start >= prevStart ? lastChunk + 1 : firstChunk - 1;
      if (ahead >= 0 && (v.total < 0 || ahead * CHUNK < v.total) && !v.chunks.has(ahead)) {
        fetchChunk(ahead, false);
      }
    },
    [view, fetchChunk],
  );

  // New view (tab/filter/sort/search changed): make sure chunk 0 and the total
  // exist. A warm LRU hit renders instantly; a cold one shows skeletons.
  useEffect(() => {
    if (!runId) return;
    const v = view();
    if (!v.chunks.has(0) || v.total < 0) fetchChunk(0, true);
    bump();
    // viewKey covers every input that changes the SQL window.
  }, [viewKey, runId, view, fetchChunk]);

  // Live crawl: a committed flush bumps revision; refresh the visible window
  // (and the count) on a trailing timer instead of on every event.
  useEffect(() => {
    if (!runId || revision === 0) return;
    if (refreshRef.current !== null) return;
    refreshRef.current = window.setTimeout(() => {
      refreshRef.current = null;
      const v = view();
      const [start, end] = rangeRef.current;
      const firstChunk = Math.floor(start / CHUNK);
      const lastChunk = Math.floor(end / CHUNK);
      for (let c = firstChunk; c <= lastChunk; c++) fetchChunk(c, c === firstChunk);
      // The tail chunk feeds tail-follow even when it is not on screen yet.
      if (v.total > 0) {
        const tail = Math.floor((v.total - 1) / CHUNK);
        if (tail > lastChunk) fetchChunk(tail, false);
      }
    }, REFRESH_MS);
  }, [revision, runId, view, fetchChunk]);

  // The run just finished: finalize rewrote inlinks/issues/orphans, so every
  // cached window of this run is stale. Drop them all and refetch what shows.
  const prevLive = useRef(live);
  useEffect(() => {
    if (prevLive.current && !live && runId) {
      // Bump first: responses already in flight belong to the old generation and
      // must not write into the cache being rebuilt here.
      genRef.current++;
      viewsRef.current.clear();
      inflightRef.current.clear();
      const [start, end] = rangeRef.current;
      const firstChunk = Math.floor(start / CHUNK);
      const lastChunk = Math.floor(end / CHUNK);
      for (let c = firstChunk; c <= lastChunk; c++) fetchChunk(c, c === firstChunk);
    }
    prevLive.current = live;
  }, [live, runId, fetchChunk]);

  useEffect(
    () => () => {
      if (flushRef.current !== null) window.clearTimeout(flushRef.current);
      if (refreshRef.current !== null) window.clearTimeout(refreshRef.current);
    },
    [],
  );

  const getRow = useCallback(
    (index: number): GridRowData | undefined => {
      const v = viewsRef.current.get(argsRef.current.viewKey);
      return v?.chunks.get(Math.floor(index / CHUNK))?.[index % CHUNK];
    },
    // version makes the grid re-read after every coalesced flush.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [version],
  );

  const current = viewsRef.current.get(viewKey);
  const total = current && current.total >= 0 ? current.total : 0;
  const knownTotal = current !== undefined && current.total >= 0;

  return { getRow, total, knownTotal, onRange, version };
}
