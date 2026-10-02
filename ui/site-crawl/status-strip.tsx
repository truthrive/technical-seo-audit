import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import type { UseSiteCrawl } from "@/features/site-crawl/use-site-crawl";

function formatEta(sec: number): string {
  if (sec < 0) return "—";
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  if (m < 60) return `${m}m ${sec % 60}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

function stateVariant(state: string): "primary" | "warning" | "success" | "destructive" | "secondary" {
  switch (state) {
    case "running":
      return "primary";
    case "paused":
      return "warning";
    case "completed":
      return "success";
    case "failed":
      return "destructive";
    default:
      return "secondary"; // stopped / cancelled: neutral, user's own call
  }
}

// One 24px line under the URL bar: what the crawl is doing right now.
// Numbers come from the ≤2/s aggregate progress event — nothing here polls.
export function StatusStrip({ sc }: { sc: UseSiteCrawl }) {
  const { t } = useTranslation();
  const { runState, progress, running, psi } = sc;
  if (!runState) return null;

  const found = progress?.found ?? runState.found;
  const crawled = progress?.crawled ?? runState.crawled;
  const issues = progress?.issues;

  return (
    <div className="flex min-w-0 shrink-0 flex-col">
      <div className="flex h-6 min-w-0 shrink-0 items-center gap-3 text-xs text-muted-foreground">
      <Badge variant={stateVariant(runState.state)}>
        {t(`siteCrawl.state_${runState.state}`, { defaultValue: runState.state })}
      </Badge>
      {running && runState.phase !== "crawling" && (
        <span>{t(`siteCrawl.phase_${runState.phase}`, { defaultValue: runState.phase })}</span>
      )}

      <span className="tabular-nums">
        {t("siteCrawl.crawledOf", { crawled, found })}
      </span>
      {running && progress && (
        <>
          <span className="tabular-nums">{t("siteCrawl.queued", { count: progress.queued })}</span>
          <span className="tabular-nums">{progress.rate.toFixed(1)} URL/s</span>
          <span className="tabular-nums">
            {t("siteCrawl.eta")} {formatEta(progress.etaSec)}
          </span>
        </>
      )}

      {/* A Crawl-delay is the single biggest thing that can hold a crawl back,
          and it comes from the site rather than from any setting the user chose.
          Say it either way: obeying it silently and ignoring it silently both
          leave an unexplained speed on screen (§0-H). */}
      {running && (progress?.crawlDelayAskedMs ?? 0) > 0 && (
        <span className="shrink-0 text-warning">
          {progress!.crawlDelayAppliedMs > 0
            ? t("siteCrawl.crawlDelayApplied", { sec: (progress!.crawlDelayAppliedMs / 1000).toFixed(1) })
            : t("siteCrawl.crawlDelayIgnored", { sec: (progress!.crawlDelayAskedMs / 1000).toFixed(1) })}
        </span>
      )}

      {/* Current URL takes whatever room is left. */}
      <span className="min-w-0 flex-1 truncate">{running ? progress?.current : ""}</span>

      {issues && (issues.critical > 0 || issues.warning > 0 || issues.notice > 0) && (
        <span className="flex shrink-0 items-center gap-2 tabular-nums">
          <IssueDot className="bg-destructive" count={issues.critical} />
          <IssueDot className="bg-warning" count={issues.warning} />
          <IssueDot className="bg-primary" count={issues.notice} />
        </span>
      )}
      </div>

      {/* PageSpeed gets its own line because it has its own clock: Google takes
          ~20s a page, so this bar keeps moving long after the crawl line is
          done, and it can be stopped without stopping the crawl. */}
      {psi && psi.total > 0 && (
        <div className="flex h-6 min-w-0 items-center gap-3 text-xs text-muted-foreground">
          <span className="shrink-0">{t("siteCrawl.psiBarLabel")}</span>
          <Progress value={(psi.done / psi.total) * 100} className="h-1.5 w-40 shrink-0" />
          <span className="shrink-0 tabular-nums">
            {psi.done}/{psi.total}
          </span>
          {psi.running ? (
            <Button
              variant="ghost"
              size="sm"
              className="h-6 shrink-0 px-2"
              onClick={() => void sc.stopPageSpeed()}
            >
              {t("siteCrawl.psiStop")}
            </Button>
          ) : (
            psi.stopped && <span className="shrink-0">{t("siteCrawl.psiStopped")}</span>
          )}
        </div>
      )}
    </div>
  );
}

function IssueDot({ className, count }: { className: string; count: number }) {
  if (count === 0) return null;
  return (
    <span className="flex items-center gap-1">
      <span className={cn("size-1.5 rounded-full", className)} />
      {count}
    </span>
  );
}
