import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Eye, HardDrive, MoreHorizontal, Play, RotateCcw, Trash2, Waypoints } from "lucide-react";

import * as SiteCrawlService from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/service";
import { RunSummary } from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/models";
import { useSetDetailCrumb } from "@/app/crumb-store";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { DomainFavicon } from "@/components/domain-favicon";
import { HistoryTable, type HistoryColumn } from "@/components/history-table";
import { PageHeader } from "@/components/page-header";
import { RunState, RunWhen } from "@/components/run-cells";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useRunLocked } from "@/features/license/run-lock";

// The full link graph is what makes the Links tab possible, and it is not
// cheap. Past this the header line turns into a soft warning — never an
// automatic deletion (the data is the user's).
const SIZE_WARN_BYTES = 500 * 1024 * 1024;

function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "0 KB";
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function formatDuration(ms: number): string | null {
  if (!Number.isFinite(ms) || ms <= 0) return null;
  const s = Math.round(ms / 1000);
  return s < 60 ? `${s}s` : `${Math.floor(s / 60)}m ${s % 60}s`;
}

// Crawl history: one row per stored run, its share of the workspace database
// shown rather than hidden (design.md §6 run-history + the storage ruling).
// Opening a run routes to the live page in read mode — one grid, not two.
export function SiteCrawlRunsPage() {
  const { t } = useTranslation();
  const runLocked = useRunLocked();
  const navigate = useNavigate();
  useSetDetailCrumb(t("siteCrawl.historyTitle"));

  const [runs, setRuns] = useState<RunSummary[] | null>(null);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("all");
  const [pendingDelete, setPendingDelete] = useState<RunSummary | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setRuns((await SiteCrawlService.Runs()) ?? []);
    } catch (e) {
      setNotice(String(e));
      setRuns([]);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const openRun = (run: RunSummary) => navigate(`/tools/site-crawl/${run.id}`);

  const resumeRun = async (run: RunSummary) => {
    setNotice(null);
    try {
      await SiteCrawlService.Resume(run.id);
      navigate(`/tools/site-crawl/${run.id}`);
    } catch (e) {
      setNotice(String(e).replace(/^sitecrawl:\s*/, ""));
    }
  };

  const recrawlRun = async (run: RunSummary) => {
    setNotice(null);
    try {
      await SiteCrawlService.Recrawl(run.id);
      navigate(`/tools/site-crawl/${run.id}`);
    } catch (e) {
      setNotice(String(e).replace(/^sitecrawl:\s*/, ""));
    }
  };

  const deleteRun = async (run: RunSummary) => {
    setNotice(null);
    try {
      await SiteCrawlService.DeleteRun(run.id);
      void load();
    } catch (e) {
      setNotice(String(e));
    }
  };

  const all = useMemo(() => runs ?? [], [runs]);

  // Paused earns a tab of its own here, unlike the other tools: this is the one
  // crawl you can pick back up, so "which one was I in the middle of" is a
  // question the filter row should answer in one click.
  const counts = useMemo(
    () => ({
      all: all.length,
      running: all.filter((r) => r.state === "running").length,
      paused: all.filter((r) => r.state === "paused").length,
      completed: all.filter((r) => r.state === "completed").length,
      failed: all.filter((r) => r.state === "failed").length,
    }),
    [all],
  );

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return all.filter(
      (r) =>
        (filter === "all" || r.state === filter) &&
        (q === "" ||
          r.host.toLowerCase().includes(q) ||
          r.seedUrl.toLowerCase().includes(q)),
    );
  }, [all, query, filter]);

  const totalBytes = all.reduce((sum, r) => sum + (r.sizeBytes || 0), 0);

  const columns: HistoryColumn<RunSummary>[] = [
    {
      id: "when",
      label: t("runs.colWhen"),
      width: "w-44",
      cell: (r) => <RunWhen iso={r.startedAt} sub={formatDuration(r.durationMs)} />,
    },
    {
      id: "site",
      label: t("siteCrawl.hColSite"),
      grow: true,
      cell: (r) => (
        <Tooltip>
          <TooltipTrigger render={<span className="flex min-w-0 items-center gap-2" />}>
            <DomainFavicon domain={r.host} />
            <span className="truncate">{r.host || r.seedUrl}</span>
          </TooltipTrigger>
          <TooltipContent className="max-w-xs break-all">{r.seedUrl}</TooltipContent>
        </Tooltip>
      ),
    },
    {
      id: "mode",
      label: t("siteCrawl.hColMode"),
      width: "w-20",
      cell: (r) => (
        <Badge variant="outline">
          {r.mode === "list" ? t("siteCrawl.modeList") : t("siteCrawl.modeSpider")}
        </Badge>
      ),
    },
    {
      id: "outcome",
      label: t("siteCrawl.hColOutcome"),
      width: "w-56",
      cell: (r) => (
        <div className="flex flex-wrap items-center gap-1">
          {r.state === "running" && <Spinner className="size-3.5" />}
          <RunState state={r.state} />
          <span className="text-xs text-muted-foreground tabular-nums">
            {t("siteCrawl.urlCount", { count: r.crawled })}
          </span>
          {r.issues > 0 && (
            <Badge variant="warning" className="tabular-nums">
              {t("siteCrawl.issueCount", { count: r.issues })}
            </Badge>
          )}
        </div>
      ),
    },
    {
      id: "size",
      label: t("siteCrawl.hColSize"),
      width: "w-24",
      align: "end",
      cell: (r) => (
        <span className="text-xs text-muted-foreground tabular-nums">
          {formatBytes(r.sizeBytes)}
        </span>
      ),
    },
    {
      id: "actions",
      label: t("runs.colActions"),
      // w-28, not w-24: td adds px-4 (32px), and two icon-sm buttons plus the
      // gap need 68px of room (tables.md §6).
      width: "w-28",
      align: "end",
      sticky: true,
      cell: (r) => {
        const running = r.state === "running";
        return (
          <div className="flex items-center justify-end gap-1">
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant="outline"
                    size="icon-sm"
                    onClick={() => openRun(r)}
                    aria-label={t("runs.open")}
                  />
                }
              >
                <Eye />
              </TooltipTrigger>
              <TooltipContent>{t("runs.open")}</TooltipContent>
            </Tooltip>
            <DropdownMenu>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("common.actions")}
                render={<DropdownMenuTrigger />}
              >
                <MoreHorizontal />
              </Button>
              <DropdownMenuContent align="end">
                <DropdownMenuGroup>
                  {r.resumable && (
                    <DropdownMenuItem
                      className="whitespace-nowrap"
                      disabled={running || runLocked}
                      onClick={() => void resumeRun(r)}
                    >
                      <Play />
                      {t("runs.resume")}
                    </DropdownMenuItem>
                  )}
                  <DropdownMenuItem
                    className="whitespace-nowrap"
                    disabled={running || runLocked}
                    onClick={() => void recrawlRun(r)}
                  >
                    <RotateCcw />
                    {t("siteCrawl.recrawl")}
                  </DropdownMenuItem>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  variant="destructive"
                  className="whitespace-nowrap"
                  disabled={running}
                  onClick={() => setPendingDelete(r)}
                >
                  <Trash2 />
                  {t("common.delete")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        );
      },
    },
  ];

  return (
    <div className="mx-auto flex w-full min-w-0 max-w-5xl flex-col gap-6">
      {/* The ONE back pattern: ArrowLeft ghost icon on the left — the old
          right-aligned "Back to crawl" text button was the outlier. */}
      <PageHeader
        title={t("siteCrawl.historyTitle")}
        subtitle={t("siteCrawl.historySubtitle")}
        back={{ label: t("siteCrawl.backToCrawl"), to: "/tools/site-crawl" }}
      />

      <HistoryTable
        rows={rows}
        columns={columns}
        minWidth={860}
        loading={runs === null}
        query={query}
        onQueryChange={setQuery}
        searchPlaceholder={t("runs.searchHistory")}
        filter={filter}
        onFilterChange={setFilter}
        filters={[
          { id: "all", label: t("runs.filterAll", { count: counts.all }) },
          { id: "running", label: t("runs.filterRunning", { count: counts.running }) },
          { id: "paused", label: t("runs.filterPaused", { count: counts.paused }) },
          { id: "completed", label: t("runs.filterCompleted", { count: counts.completed }) },
          { id: "failed", label: t("runs.filterFailed", { count: counts.failed }) },
        ]}
        emptyIcon={Waypoints}
        emptyTitle={t("siteCrawl.historyEmpty")}
        emptyBody={t("siteCrawl.historyEmptyHint")}
        noMatchTitle={t("siteCrawl.historyNoMatch")}
        noMatchBody={t("siteCrawl.historyNoMatchHint")}
        notice={notice}
        // How much of the workspace database this tool is holding. Shown rather
        // than hidden: the Links tab is worth its size, but the user is the one
        // who gets to decide that.
        toolbarExtra={
          all.length > 0 && (
            <span
              className={
                totalBytes > SIZE_WARN_BYTES
                  ? "flex items-center gap-1.5 text-xs text-warning"
                  : "flex items-center gap-1.5 text-xs text-muted-foreground"
              }
            >
              <HardDrive className="size-3.5" />
              <span className="tabular-nums">
                {totalBytes > SIZE_WARN_BYTES
                  ? t("siteCrawl.sizeWarning", { size: formatBytes(totalBytes) })
                  : t("siteCrawl.totalSize", { count: all.length, size: formatBytes(totalBytes) })}
              </span>
            </span>
          )
        }
        onClear={async () => {
          try {
            await SiteCrawlService.ClearHistory();
            void load();
          } catch (e) {
            setNotice(String(e));
          }
        }}
        clearBody={t("siteCrawl.confirmClearHistoryBody", { size: formatBytes(totalBytes) })}
      />

      <ConfirmDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title={t("runs.confirmDeleteTitle")}
        description={t("siteCrawl.confirmDeleteRunBody", {
          host: pendingDelete?.host ?? "",
          count: pendingDelete?.crawled ?? 0,
        })}
        confirmLabel={t("common.delete")}
        onConfirm={() => {
          if (pendingDelete) void deleteRun(pendingDelete);
          setPendingDelete(null);
        }}
      />
    </div>
  );
}
