import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import { Gauge, Play } from "lucide-react";

import * as SiteCrawlService from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/service";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { ProgressCircle } from "@/components/ui/progress-circle";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ProviderMissingAlert } from "@/features/providers/missing-key-alert";
import { invalidateProviders, useProviderStates } from "@/features/providers/store";
import { currentOptions } from "@/features/site-crawl/options-store";
import type { PSIProgress } from "@/features/site-crawl/use-site-crawl";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useRunLocked } from "@/features/license/run-lock";

// -1 means "not measured" on every numeric field, which is not 0: a page with
// no real-user data and a page that genuinely scored 0 must not look alike.
interface PSIRow {
  url: string;
  strategy: string;
  fetchedAt: string;
  score: number;
  a11yScore: number;
  seoScore: number;
  bpScore: number;
  fcpMs: number;
  lcpMs: number;
  cls: number;
  tbtMs: number;
  siMs: number;
  cruxSource: string; // url | origin | ""
  cruxVerdict: string;
  cruxLcpMs: number;
  cruxInpMs: number;
  cruxCls: number;
  cruxFcpMs: number;
  cruxTtfbMs: number;
  error?: string;
}

function firstEventPayload<T>(ev: { data: unknown }): T {
  return (Array.isArray(ev.data) ? ev.data[0] : ev.data) as T;
}

function scoreVariant(score: number): "success" | "warning" | "destructive" | "secondary" {
  if (score < 0) return "secondary";
  if (score >= 90) return "success";
  if (score >= 50) return "warning";
  return "destructive";
}

const fmtMs = (n: number) => (n > 0 ? `${(n / 1000).toFixed(1)}s` : "—");

// Module-level so the store hook's id list keeps its identity across renders.
const PSI_PROVIDER = ["pagespeed"];

/** One score cell, or the "—" that says Google did not report that category. */
function ScoreCell({ value, failed }: { value: number; failed?: boolean }) {
  const { t } = useTranslation();
  if (failed) return <Badge variant="destructive">{t("siteCrawl.psiError")}</Badge>;
  return (
    <Badge variant={scoreVariant(value)} className="tabular-nums">
      {value >= 0 ? value : "—"}
    </Badge>
  );
}

