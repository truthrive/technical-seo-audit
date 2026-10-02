import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Lightbulb } from "lucide-react";

import * as SiteCrawlService from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/service";
import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Spinner } from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

interface Opportunity {
  auditId: string;
  title: string;
  pages: number;
  savingsMs: number;
  savingsBytes: number;
}

interface OpportunityPage {
  url: string;
  savingsMs: number;
  savingsBytes: number;
}

const fmtMs = (n: number) => (n > 0 ? `${(n / 1000).toFixed(1)}s` : "—");

function fmtBytes(n: number): string {
  if (n <= 0) return "—";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

// The Opportunities tab: PageSpeed's scores say a page is slow, these say what
// to do about it. Rolled up by audit because the useful unit of work is "fix
// this one thing on the 62 pages it affects", not 62 separate problems — which
// is also why one click drills into the URLs behind a row.
export function OpportunitiesPanel({ runId }: { runId: string }) {
  const { t } = useTranslation();
  const [list, setList] = useState<Opportunity[] | null>(null);
  const [focus, setFocus] = useState<Opportunity | null>(null);
  const [pages, setPages] = useState<OpportunityPage[] | null>(null);

  useEffect(() => {
    let alive = true;
    setList(null);
    setFocus(null);
    setPages(null);
    SiteCrawlService.Opportunities(runId)
      .then((rows) => alive && setList((rows ?? []) as Opportunity[]))
      .catch(() => alive && setList([]));
    return () => {
      alive = false;
    };
  }, [runId]);

  const open = useCallback(
    (o: Opportunity) => {
      setFocus(o);
      setPages(null);
      SiteCrawlService.OpportunityPages(runId, o.auditId)
        .then((rows) => setPages((rows ?? []) as OpportunityPage[]))
        .catch(() => setPages([]));
    },
    [runId],
  );

  if (list === null) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center rounded-xl bg-card shadow-soft">
        <Spinner className="size-5 text-muted-foreground" />
      </div>
    );
  }

  if (list.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center rounded-xl bg-card shadow-soft">
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Lightbulb />
            </EmptyMedia>
            <EmptyTitle>{t("siteCrawl.oppEmptyTitle")}</EmptyTitle>
            <EmptyDescription>{t("siteCrawl.oppEmptyBody")}</EmptyDescription>
          </EmptyHeader>
        </Empty>
      </div>
    );
  }

  if (focus) {
    return (
      <div className="flex min-h-0 flex-1 flex-col gap-2">
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="ghost" size="sm" onClick={() => setFocus(null)}>
            <ArrowLeft data-icon="inline-start" />
            {t("siteCrawl.oppBack")}
          </Button>
          <span className="min-w-0 truncate text-sm font-medium">{focus.title}</span>
          <span className="shrink-0 text-xs text-muted-foreground">
            {t("siteCrawl.oppPagesFor", { count: focus.pages })}
          </span>
        </div>
        <ScrollArea className="min-h-0 flex-1" viewportClassName="h-full">
          <Table className="min-w-[720px]">
            <TableHeader>
              <TableRow>
                <TableHead>{t("siteCrawl.psiColUrl")}</TableHead>
                <TableHead className="w-28 text-right">{t("siteCrawl.oppColSavingsMs")}</TableHead>
                <TableHead className="w-28 text-right">
                  {t("siteCrawl.oppColSavingsBytes")}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(pages ?? []).map((p) => (
                <TableRow key={p.url}>
                  <TableCell>
                    <Tooltip>
                      <TooltipTrigger render={<span className="block max-w-full truncate" />}>
                        {p.url}
                      </TooltipTrigger>
                      <TooltipContent className="max-w-md break-all">{p.url}</TooltipContent>
                    </Tooltip>
                  </TableCell>
                  <TableCell className="w-28 text-right tabular-nums">{fmtMs(p.savingsMs)}</TableCell>
                  <TableCell className="w-28 text-right tabular-nums">
                    {fmtBytes(p.savingsBytes)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </ScrollArea>
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      <ScrollArea className="min-h-0 flex-1" viewportClassName="h-full">
        <Table className="min-w-[720px]">
          <TableHeader>
            <TableRow>
              <TableHead>{t("siteCrawl.oppColAudit")}</TableHead>
              <TableHead className="w-24 text-right">{t("siteCrawl.oppColPages")}</TableHead>
              <TableHead className="w-28 text-right">{t("siteCrawl.oppColSavingsMs")}</TableHead>
              <TableHead className="w-28 text-right">{t("siteCrawl.oppColSavingsBytes")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {list.map((o) => (
              <TableRow
                key={o.auditId}
                className="cursor-pointer"
                onClick={() => open(o)}
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    open(o);
                  }
                }}
              >
                <TableCell className="truncate">{o.title || o.auditId}</TableCell>
                <TableCell className="w-24 text-right tabular-nums">{o.pages}</TableCell>
                <TableCell className="w-28 text-right tabular-nums">{fmtMs(o.savingsMs)}</TableCell>
                <TableCell className="w-28 text-right tabular-nums">
                  {fmtBytes(o.savingsBytes)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </ScrollArea>
    </div>
  );
}
