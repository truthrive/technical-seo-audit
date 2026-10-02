import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { CornerUpRight, History, Play, TriangleAlert, Waypoints, X } from "lucide-react";

import * as SiteCrawlService from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/service";
import { IssueInfo, RowQuery } from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/models";
import { useSetDetailCrumb } from "@/app/crumb-store";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { ErrorText } from "@/components/error-text";
import { Button } from "@/components/ui/button";
import { DataGrid, type GridColumn, type GridSort } from "@/components/ui/data-grid";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Spinner } from "@/components/ui/spinner";
import { makeRenderCell } from "@/features/site-crawl/cells";
import { DEFAULT_TAB, issueTarget, tabDef } from "@/features/site-crawl/columns";
import { CrawlBar } from "@/features/site-crawl/crawl-bar";
import { DetailPane, type PaneSelection } from "@/features/site-crawl/detail-pane";
import { GridToolbar } from "@/features/site-crawl/grid-toolbar";
import {
  setColumnWidth,
  setLayout,
  useColumnWidths,
  useLayout,
} from "@/features/site-crawl/layout-store";
import { OpportunitiesPanel } from "@/features/site-crawl/opportunities-panel";
import { OverviewPanel } from "@/features/site-crawl/overview-panel";
import { PageSpeedPanel } from "@/features/site-crawl/pagespeed-panel";
import { Splitter } from "@/features/site-crawl/splitter";
import { VisualizationPanel } from "@/features/site-crawl/visualization-panel";
import { StatusStrip } from "@/features/site-crawl/status-strip";
import { TabStrip } from "@/features/site-crawl/tab-strip";
import { useCrawlGrid } from "@/features/site-crawl/use-crawl-grid";
import { useSiteCrawl } from "@/features/site-crawl/use-site-crawl";

const SEARCH_DEBOUNCE_MS = 300;