// The PageSpeed tab.
//
// Measuring normally happens during the crawl (Configuration → PageSpeed), the
// way Screaming Frog does it. This tab is where the numbers land, and where a
// run crawled before the option was on can be filled in afterwards without
// crawling it again.
export function PageSpeedPanel({ runId, selectedUrls }: { runId: string; selectedUrls: string[] }) {
  const { t } = useTranslation();
  const runLocked = useRunLocked();
  const [strategy, setStrategy] = useState<"mobile" | "desktop">(() =>
    currentOptions().psiStrategy === "desktop" ? "desktop" : "mobile",
  );
  const [results, setResults] = useState<PSIRow[] | null>(null);
  const [progress, setProgress] = useState<PSIProgress | null>(null);
  const [pending, setPending] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  // The key lives in Connections like every other credential; this panel only
  // asks whether one is there.
  const { states } = useProviderStates(PSI_PROVIDER);
  const psi = states[0];

  useEffect(() => {
    let alive = true;
    setResults(null);
    SiteCrawlService.PageSpeedResults(runId)
      .then((list) => alive && setResults((list ?? []) as PSIRow[]))
      .catch(() => alive && setResults([]));
    return () => {
      alive = false;
    };
  }, [runId]);

  // How many pages the Run button would measure. Re-asked whenever the device
  // changes (a desktop pass is a separate measurement) or a result lands.
  const measured = progress?.done ?? 0;
  useEffect(() => {
    let alive = true;
    SiteCrawlService.PendingPageSpeedCount(runId, strategy)
      .then((n) => alive && setPending(n))
      .catch(() => alive && setPending(null));
    return () => {
      alive = false;
    };
  }, [runId, strategy, measured, results]);

  useEffect(() => {
    const offResult = Events.On("sitecrawl:psi-result", (ev: { data: unknown }) => {
      const e = firstEventPayload<{ runId: string; result: PSIRow }>(ev);
      if (e.runId !== runId) return;
      setResults((cur) => {
        const rest = (cur ?? []).filter(
          (r) => !(r.url === e.result.url && r.strategy === e.result.strategy),
        );
        return [e.result, ...rest];
      });
    });
    // Same event the status strip reads, so the two counters can never disagree.
    const offProgress = Events.On("sitecrawl:psi-progress", (ev: { data: unknown }) => {
      const p = firstEventPayload<PSIProgress>(ev);
      if (p.runId !== runId) return;
      setProgress(p);
    });
    return () => {
      offResult();
      offProgress();
    };
  }, [runId]);

  const running = progress?.running === true;
  // No ticked rows means "measure everything still missing" — the same set the
  // crawl would have measured on its own.
  const willMeasure = selectedUrls.length > 0 ? selectedUrls.length : (pending ?? 0);

  const run = useCallback(async () => {
    setError(null);
    try {
      await SiteCrawlService.PageSpeed(runId, selectedUrls, strategy);
    } catch (e) {
      const msg = String(e);
      // The key was removed in Connections since this panel last looked.
      if (msg.includes("missing-key")) invalidateProviders(PSI_PROVIDER);
      else setError(msg.replace(/^sitecrawl:\s*/, ""));
    }
  }, [runId, selectedUrls, strategy]);

  if (psi.loaded && !psi.configured) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center rounded-xl bg-card p-6 shadow-soft">
        <div className="w-full max-w-lg">
          <ProviderMissingAlert
            id="pagespeed"
            title={t("siteCrawl.psiKeyTitle")}
            body={t("siteCrawl.psiKeyBody")}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      <div className="flex shrink-0 flex-wrap items-center gap-2">
        <ToggleGroup
          value={[strategy]}
          onValueChange={(v) => setStrategy((v[0] as "mobile" | "desktop") ?? "mobile")}
          variant="outline"
          size="sm"
          spacing={0}
        >
          <ToggleGroupItem value="mobile">{t("siteCrawl.psiMobile")}</ToggleGroupItem>
          <ToggleGroupItem value="desktop">{t("siteCrawl.psiDesktop")}</ToggleGroupItem>
        </ToggleGroup>

        {running ? (
          <>
            <span className="flex items-center gap-2 text-xs text-muted-foreground">
              <ProgressCircle
                value={
                  progress && progress.total > 0
                    ? (progress.done / progress.total) * 100
                    : undefined
                }
                className="size-6"
              />
              <span className="tabular-nums">
                {progress?.done ?? 0}/{progress?.total ?? 0}
              </span>
            </span>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void SiteCrawlService.StopPageSpeed(runId)}
            >
              {t("siteCrawl.psiStop")}
            </Button>
          </>
        ) : (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  size="sm"
                  disabled={willMeasure === 0 || runLocked}
                  onClick={() => void run()}
                />
              }
            >
              <Play data-icon="inline-start" />
              {selectedUrls.length > 0
                ? t("siteCrawl.psiRunCount", { count: selectedUrls.length })
                : t("siteCrawl.psiRunAll", { count: willMeasure })}
            </TooltipTrigger>
            <TooltipContent className="max-w-xs">{t("siteCrawl.psiRunHint")}</TooltipContent>
          </Tooltip>
        )}

        {!running && willMeasure === 0 && pending !== null && (
          <span className="text-xs text-muted-foreground">{t("siteCrawl.psiNothingToMeasure")}</span>
        )}
        {error && <span className="max-w-72 truncate text-xs text-destructive">{error}</span>}
      </div>

      {results !== null && results.length === 0 && !running ? (
        <div className="flex min-h-0 flex-1 items-center justify-center rounded-xl bg-card shadow-soft">
          <Empty>
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <Gauge />
              </EmptyMedia>
              <EmptyTitle>{t("siteCrawl.psiEmptyTitle")}</EmptyTitle>
              <EmptyDescription>{t("siteCrawl.psiEmptyBody")}</EmptyDescription>
            </EmptyHeader>
          </Empty>
        </div>
      ) : (
        <ScrollArea className="min-h-0 flex-1" viewportClassName="h-full">
          <Table className="min-w-[1320px]">
            <TableHeader>
              {/* Two header rows: the lab numbers and the field numbers are
                  different measurements and must not read as one block. */}
              <TableRow className="hover:bg-transparent">
                <TableHead className="text-muted-foreground" />
                <TableHead colSpan={4} className="text-muted-foreground">
                  {t("siteCrawl.psiScoresGroup")}
                </TableHead>
                <TableHead colSpan={5} className="text-muted-foreground">
                  {t("siteCrawl.psiLabGroup")}
                </TableHead>
                <TableHead colSpan={4} className="text-muted-foreground">
                  {t("siteCrawl.psiFieldGroup")}
                </TableHead>
                <TableHead />
              </TableRow>
              <TableRow>
                <TableHead>{t("siteCrawl.psiColUrl")}</TableHead>
                <TableHead className="w-20">{t("siteCrawl.psiColScore")}</TableHead>
                <TableHead className="w-20">{t("siteCrawl.psiColA11y")}</TableHead>
                <TableHead className="w-20">{t("siteCrawl.psiColSEO")}</TableHead>
                <TableHead className="w-20">{t("siteCrawl.psiColBP")}</TableHead>
                <TableHead className="w-20 text-right">FCP</TableHead>
                <TableHead className="w-20 text-right">LCP</TableHead>
                <TableHead className="w-20 text-right">CLS</TableHead>
                <TableHead className="w-20 text-right">TBT</TableHead>
                <TableHead className="w-20 text-right">SI</TableHead>
                <TableHead className="w-20 text-right">LCP</TableHead>
                <TableHead className="w-20 text-right">INP</TableHead>
                <TableHead className="w-20 text-right">CLS</TableHead>
                <TableHead className="w-20 text-right">TTFB</TableHead>
                <TableHead className="w-24">{t("siteCrawl.psiColStrategy")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(results ?? []).map((r) => {
                const failed = Boolean(r.error);
                // No field block at all: say so once, across the four columns,
                // instead of leaving four blank cells that read as a bug.
                const noField = !failed && r.cruxSource === "";
                return (
                  <TableRow key={`${r.url}|${r.strategy}`}>
                    <TableCell>
                      <Tooltip>
                        <TooltipTrigger render={<span className="block max-w-full truncate" />}>
                          {r.url}
                        </TooltipTrigger>
                        <TooltipContent className="max-w-md break-all">
                          {r.error ? `${r.url} — ${r.error}` : r.url}
                        </TooltipContent>
                      </Tooltip>
                    </TableCell>
                    <TableCell className="w-20">
                      <ScoreCell value={r.score} failed={failed} />
                    </TableCell>
                    <TableCell className="w-20">
                      <ScoreCell value={r.a11yScore} />
                    </TableCell>
                    <TableCell className="w-20">
                      <ScoreCell value={r.seoScore} />
                    </TableCell>
                    <TableCell className="w-20">
                      <ScoreCell value={r.bpScore} />
                    </TableCell>

                    <TableCell className="w-20 text-right tabular-nums">{fmtMs(r.fcpMs)}</TableCell>
                    <TableCell className="w-20 text-right tabular-nums">{fmtMs(r.lcpMs)}</TableCell>
                    <TableCell className="w-20 text-right tabular-nums">
                      {failed ? "—" : r.cls.toFixed(3)}
                    </TableCell>
                    <TableCell className="w-20 text-right tabular-nums">
                      {r.tbtMs > 0 ? `${r.tbtMs}ms` : "—"}
                    </TableCell>
                    <TableCell className="w-20 text-right tabular-nums">{fmtMs(r.siMs)}</TableCell>

                    {noField ? (
                      <TableCell colSpan={4} className="text-xs text-muted-foreground">
                        {t("siteCrawl.psiCruxNone")}
                      </TableCell>
                    ) : (
                      <>
                        <FieldCell source={r.cruxSource}>{fmtMs(r.cruxLcpMs)}</FieldCell>
                        <FieldCell source={r.cruxSource}>
                          {r.cruxInpMs >= 0 ? `${r.cruxInpMs}ms` : "—"}
                        </FieldCell>
                        <FieldCell source={r.cruxSource}>
                          {r.cruxCls >= 0 ? r.cruxCls.toFixed(2) : "—"}
                        </FieldCell>
                        <FieldCell source={r.cruxSource}>{fmtMs(r.cruxTtfbMs)}</FieldCell>
                      </>
                    )}

                    <TableCell className="w-24 text-xs text-muted-foreground">
                      {t(`siteCrawl.psi_${r.strategy}`, { defaultValue: r.strategy })}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </ScrollArea>
      )}
    </div>
  );
}

/** A field number, dimmed and explained when it belongs to the whole domain
 *  rather than to this page — borrowed numbers shown as the page's own would
 *  be the one lie this table must not tell. */
function FieldCell({ source, children }: { source: string; children: React.ReactNode }) {
  const { t } = useTranslation();
  if (source !== "origin") {
    return <TableCell className="w-20 text-right tabular-nums">{children}</TableCell>;
  }
  return (
    <TableCell className="w-20 text-right tabular-nums text-muted-foreground">
      <Tooltip>
        <TooltipTrigger render={<span />}>{children}</TooltipTrigger>
        <TooltipContent className="max-w-xs">{t("siteCrawl.psiCruxOrigin")}</TooltipContent>
      </Tooltip>
    </TableCell>
  );
}
