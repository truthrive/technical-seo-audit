import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { CornerUpRight, ExternalLink, X } from "lucide-react";

import * as SiteCrawlService from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/service";
import { LinkRow, Page } from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/models";
import { DomainFavicon } from "@/components/domain-favicon";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { GridRowData } from "@/components/ui/data-grid";
import { issueTarget } from "@/features/site-crawl/columns";

const LINKS_PAGE = 200;

export interface PaneSelection {
  rowKind: "page" | "link" | "image" | "issue";
  row: GridRowData;
}

type PaneTab = "details" | "inlinks" | "outlinks" | "images" | "serp" | "issues";

// Bottom detail pane: everything a 32px row cannot say. Closed by default —
// the 800px window budget is why — and its row data loads on a 150ms debounce
// with a sequence guard, so holding ArrowDown across 200 rows costs a handful
// of requests, not 200.
export function DetailPane({
  runId,
  selection,
  onNavigate,
  onClose,
}: {
  runId: string;
  selection: PaneSelection | null;
  onNavigate: (tab: string, filter: string) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [tab, setTab] = useState<PaneTab>("details");
  const [page, setPage] = useState<Page | null>(null);
  const [loading, setLoading] = useState(false);
  const seq = useRef(0);

  const isPage = selection !== null && (selection.rowKind === "page" || selection.rowKind === "image");
  const url = isPage ? selection.row.id : "";

  useEffect(() => {
    if (!isPage || !url) {
      setPage(null);
      return;
    }
    const mySeq = ++seq.current;
    setLoading(true);
    const timer = window.setTimeout(() => {
      SiteCrawlService.Page(runId, url)
        .then((p) => {
          if (seq.current === mySeq) setPage(p);
        })
        .catch(() => {
          if (seq.current === mySeq) setPage(null);
        })
        .finally(() => {
          if (seq.current === mySeq) setLoading(false);
        });
    }, 150);
    return () => window.clearTimeout(timer);
  }, [runId, url, isPage]);

  const paneTabs: PaneTab[] = isPage
    ? ["details", "inlinks", "outlinks", "images", "serp", "issues"]
    : ["details"];

  return (
    <div className="flex h-full min-h-0 flex-col rounded-xl bg-card shadow-soft">
      <div className="flex shrink-0 items-center gap-1 border-b border-border/50 px-2 py-1.5">
        {paneTabs.map((pt) => (
          <button
            key={pt}
            type="button"
            onClick={() => setTab(pt)}
            className={cn(
              "rounded-full px-2.5 py-1 text-xs whitespace-nowrap transition-colors",
              tab === pt ? "bg-primary/10 font-medium text-primary" : "text-foreground/60 hover:bg-muted",
            )}
          >
            {t(`siteCrawl.pane_${pt}`)}
          </button>
        ))}
        <span className="min-w-0 flex-1 truncate px-2 text-xs text-muted-foreground">
          {selection?.row.id}
        </span>
        <Button variant="ghost" size="icon-sm" aria-label={t("common.cancel")} onClick={onClose}>
          <X />
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-3">
        {!selection ? (
          <p className="text-sm text-muted-foreground">{t("siteCrawl.paneEmpty")}</p>
        ) : selection.rowKind === "link" ? (
          <LinkDetails row={selection.row} />
        ) : selection.rowKind === "issue" ? (
          <IssueDetails row={selection.row} onNavigate={onNavigate} />
        ) : loading && !page ? (
          <PaneSkeleton />
        ) : !page ? (
          <p className="text-sm text-muted-foreground">{t("siteCrawl.paneEmpty")}</p>
        ) : tab === "details" ? (
          <UrlDetails page={page} />
        ) : tab === "inlinks" ? (
          <LinkList key={`in:${url}`} runId={runId} url={url} side="in" />
        ) : tab === "outlinks" ? (
          <LinkList key={`out:${url}`} runId={runId} url={url} side="out" />
        ) : tab === "images" ? (
          <ImageDetails page={page} />
        ) : tab === "serp" ? (
          <SerpSnippet page={page} />
        ) : (
          <PageIssues page={page} onNavigate={onNavigate} />
        )}
      </div>
    </div>
  );
}

function PaneSkeleton() {
  return (
    <div className="flex flex-col gap-2">
      {[...Array(4)].map((_, i) => (
        <Skeleton key={i} className="h-4 w-full max-w-md" />
      ))}
    </div>
  );
}

function KV({ label, children }: { label: string; children: React.ReactNode }) {
  if (children === null || children === undefined || children === "") return null;
  return (
    <>
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="min-w-0 truncate text-sm">{children}</div>
    </>
  );
}

// Key/value grid, not a table, not a Card (design.md's pane ruling).
function UrlDetails({ page }: { page: Page }) {
  const { t } = useTranslation();
  return (
    <div className="grid max-w-3xl grid-cols-[10rem_minmax(0,1fr)] items-baseline gap-x-4 gap-y-1.5">
      <KV label={t("siteCrawl.col_url")}>{page.url}</KV>
      <KV label={t("siteCrawl.col_status")}>{page.status || t("siteCrawl.statusNone")}</KV>
      <KV label={t("siteCrawl.col_contentType")}>{page.contentType}</KV>
      <KV label={t("siteCrawl.col_indexable")}>
        {page.indexable
          ? t("siteCrawl.indexable")
          : `${t("siteCrawl.nonIndexable")}${page.indexability ? ` — ${t(`siteCrawl.reason_${page.indexability}`, { defaultValue: page.indexability })}` : ""}`}
      </KV>
      <KV label={t("siteCrawl.col_canonical")}>{page.canonical}</KV>
      <KV label={t("siteCrawl.col_metaRobots")}>{page.metaRobots}</KV>
      <KV label={t("siteCrawl.col_xRobots")}>{page.xRobotsTag}</KV>
      <KV label={t("siteCrawl.col_title")}>{page.title}</KV>
      <KV label={t("siteCrawl.col_metaDesc")}>{page.metaDesc}</KV>
      <KV label={t("siteCrawl.col_h1")}>{page.h1.join(" · ")}</KV>
      <KV label={t("siteCrawl.col_wordCount")}>{page.wordCount || ""}</KV>
      <KV label={t("siteCrawl.col_lang")}>{page.lang}</KV>
      <KV label={t("siteCrawl.col_depth")}>{String(page.depth)}</KV>
      <KV label={t("siteCrawl.col_discoveredBy")}>
        {t(`siteCrawl.source_${page.source}`, { defaultValue: page.source })}
      </KV>
      <KV label={t("siteCrawl.col_inlinks")}>{String(page.inlinks)}</KV>
      <KV label={t("siteCrawl.col_size")}>{page.sizeBytes ? `${(page.sizeBytes / 1024).toFixed(1)} KB` : ""}</KV>
      <KV label={t("siteCrawl.col_responseMs")}>{page.responseMs ? `${page.responseMs} ms` : ""}</KV>
      {page.redirects.length > 0 && (
        <>
          <div className="text-xs text-muted-foreground">{t("siteCrawl.paneRedirects")}</div>
          <div className="flex min-w-0 flex-col gap-0.5 text-sm">
            {page.redirects.map((h, i) => (
              <span key={i} className="flex min-w-0 items-center gap-1.5">
                <Badge variant="warning" className="tabular-nums">
                  {h.status}
                </Badge>
                <CornerUpRight className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 truncate">{h.location}</span>
              </span>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

// Windowed link list with an explicit "load more" — an inlink list for a
// homepage can be tens of thousands of rows, so it never loads eagerly.
function LinkList({ runId, url, side }: { runId: string; url: string; side: "in" | "out" }) {
  const { t } = useTranslation();
  const [rows, setRows] = useState<LinkRow[]>([]);
  const [done, setDone] = useState(false);
  const [loading, setLoading] = useState(false);

  const seq = useRef(0);

  const loadMore = (offset: number) => {
    setLoading(true);
    const mySeq = ++seq.current;
    const call = side === "in" ? SiteCrawlService.Inlinks : SiteCrawlService.Outlinks;
    call(runId, url, offset, LINKS_PAGE)
      .then((page) => {
        if (seq.current !== mySeq) return;
        setRows((cur) => (offset === 0 ? (page ?? []) : [...cur, ...(page ?? [])]));
        setDone((page ?? []).length < LINKS_PAGE);
      })
      .catch(() => {
        if (seq.current === mySeq) setDone(true);
      })
      .finally(() => {
        if (seq.current === mySeq) setLoading(false);
      });
  };

  // The first window is debounced like the pane's own page fetch above, and for
  // the same reason: this component is keyed by URL, so arrowing down the grid
  // remounts it per row. Firing immediately turned "hold ArrowDown across 200
  // rows" into 200 uncancellable LIMIT 200 joins, all landing on components that
  // no longer exist — exactly what the pane's 150ms debounce was written to stop.
  useEffect(() => {
    const timer = window.setTimeout(() => loadMore(0), 150);
    return () => {
      window.clearTimeout(timer);
      // Reading the LATEST seq is the whole point: bumping it is how a request
      // already in flight learns its component is gone. A copy taken when the
      // effect ran would be the very number we are trying to invalidate.
      // eslint-disable-next-line react-hooks/exhaustive-deps
      seq.current++;
    };
    // Mount only. The component is keyed by URL, so a different row is a
    // different instance — re-running on loadMore's identity would refetch the
    // first window on every render instead.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!loading && rows.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("siteCrawl.paneNoLinks")}</p>;
  }
  return (
    <div className="flex max-w-4xl flex-col">
      {rows.map((l, i) => (
        <div
          key={i}
          className="grid grid-cols-[minmax(0,1fr)_auto_10rem] items-center gap-3 border-b border-border/40 py-1 text-sm last:border-0"
        >
          <span className="truncate">{side === "in" ? l.source : l.target}</span>
          {l.flags & 2 ? <Badge variant="warning">{t("siteCrawl.filter_nofollow")}</Badge> : <span />}
          <span className="truncate text-xs text-muted-foreground">{l.anchor}</span>
        </div>
      ))}
      {loading && <Skeleton className="my-2 h-4 w-full max-w-md" />}
      {!done && !loading && (
        <Button variant="ghost" size="sm" className="self-start" onClick={() => loadMore(rows.length)}>
          {t("siteCrawl.paneMore")}
        </Button>
      )}
    </div>
  );
}

function ImageDetails({ page }: { page: Page }) {
  const { t } = useTranslation();
  if (page.images.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("siteCrawl.paneNoImages")}</p>;
  }
  return (
    <div className="flex max-w-4xl flex-col">
      {page.images.map((img, i) => (
        <div
          key={i}
          className="grid grid-cols-[minmax(0,1fr)_minmax(0,14rem)] items-center gap-3 border-b border-border/40 py-1 text-sm last:border-0"
        >
          <span className="truncate">{img.src}</span>
          {img.alt ? (
            <span className="truncate text-xs text-muted-foreground">{img.alt}</span>
          ) : (
            <Badge variant="warning">{t("siteCrawl.paneNoAlt")}</Badge>
          )}
        </div>
      ))}
    </div>
  );
}

// How the page would look in Google — rendered purely from the stored record,
// zero backend calls (the keyword-detail-panel look).
function SerpSnippet({ page }: { page: Page }) {
  const { t } = useTranslation();
  let host: string;
  let path = "";
  try {
    const u = new URL(page.url);
    host = u.hostname;
    path = u.pathname === "/" ? "" : ` › ${u.pathname.split("/").filter(Boolean).join(" › ")}`;
  } catch {
    host = page.url;
  }
  return (
    <div className="max-w-xl rounded-xl bg-muted/50 p-4">
      <div className="flex items-center gap-2">
        <DomainFavicon domain={host} />
        <div className="min-w-0">
          <div className="truncate text-xs">{host}</div>
          <div className="truncate text-xs text-muted-foreground">
            {host}
            {path}
          </div>
        </div>
      </div>
      <div className="mt-1 truncate text-lg text-primary">{page.title || page.url}</div>
      <p className="line-clamp-2 text-sm text-muted-foreground">
        {page.metaDesc || t("siteCrawl.paneNoMeta")}
      </p>
    </div>
  );
}

function PageIssues({
  page,
  onNavigate,
}: {
  page: Page;
  onNavigate: (tab: string, filter: string) => void;
}) {
  const { t } = useTranslation();
  if (page.issues.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("siteCrawl.paneNoIssues")}</p>;
  }
  return (
    <div className="flex max-w-3xl flex-col">
      {page.issues.map((code) => {
        const target = issueTarget(code);
        return (
          <div
            key={code}
            className="flex items-center gap-3 border-b border-border/40 py-1.5 text-sm last:border-0"
          >
            <span className="min-w-0 flex-1 truncate">
              {t(`siteCrawl.issue_${code}`, { defaultValue: code })}
            </span>
            {/* A warning without an action is a broken warning (§0-H). */}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onNavigate(target.tab, target.filter)}
            >
              <ExternalLink data-icon="inline-start" />
              {t("siteCrawl.goToAffected")}
            </Button>
          </div>
        );
      })}
    </div>
  );
}

function LinkDetails({ row }: { row: GridRowData }) {
  const { t } = useTranslation();
  const [source, target, anchor, placement] = row.cells;
  return (
    <div className="grid max-w-3xl grid-cols-[10rem_minmax(0,1fr)] items-baseline gap-x-4 gap-y-1.5">
      <KV label={t("siteCrawl.col_source")}>{source}</KV>
      <KV label={t("siteCrawl.col_target")}>{target}</KV>
      <KV label={t("siteCrawl.col_anchor")}>{anchor}</KV>
      <KV label={t("siteCrawl.col_placement")}>
        {t(`siteCrawl.placement_${placement}`, { defaultValue: placement })}
      </KV>
      <div className="text-xs text-muted-foreground">{t("siteCrawl.paneRel")}</div>
      <div className="flex gap-1.5">
        {row.flags & 2 ? (
          <Badge variant="warning">{t("siteCrawl.filter_nofollow")}</Badge>
        ) : (
          <Badge variant="outline">{t("siteCrawl.paneFollow")}</Badge>
        )}
        {(row.flags & 1) === 0 && <Badge variant="secondary">{t("siteCrawl.filter_external")}</Badge>}
      </div>
    </div>
  );
}

function IssueDetails({
  row,
  onNavigate,
}: {
  row: GridRowData;
  onNavigate: (tab: string, filter: string) => void;
}) {
  const { t } = useTranslation();
  const [code, category, severity, urls] = row.cells;
  const target = issueTarget(code);
  const sev = Number(severity);
  return (
    <div className="flex max-w-2xl flex-col items-start gap-2">
      <div className="flex items-center gap-2">
        <Badge variant={sev >= 3 ? "destructive" : sev === 2 ? "warning" : "primary"}>
          {sev >= 3 ? t("siteCrawl.sevCritical") : sev === 2 ? t("siteCrawl.sevWarning") : t("siteCrawl.sevNotice")}
        </Badge>
        <span className="font-medium">{t(`siteCrawl.issue_${code}`, { defaultValue: code })}</span>
      </div>
      <p className="text-sm text-muted-foreground">
        {t("siteCrawl.issueAffects", {
          count: Number(urls),
          category: t(`siteCrawl.cat_${category}`, { defaultValue: category }),
        })}
      </p>
      <Button size="sm" onClick={() => onNavigate(target.tab, target.filter)}>
        <ExternalLink data-icon="inline-start" />
        {t("siteCrawl.goToAffected")}
      </Button>
    </div>
  );
}