export function SiteCrawlPage() {
  const { t } = useTranslation();
  const { runId: runIdParam } = useParams<{ runId: string }>();
  const sc = useSiteCrawl();
  const pageRef = useRef<HTMLDivElement>(null);

  const [tab, setTab] = useState(DEFAULT_TAB);
  const [filter, setFilter] = useState("all");
  const [sort, setSort] = useState<GridSort | null>(null);
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<number | null>(null);
  // Multi-select, keyed by URL so it survives sorts and filter hops. Lives for
  // the run: a new crawl (adopt) resets it via the runId effect below.
  const [checkedIds, setCheckedIds] = useState<ReadonlySet<string>>(new Set());
  const [confirmClear, setConfirmClear] = useState(false);
  const [resumable, setResumable] = useState<{ id: string; seedUrl: string } | null>(null);
  const [issueCatalog, setIssueCatalog] = useState<IssueInfo[]>([]);

  const layout = useLayout();

  // Deep link /tools/site-crawl/:runId opens a stored run.
  useEffect(() => {
    if (runIdParam && sc.run?.runId !== runIdParam) void sc.open(runIdParam);
    // sc.open is stable; re-running on every sc identity change would refetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runIdParam]);
  useSetDetailCrumb(runIdParam ? (sc.run?.seedUrl || runIdParam) : null);

  // A crawl interrupted by closing the app was checkpointed and marked
  // resumable at startup — surface it instead of hoping the user remembers.
  useEffect(() => {
    if (runIdParam || sc.run) return;
    let alive = true;
    SiteCrawlService.Runs()
      .then((runs) => {
        if (!alive || !runs) return;
        const r = runs.find((x) => x.state === "paused" && x.resumable);
        if (r) setResumable({ id: r.id, seedUrl: r.seedUrl });
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
    // Once, on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Rule labels/severities for the Overview tree, one call per page life.
  useEffect(() => {
    SiteCrawlService.IssueCatalog()
      .then((list) => setIssueCatalog(list ?? []))
      .catch(() => {});
  }, []);

  // Search debounce: typing hits SQL only after a pause.
  useEffect(() => {
    const id = window.setTimeout(() => setSearch(searchInput), SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(id);
  }, [searchInput]);

  // Panel/pane sizes live in CSS vars written by a layout effect keyed on the
  // committed values — never in the style prop, where any re-render (progress
  // ticks at 2/s) would stomp a drag in progress.
  useLayoutEffect(() => {
    pageRef.current?.style.setProperty("--sc-side-w", `${layout.sideW}px`);
    pageRef.current?.style.setProperty("--sc-pane-h", `${layout.paneH}px`);
  }, [layout.sideW, layout.paneH]);

  // The Overview panel folds itself away when the body drops under 1000px
  // (1024 window + open sidebar leaves ~714px) — the user can still reopen it.
  const wasNarrow = useRef(false);
  useEffect(() => {
    const el = pageRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => {
      // Width 0 is a hidden tab (display:none), not a narrow window.
      if (el.clientWidth === 0) return;
      const narrow = el.clientWidth < 1000;
      if (narrow && !wasNarrow.current) setLayout({ sideOpen: false });
      wasNarrow.current = narrow;
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const def = tabDef(tab);
  const widthOverrides = useColumnWidths(tab);
  const columns = useMemo<GridColumn[]>(
    () =>
      def.columns.map((c) => ({
        id: c.id,
        width: widthOverrides[c.id] ?? c.width,
        minWidth: c.minWidth,
        grow: c.grow,
        align: c.align,
        sortable: c.sortable,
      })),
    [def, widthOverrides],
  );
  const colIds = useMemo(() => columns.map((c) => c.id), [columns]);

  const grid = useCrawlGrid({
    runId: sc.run?.runId ?? null,
    tab,
    filter,
    search,
    sort,
    cols: colIds,
    revision: sc.revision,
    live: sc.running,
  });

  /** One entry point for every navigation source: tab strip, Overview tree,
   *  Issues rows, the pane's "Go to affected URLs". */
  const navigate = useCallback((nextTab: string, nextFilter: string) => {
    setTab(nextTab);
    setFilter(nextFilter);
    setSort(null);
    setSelected(null);
  }, []);

  const switchTab = useCallback((next: string) => navigate(next, "all"), [navigate]);

  // unsorted → asc → desc → unsorted (design.md sort cycle).
  const cycleSort = useCallback((colId: string) => {
    setSort((cur) => {
      if (!cur || cur.id !== colId) return { id: colId, desc: false };
      if (!cur.desc) return { id: colId, desc: true };
      return null;
    });
  }, []);

  // Selection is positional; a new window invalidates it.
  const viewKey = `${sc.run?.runId}|${tab}|${filter}|${search}|${sort?.id}|${sort?.desc}`;
  const prevViewKey = useRef(viewKey);
  useEffect(() => {
    if (prevViewKey.current !== viewKey) {
      prevViewKey.current = viewKey;
      setSelected(null);
    }
  }, [viewKey]);

  // The checkbox set is keyed by URL, so it only resets when the run changes.
  const runId = sc.run?.runId ?? null;
  const prevRunId = useRef(runId);
  useEffect(() => {
    if (prevRunId.current !== runId) {
      prevRunId.current = runId;
      setCheckedIds(new Set());
    }
  }, [runId]);

  const toggleChecked = useCallback((id: string, next: boolean) => {
    setCheckedIds((cur) => {
      const out = new Set(cur);
      if (next) out.add(id);
      else out.delete(id);
      return out;
    });
  }, []);

  // Checkboxes only where a row IS a URL — a link edge or an issue rule has
  // nothing to export by URL.
  const checkable = def.rowKind === "page" || def.rowKind === "image";

  const renderCell = useMemo(() => makeRenderCell(t), [t]);

  const selection = useMemo<PaneSelection | null>(() => {
    if (selected === null) return null;
    const row = grid.getRow(selected);
    if (!row) return null;
    return { rowKind: def.rowKind, row };
  }, [selected, grid, def.rowKind]);

  // Activating a row: Issues rows are a navigation surface — they jump to the
  // affected URLs; everywhere else "open" means the detail pane.
  const activateRow = useCallback(
    (index: number) => {
      const row = grid.getRow(index);
      if (!row) return;
      if (def.rowKind === "issue") {
        const target = issueTarget(row.id);
        navigate(target.tab, target.filter);
        return;
      }
      setSelected(index);
      setLayout({ paneOpen: true });
    },
    [grid, def.rowKind, navigate],
  );

  const buildQuery = useCallback(
    () =>
      RowQuery.createFrom({
        runId: sc.run?.runId ?? "",
        tab,
        filter: filter === "all" ? "" : filter,
        search,
        cols: colIds,
        sort: sort?.id ?? "",
        desc: sort?.desc ?? false,
        offset: 0,
        limit: 0,
        wantTotal: false,
        // A non-empty selection narrows every export to exactly those rows.
        ids: checkable ? Array.from(checkedIds) : [],
      }),
    [sc.run, tab, filter, search, colIds, sort, checkable, checkedIds],
  );

  const exportView = useCallback(
    (format: string) => SiteCrawlService.Export(format, buildQuery()),
    [buildQuery],
  );

  const clearRun = useCallback(async () => {
    const id = sc.run?.runId;
    if (!id) return;
    await SiteCrawlService.DeleteRun(id).catch(() => {});
    sc.discard();
    setSelected(null);
  }, [sc]);

  const followTail = sc.running && sort === null && filter === "all" && search === "";

  // The crawl ended at one page because the site lives on another domain.
  // Screaming Frog's most-asked question — answer it with a button (§0-H).
  const redirectTarget =
    sc.runState?.state === "completed" && sc.runState.reason === "seed-redirect"
      ? sc.runState.redirectTarget
      : undefined;
  const redirectHost = (() => {
    if (!redirectTarget) return "";
    try {
      return new URL(redirectTarget).hostname;
    } catch {
      return redirectTarget;
    }
  })();

  // The crawl reached nothing because the address resolved to this machine.
  // Almost always a DNS server filtering the domain, which reads as "the app is
  // broken" unless the app says otherwise — and it names the fix (§0-H).
  const dnsBlocked =
    sc.runState?.state === "completed" && sc.runState.reason === "dns-blocked";
  const seedHost = (() => {
    if (!sc.run?.seedUrl) return "";
    try {
      return new URL(sc.run.seedUrl).hostname;
    } catch {
      return sc.run.seedUrl;
    }
  })();

  const empty = !sc.run ? (
    <Empty>
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <Waypoints />
        </EmptyMedia>
        <EmptyTitle>{t("siteCrawl.emptyTitle")}</EmptyTitle>
        <EmptyDescription>{t("siteCrawl.emptyBody")}</EmptyDescription>
      </EmptyHeader>
    </Empty>
  ) : !grid.knownTotal ? (
    <Spinner className="size-5 text-muted-foreground" />
  ) : (
    <Empty>
      <EmptyHeader>
        <EmptyTitle>{t("siteCrawl.noRowsTitle")}</EmptyTitle>
        <EmptyDescription>
          {search.trim() !== "" || filter !== "all"
            ? t("siteCrawl.noRowsFiltered")
            : t("siteCrawl.noRowsBody")}
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  );

  return (
    <div ref={pageRef} className="flex h-full min-h-0 min-w-0 flex-col gap-3">
      <CrawlBar sc={sc} onClear={() => setConfirmClear(true)} />

      {/* Own line, not inside the bar: the bar must never reflow the moment
          an error appears (§0-E). */}
      <ErrorText error={sc.error} className="shrink-0 text-xs" />

      {resumable && !sc.run && (
        <div className="flex shrink-0 items-center gap-2 rounded-xl bg-card px-4 py-2 text-sm shadow-soft">
          <History className="size-4 shrink-0 text-muted-foreground" />
          <span className="min-w-0 truncate">
            {t("siteCrawl.resumableBanner", { url: resumable.seedUrl })}
          </span>
          <Button
            size="sm"
            className="ml-auto"
            onClick={() => {
              const target = resumable;
              setResumable(null);
              void sc.open(target.id).then((ok) => {
                if (ok) void sc.resume();
              });
            }}
          >
            <Play data-icon="inline-start" />
            {t("siteCrawl.resume")}
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t("common.cancel")}
            onClick={() => setResumable(null)}
          >
            <X />
          </Button>
        </div>
      )}

      <StatusStrip sc={sc} />

      {sc.noBrowser && (
        <div className="flex shrink-0 items-center gap-2 rounded-xl bg-warning/10 px-4 py-2 text-sm">
          <TriangleAlert className="size-4 shrink-0 text-warning" />
          <span className="min-w-0 truncate">{t("siteCrawl.noBrowserBanner")}</span>
        </div>
      )}

      {dnsBlocked && (
        <div className="flex shrink-0 items-start gap-2 rounded-xl bg-warning/10 px-4 py-2 text-sm">
          <TriangleAlert className="mt-0.5 size-4 shrink-0 text-warning" />
          <span className="min-w-0">{t("siteCrawl.dnsBlockedBanner", { host: seedHost })}</span>
        </div>
      )}

      {redirectTarget && (
        <div className="flex shrink-0 items-center gap-2 rounded-xl bg-warning/10 px-4 py-2 text-sm">
          <CornerUpRight className="size-4 shrink-0 text-warning" />
          <span className="min-w-0 truncate">
            {t("siteCrawl.seedRedirectBanner", { target: redirectTarget })}
          </span>
          <Button
            size="sm"
            className="ml-auto shrink-0"
            onClick={() => void sc.start([redirectTarget])}
          >
            <Play data-icon="inline-start" />
            {t("siteCrawl.crawlTarget", { host: redirectHost })}
          </Button>
        </div>
      )}

      <TabStrip active={tab} onSelect={switchTab} />

      <div className="flex min-h-0 min-w-0 flex-1 gap-1.5">
        {/* PageSpeed and Visualization replace the whole grid area — they are
            reports, not row windows. Every grid tab keeps the ONE live grid. */}
        {tab === "pagespeed" && sc.run ? (
          <PageSpeedPanel runId={sc.run.runId} selectedUrls={Array.from(checkedIds)} />
        ) : tab === "opportunities" && sc.run ? (
          <OpportunitiesPanel runId={sc.run.runId} />
        ) : tab === "visualization" && sc.run ? (
          <VisualizationPanel runId={sc.run.runId} />
        ) : (
        <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-1.5">
          <GridToolbar
            tab={tab}
            filters={def.filters}
            filter={filter}
            onFilter={(f) => {
              setFilter(f);
              setSelected(null);
            }}
            search={searchInput}
            onSearch={setSearchInput}
            counts={sc.counts}
            total={grid.total}
            knownTotal={grid.knownTotal}
            onExport={exportView}
            selectedCount={checkable ? checkedIds.size : 0}
            onClearSelected={() => setCheckedIds(new Set())}
            paneOpen={layout.paneOpen}
            onTogglePane={() => setLayout({ paneOpen: !layout.paneOpen })}
            sideOpen={layout.sideOpen}
            onToggleSide={() => setLayout({ sideOpen: !layout.sideOpen })}
          />

          <DataGrid
            className="min-h-0 flex-1"
            columns={columns}
            headerLabel={(id) => t(`siteCrawl.col_${id}`)}
            rowCount={grid.total}
            getRow={grid.getRow}
            renderCell={renderCell}
            sort={sort}
            onSort={cycleSort}
            selected={selected}
            onSelect={setSelected}
            onActivate={activateRow}
            onRange={grid.onRange}
            onColumnResize={(colId, width) => setColumnWidth(tab, colId, width)}
            checked={checkable ? checkedIds : undefined}
            onCheck={checkable ? toggleChecked : undefined}
            followTail={followTail}
            jumpLabel={t("siteCrawl.jumpToLatest")}
            empty={empty}
          />

          {layout.paneOpen && sc.run && (
            <>
              <Splitter
                orientation="horizontal"
                cssVar="--sc-pane-h"
                value={layout.paneH}
                min={160}
                max={480}
                onCommit={(v) => setLayout({ paneH: v })}
                rootRef={pageRef}
                label={t("siteCrawl.paneResize")}
              />
              <div className="min-h-0 shrink-0" style={{ height: "var(--sc-pane-h)" }}>
                <DetailPane
                  runId={sc.run.runId}
                  selection={selection}
                  onNavigate={navigate}
                  onClose={() => setLayout({ paneOpen: false })}
                />
              </div>
            </>
          )}
        </div>
        )}

        {layout.sideOpen && (
          <>
            <Splitter
              orientation="vertical"
              cssVar="--sc-side-w"
              value={layout.sideW}
              min={200}
              max={420}
              onCommit={(v) => setLayout({ sideW: v })}
              rootRef={pageRef}
              label={t("siteCrawl.overviewResize")}
            />
            <div className="min-h-0 shrink-0" style={{ width: "var(--sc-side-w)" }}>
              <OverviewPanel
                counts={sc.counts}
                issues={issueCatalog}
                activeTab={tab}
                activeFilter={filter}
                onNavigate={navigate}
              />
            </div>
          </>
        )}
      </div>

      <ConfirmDialog
        open={confirmClear}
        onOpenChange={setConfirmClear}
        title={t("siteCrawl.clearTitle")}
        description={t("siteCrawl.clearBody", { count: sc.runState?.crawled ?? 0 })}
        confirmLabel={t("siteCrawl.clearConfirm")}
        onConfirm={() => void clearRun()}
      />
    </div>
  );
}
