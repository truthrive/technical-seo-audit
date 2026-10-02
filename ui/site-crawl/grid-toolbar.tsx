import { useCallback, useLayoutEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, Download, PanelBottom, PanelRight, Search, X } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { Separator } from "@/components/ui/separator";
import { Spinner } from "@/components/ui/spinner";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ErrorText } from "@/components/error-text";
import { cn } from "@/lib/utils";
import { useIsActiveTab } from "@/lib/tab-context";

const EXPORT_FORMATS = ["csv", "json", "xml"] as const;

// Search / filter / export row above the grid. Search and filters are pushed
// down to SQL (FTS5) — the explicit exception to the client-side-filter rule,
// recorded in design.md's data-grid section.
export function GridToolbar({
  tab,
  filters,
  filter,
  onFilter,
  search,
  onSearch,
  counts,
  total,
  knownTotal,
  onExport,
  selectedCount = 0,
  onClearSelected,
  paneOpen,
  onTogglePane,
  sideOpen,
  onToggleSide,
}: {
  tab: string;
  filters: string[];
  filter: string;
  onFilter: (f: string) => void;
  search: string;
  onSearch: (s: string) => void;
  counts: Record<string, number>;
  total: number;
  knownTotal: boolean;
  onExport: (format: string) => Promise<string>;
  /** Checked rows across the run. While non-zero, Export exports only them. */
  selectedCount?: number;
  onClearSelected?: () => void;
  paneOpen: boolean;
  onTogglePane: () => void;
  sideOpen: boolean;
  onToggleSide: () => void;
}) {
  const { t } = useTranslation();
  const [exporting, setExporting] = useState(false);
  const [exportedPath, setExportedPath] = useState<string | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  const runExport = async (format: string) => {
    setExporting(true);
    setExportError(null);
    try {
      const path = await onExport(format);
      // Empty path = the user cancelled Save As; not an error, not a result.
      if (path) setExportedPath(path);
    } catch (e) {
      setExportError(String(e).replace(/^sitecrawl:\s*/, ""));
    } finally {
      setExporting(false);
    }
  };

  const countOf = useCallback(
    (f: string): number | undefined => (f === "all" ? counts[tab] : counts[`${tab}.${f}`]),
    [counts, tab],
  );

  // How many chips actually fit. The list changes per tab (columns.ts) and each
  // label carries a live count, so a fixed "first N" split cannot work — the row
  // has to be measured. Everything past the fit goes into a More menu, the way
  // tab-strip.tsx already groups this screen's 17 tabs.
  const barRef = useRef<HTMLDivElement>(null);
  const measureRef = useRef<HTMLDivElement>(null);
  const [visibleCount, setVisibleCount] = useState(filters.length);

  // Re-measure when a count changes WIDTH, not when it changes value. `counts`
  // ticks continuously during a crawl; keying the effect on it re-ran the
  // measurement on every row and walked chips into the More menu under the
  // user's cursor. Digit length is what actually moves the label.
  const labelWidthKey = filters.map((f) => String(countOf(f) ?? "").length).join(",");
  // A tab opened in the background mounts hidden, where every chip measures 0.
  // Measure once the tab is on screen.
  const tabActive = useIsActiveTab();

  useLayoutEffect(() => {
    const bar = barRef.current;
    const probe = measureRef.current;
    if (!bar || !probe || !tabActive) return;
    // Measure the REAL chip, not a stand-in: ToggleGroupItem at spacing={0}
    // resolves to px-2, adds outline borders and renders its count in tabular
    // figures, none of which a plain span reproduces.
    const widths = Array.from(
      probe.querySelectorAll<HTMLElement>('[data-slot="toggle-group-item"]'),
    ).map((el) => el.offsetWidth);
    const MORE_W = 96; // the More button plus the gap before it
    const recount = () => {
      const room = bar.clientWidth;
      let used = 0;
      let n = 0;
      while (n < widths.length && used + widths[n] <= room) {
        used += widths[n];
        n += 1;
      }
      // Keep dropping chips until the More button itself fits. One pass was not
      // enough: with wide labels the button still overflowed and clipped over
      // Export — the very thing the menu exists to prevent. Reaching zero is a
      // valid answer at the app's minimum width; the menu then holds them all.
      while (n > 0 && n < widths.length && used + MORE_W > room) {
        n -= 1;
        used -= widths[n];
      }
      setVisibleCount(n);
    };
    recount();
    const ro = new ResizeObserver(recount);
    ro.observe(bar);
    return () => ro.disconnect();
  }, [filters, labelWidthKey, tabActive]);

  const tabTotal = counts[tab];
  const narrowed =
    knownTotal && (search.trim() !== "" || filter !== "all") && tabTotal !== undefined && total < tabTotal;

  return (
    <div className="relative flex min-w-0 shrink-0 items-center gap-2">
      <InputGroup className="w-full max-w-64 min-w-32 shrink">
        <InputGroupAddon>
          <Search />
        </InputGroupAddon>
        <InputGroupInput
          placeholder={t("siteCrawl.searchPlaceholder")}
          value={search}
          onChange={(e) => onSearch(e.target.value)}
        />
      </InputGroup>

      <div ref={barRef} className="flex min-w-0 flex-1 items-center gap-1">
        {filter.startsWith("issue:") ? (
          // A jump from the Issues tab / Overview tree: the filter is an issue
          // code no chip row covers — show it as a removable chip instead.
          <Badge variant="primary" className="h-7 gap-1.5 pl-3">
            {t(`siteCrawl.issue_${filter.slice(6)}`, { defaultValue: filter.slice(6) })}
            <button
              type="button"
              aria-label={t("siteCrawl.clearIssueFilter")}
              onClick={() => onFilter("all")}
              className="rounded-full p-0.5 hover:bg-primary/20"
            >
              <X className="size-3" />
            </button>
          </Badge>
        ) : (
          <>
            {/* An empty ToggleGroup still paints its outline, so at zero the row
                shows the More button alone. */}
            {visibleCount > 0 && (
            <ToggleGroup
              value={[filter]}
              onValueChange={(v) => onFilter((v[0] as string) ?? "all")}
              variant="outline"
              size="sm"
              spacing={0}
              className="w-max shrink-0"
            >
              {filters.slice(0, visibleCount).map((f) => {
                const n = countOf(f);
                return (
                  <ToggleGroupItem key={f} value={f} className="whitespace-nowrap">
                    {t(`siteCrawl.filter_${f}`)}
                    {n !== undefined && <span className="ml-1 tabular-nums opacity-60">{n}</span>}
                  </ToggleGroupItem>
                );
              })}
            </ToggleGroup>
            )}
            {visibleCount < filters.length && (
              <FilterOverflow
                hidden={filters.slice(visibleCount)}
                filter={filter}
                onFilter={onFilter}
                countOf={countOf}
              />
            )}
          </>
        )}
      </div>

      {/* Measured off-screen with the SAME markup the row renders, so the widths
          are the real ones — padding, borders and tabular figures included. */}
      <div ref={measureRef} aria-hidden className="pointer-events-none absolute -z-10 opacity-0">
        <ToggleGroup
          value={[]}
          onValueChange={() => {}}
          variant="outline"
          size="sm"
          spacing={0}
          className="w-max"
        >
          {filters.map((f) => {
            const n = countOf(f);
            return (
              <ToggleGroupItem key={f} value={f} className="whitespace-nowrap">
                {t(`siteCrawl.filter_${f}`)}
                {n !== undefined && <span className="ml-1 tabular-nums opacity-60">{n}</span>}
              </ToggleGroupItem>
            );
          })}
        </ToggleGroup>
      </div>

      {knownTotal && (
        <span className="shrink-0 text-xs whitespace-nowrap text-muted-foreground tabular-nums">
          {narrowed
            ? t("siteCrawl.showingOf", { shown: total, total: tabTotal })
            : t("siteCrawl.rowCount", { count: total })}
        </span>
      )}

      {selectedCount > 0 && (
        <Badge variant="primary" className="h-7 shrink-0 gap-1.5 pl-3 tabular-nums">
          {t("siteCrawl.selectedCount", { count: selectedCount })}
          {onClearSelected && (
            <button
              type="button"
              aria-label={t("siteCrawl.clearSelection")}
              onClick={onClearSelected}
              className="rounded-full p-0.5 hover:bg-primary/20"
            >
              <X className="size-3" />
            </button>
          )}
        </Badge>
      )}

      {exportedPath && !exportError && (
        <Tooltip>
          <TooltipTrigger render={<span className="max-w-40 shrink truncate text-xs text-muted-foreground" />}>
            {t("siteCrawl.exportedTo", { path: exportedPath })}
          </TooltipTrigger>
          <TooltipContent className="max-w-sm break-all">{exportedPath}</TooltipContent>
        </Tooltip>
      )}
      <ErrorText error={exportError} className="max-w-40 shrink text-xs" />

      <DropdownMenu>
        <Button
          variant="outline"
          size="sm"
          disabled={exporting || total === 0}
          render={<DropdownMenuTrigger />}
        >
          {exporting ? <Spinner data-icon="inline-start" /> : <Download data-icon="inline-start" />}
          {selectedCount > 0 ? t("siteCrawl.exportSelected", { count: selectedCount }) : t("siteCrawl.export")}
        </Button>
        <DropdownMenuContent align="end" className="w-auto">
          {EXPORT_FORMATS.map((f) => (
            <DropdownMenuItem key={f} className="whitespace-nowrap" onClick={() => void runExport(f)}>
              {t(`siteCrawl.export_${f}`)}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* View toggles are about the workspace, not the data — the separator
          keeps the row reading [search][filters][counts][export] | [view]. */}
      <Separator orientation="vertical" className="h-5" />
      <div className="flex shrink-0 items-center gap-1">
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                aria-pressed={paneOpen}
                aria-label={t("siteCrawl.togglePane")}
                onClick={onTogglePane}
              />
            }
          >
            <PanelBottom className={cn(paneOpen && "text-primary")} />
          </TooltipTrigger>
          <TooltipContent>{t("siteCrawl.togglePane")}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                aria-pressed={sideOpen}
                aria-label={t("siteCrawl.toggleOverview")}
                onClick={onToggleSide}
              />
            }
          >
            <PanelRight className={cn(sideOpen && "text-primary")} />
          </TooltipTrigger>
          <TooltipContent>{t("siteCrawl.toggleOverview")}</TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}

/**
 * The chips that did not fit. The button wears the active chip's own name when
 * the current filter is one of them — a menu labelled "More" while it holds the
 * filter actually in force says nothing about what the grid is showing. Same
 * move tab-strip.tsx makes for this screen's tab groups.
 */
function FilterOverflow({
  hidden,
  filter,
  onFilter,
  countOf,
}: {
  hidden: string[];
  filter: string;
  onFilter: (id: string) => void;
  countOf: (f: string) => number | undefined;
}) {
  const { t } = useTranslation();
  const active = hidden.includes(filter) ? filter : null;

  return (
    <DropdownMenu>
      <Button
        variant={active ? "default" : "outline"}
        size="sm"
        className="shrink-0"
        render={<DropdownMenuTrigger />}
      >
        {active ? t(`siteCrawl.filter_${active}`) : t("siteCrawl.filterMore")}
        <ChevronDown className="size-3.5 opacity-60" />
      </Button>
      {/* w-auto: the popup takes the trigger's width by default and clips the
          labels mid-word. */}
      <DropdownMenuContent align="end" className="w-auto">
        {hidden.map((f) => {
          const n = countOf(f);
          return (
            <DropdownMenuItem
              key={f}
              className="whitespace-nowrap"
              onClick={() => onFilter(f)}
            >
              {t(`siteCrawl.filter_${f}`)}
              {n !== undefined && <span className="ml-2 tabular-nums opacity-60">{n}</span>}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
