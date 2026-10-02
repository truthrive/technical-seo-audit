import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";

import * as SiteCrawlService from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/service";
import { Options, RunStatus } from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/models";
import { currentOptions, type CrawlOptions } from "@/features/site-crawl/options-store";
import { useTabId } from "@/lib/tab-context";
import { onTabDisposed } from "@/app/tabs/tabs-store";

// Wire shape of "sitecrawl:progress" (Go: sitecrawl.ProgressEvent). Aggregate
// heartbeat ≤2/s — it never carries rows; Revision is the only signal the grid
// needs to refetch its window.
export interface CrawlProgress {
  runId: string;
  phase: string;
  found: number;
  crawled: number;
  queued: number;
  inFlight: number;
  skipped: number;
  issues: { critical: number; warning: number; notice: number };
  status: { ok: number; redirect: number; clientErr: number; serverErr: number; failed: number; blocked: number };
  rate: number;
  elapsedMs: number;
  etaSec: number;
  revision: number;
  current?: string;
  // What the seed's robots.txt asked for, and what the crawl actually paces by
  // (0 when the Crawl-delay option is off). The strip says which — a crawl held
  // at one request at a time by a directive nobody set has to explain itself.
  crawlDelayAskedMs: number;
  crawlDelayAppliedMs: number;
}

// Wire shape of "sitecrawl:run-state" (Go: sitecrawl.RunStateEvent).
//
// This event — not job:progress — is the truth about whether the crawl runs.
// During a pause the job goroutine stays alive and job:progress still says
// "running"; a UI keyed on it shows a spinner over a stopped crawl (design.md).
export interface CrawlRunState {
  runId: string;
  jobId: string;
  state: string; // running | paused | completed | cancelled | stopped | failed
  phase: string;
  reason?: string;
  resumable: boolean;
  found: number;
  crawled: number;
  durationMs: number;
  error?: string;
  /** Where the seed 301'd, present when reason is "seed-redirect". */
  redirectTarget?: string;
}

// Wire shape of "sitecrawl:psi-progress" (Go: sitecrawl.PSIProgressEvent).
//
// PageSpeed keeps its own heartbeat because it outlives the crawl — Google
// takes ~20s per page, so measuring continues long after the last URL was
// fetched, and it also runs with no crawl at all when the user fills in a
// stored run. Total grows while the crawl is still finding pages.
export interface PSIProgress {
  runId: string;
  done: number;
  total: number;
  running: boolean;
  stopped: boolean;
}

export interface ActiveCrawl {
  runId: string;
  jobId: string;
  seedUrl: string;
}

// Wails v3 wraps emitted payloads in an array.
function firstEventPayload<T>(ev: { data: unknown }): T {
  return (Array.isArray(ev.data) ? ev.data[0] : ev.data) as T;
}

/** Keep in step with runs.Terminal in internal/core/runs/state.go. */
export const TERMINAL_STATES = ["completed", "cancelled", "stopped", "failed", "interrupted"];

// The crawl outlives the page: leaving the tool must not orphan a run that is
// still burning the user's bandwidth. The active run is remembered at module
// scope so remounting re-attaches to it (Status() backfills whatever events
// were missed while away).
//
// Remembered per TAB (T8): two Site Crawl tabs each follow their own crawl.
// Tab ids are unique across workspaces and survive a restart (tabs-store), so
// a crawl found again after switching client, sleeping or reloading is still
// the right one. Dropped only when its tab is closed.
const runsByTab = new Map<string, ActiveCrawl>();

onTabDisposed((tabId, reason) => {
  if (reason === "close") runsByTab.delete(tabId);
});

function rememberedRun(tabId: string): ActiveCrawl | null {
  return runsByTab.get(tabId) ?? null;
}

function remember(tabId: string, run: ActiveCrawl | null) {
  if (run) runsByTab.set(tabId, run);
  else runsByTab.delete(tabId);
}

/** Single owner of the active run's state and event subscriptions. */
export function useSiteCrawl() {
  const tabId = useTabId();
  const [run, setRun] = useState<ActiveCrawl | null>(() => rememberedRun(tabId));
  const [runState, setRunState] = useState<CrawlRunState | null>(null);
  const [progress, setProgress] = useState<CrawlProgress | null>(null);
  const [counts, setCounts] = useState<Record<string, number>>({});
  const [error, setError] = useState<string | null>(null);
  // JS rendering was requested but no Chrome/Edge exists to honour it.
  const [noBrowser, setNoBrowser] = useState(false);
  const [psi, setPsi] = useState<PSIProgress | null>(null);

  const runRef = useRef<string | null>(rememberedRun(tabId)?.runId ?? null);
  const facetsTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const facetsAt = useRef(0);
  const seenRevision = useRef(-1);

  const running = runState?.state === "running";
  const paused = runState?.state === "paused";
  const revision = progress?.revision ?? 0;

  // The Overview tree's counts come from Facets() (SQL over the whole run) —
  // the progress event deliberately carries no data, only Revision. Refresh
  // throttled to the flush cadence while crawling; immediately on a terminal
  // state, because finalize rewrites issues after the last revision we saw.
  const refreshFacets = useCallback((immediate: boolean) => {
    const id = runRef.current;
    if (!id) return;
    const fetchNow = () => {
      facetsAt.current = Date.now();
      SiteCrawlService.Facets(id)
        .then((f) => {
          if (runRef.current === id && f) setCounts(f as Record<string, number>);
        })
        .catch(() => {});
    };
    if (immediate) {
      if (facetsTimer.current) {
        clearTimeout(facetsTimer.current);
        facetsTimer.current = null;
      }
      fetchNow();
      return;
    }
    if (facetsTimer.current) return;
    const wait = Math.max(0, 2000 - (Date.now() - facetsAt.current));
    facetsTimer.current = setTimeout(() => {
      facetsTimer.current = null;
      fetchNow();
    }, wait);
  }, []);

  const adopt = useCallback((next: ActiveCrawl) => {
    remember(tabId, next);
    runRef.current = next.runId;
    setRun(next);
    setProgress(null);
    setCounts({});
    setError(null);
    setNoBrowser(false);
    setPsi(null);
    // Optimistic until the first run-state event lands.
    setRunState({
      runId: next.runId,
      jobId: next.jobId,
      state: "running",
      phase: "preparing",
      resumable: false,
      found: 0,
      crawled: 0,
      durationMs: 0,
    });
  }, [tabId]);

  // Re-attach after a remount: the run kept going while the page was away, so
  // ask Go where it stands instead of trusting stale memory.
  useEffect(() => {
    const current = runRef.current;
    if (!current) return;
    let alive = true;
    SiteCrawlService.Status(current)
      .then((st: RunStatus) => {
        if (!alive || runRef.current !== current) return;
        setRunState({
          runId: st.runId,
          jobId: st.jobId ?? current,
          state: st.state,
          phase: st.phase,
          resumable: st.state === "paused",
          found: st.found,
          crawled: st.crawled,
          durationMs: 0,
        });
        return SiteCrawlService.Facets(current);
      })
      .then((f) => {
        if (alive && f) setCounts(f as Record<string, number>);
      })
      .catch(() => {
        // Run row gone (history cleared elsewhere) — drop it.
        if (alive && runRef.current === current) {
          remember(tabId, null);
          runRef.current = null;
          setRun(null);
          setRunState(null);
        }
      });
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => {
    const offProgress = Events.On("sitecrawl:progress", (ev: { data: unknown }) => {
      const p = firstEventPayload<CrawlProgress>(ev);
      if (!runRef.current || p.runId !== runRef.current) return;
      setProgress(p);
      if (p.revision !== seenRevision.current) {
        seenRevision.current = p.revision;
        refreshFacets(false);
      }
    });
    const offState = Events.On("sitecrawl:run-state", (ev: { data: unknown }) => {
      const s = firstEventPayload<CrawlRunState>(ev);
      if (!runRef.current || s.runId !== runRef.current) return;
      setRunState(s);
      if (s.error) setError(s.error);
      if (TERMINAL_STATES.includes(s.state)) refreshFacets(true);
    });
    const offRender = Events.On("sitecrawl:render-state", (ev: { data: unknown }) => {
      const s = firstEventPayload<{ runId: string; state: string }>(ev);
      if (!runRef.current || s.runId !== runRef.current) return;
      if (s.state === "no-browser") setNoBrowser(true);
    });
    const offPsi = Events.On("sitecrawl:psi-progress", (ev: { data: unknown }) => {
      const p = firstEventPayload<PSIProgress>(ev);
      if (!runRef.current || p.runId !== runRef.current) return;
      setPsi(p);
    });
    return () => {
      offProgress();
      offState();
      offRender();
      offPsi();
      if (facetsTimer.current) {
        clearTimeout(facetsTimer.current);
        facetsTimer.current = null;
      }
    };
  }, [refreshFacets]);

  const start = useCallback(
    async (seeds: string[], override?: Partial<CrawlOptions>): Promise<string | null> => {
      setError(null);
      try {
        // The global Configuration store is the options source; an override
        // covers transient choices like list mode.
        const opts = { ...currentOptions(), ...override };
        const started = await SiteCrawlService.Start(seeds, Options.createFrom(opts));
        adopt({ runId: started.runId, jobId: started.jobId, seedUrl: seeds[0] ?? "" });
        return started.runId;
      } catch (e) {
        setError(String(e).replace(/^sitecrawl:\s*/, ""));
        return null;
      }
    },
    [adopt],
  );

  const pause = useCallback(async () => {
    if (runRef.current) await SiteCrawlService.Pause(runRef.current).catch((e) => setError(String(e)));
  }, []);

  const resume = useCallback(async () => {
    const id = runRef.current;
    if (!id) return;
    try {
      const started = await SiteCrawlService.Resume(id);
      if (run) {
        const next = { ...run, jobId: started.jobId };
        remember(tabId, next);
        setRun(next);
      }
    } catch (e) {
      setError(String(e).replace(/^sitecrawl:\s*/, ""));
    }
  }, [run, tabId]);

  const stop = useCallback(async () => {
    if (runRef.current) await SiteCrawlService.Stop(runRef.current).catch((e) => setError(String(e)));
  }, []);

  /** Ends the PageSpeed pass only; the crawl keeps going. Everything measured
   *  so far is kept, and a later Run picks up exactly where this stopped. */
  const stopPageSpeed = useCallback(async () => {
    if (runRef.current) {
      await SiteCrawlService.StopPageSpeed(runRef.current).catch((e) => setError(String(e)));
    }
  }, []);

  /** Re-open a stored run (deep link /tools/site-crawl/:runId, resume banner). */
  const open = useCallback(
    async (runId: string): Promise<boolean> => {
      try {
        const st = await SiteCrawlService.Status(runId);
        adopt({ runId: st.runId, jobId: st.jobId ?? st.runId, seedUrl: "" });
        setRunState({
          runId: st.runId,
          jobId: st.jobId ?? st.runId,
          state: st.state,
          phase: st.phase,
          resumable: st.state === "paused",
          found: st.found,
          crawled: st.crawled,
          durationMs: 0,
        });
        const f = await SiteCrawlService.Facets(runId);
        if (f) setCounts(f as Record<string, number>);
        return true;
      } catch (e) {
        setError(String(e).replace(/^sitecrawl:\s*/, ""));
        return false;
      }
    },
    [adopt],
  );

  /** Drops the run from the screen after its rows were deleted. */
  const discard = useCallback(() => {
    remember(tabId, null);
    runRef.current = null;
    setRun(null);
    setRunState(null);
    setProgress(null);
    setCounts({});
    setError(null);
    setPsi(null);
  }, [tabId]);

  return {
    run,
    runState,
    progress,
    counts,
    revision,
    running,
    paused,
    error,
    noBrowser,
    psi,
    start,
    pause,
    resume,
    stop,
    stopPageSpeed,
    open,
    discard,
  };
}

export type UseSiteCrawl = ReturnType<typeof useSiteCrawl>;
