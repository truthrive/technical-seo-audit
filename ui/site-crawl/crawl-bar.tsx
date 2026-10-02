import { useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Browser } from "@wailsio/runtime";
import {
  CircleHelp,
  Globe,
  History,
  ListPlus,
  Pause,
  Play,
  Settings2,
  Square,
  Trash2,
  Waypoints,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { RunProgressButton } from "@/components/run-progress-button";
import { ConfigDialog } from "@/features/site-crawl/config-dialog";
import type { UseSiteCrawl } from "@/features/site-crawl/use-site-crawl";
import { RunLock, useRunLocked } from "@/features/license/run-lock";
import { docsUrl } from "@/lib/deep-links";

// URL bar + run controls. Pause and Stop are NOT destructive (Stop keeps
// everything crawled so far) so neither confirms — a dialog on the button you
// press when something looks wrong reads as the app refusing (§0-A). Clear is
// the destructive one and confirms in the page.
//
// Two modes, Screaming Frog's pair: Spider discovers from one URL; List
// crawls exactly a pasted set and follows nothing.
export function CrawlBar({ sc, onClear }: { sc: UseSiteCrawl; onClear: () => void }) {
  const { t } = useTranslation();
  const runLocked = useRunLocked();
  const [seed, setSeed] = useState("");
  const [mode, setMode] = useState<"spider" | "list">("spider");
  const [listUrls, setListUrls] = useState("");
  const [listOpen, setListOpen] = useState(false);
  const [configOpen, setConfigOpen] = useState(false);
  const [starting, setStarting] = useState(false);

  const { run, running, paused, progress } = sc;
  const busy = running || paused;
  const listCount = listUrls.split("\n").filter((l) => l.trim()).length;

  const startCrawl = async () => {
    if (busy || starting) return;
    const seeds =
      mode === "list"
        ? listUrls.split("\n").map((l) => l.trim()).filter(Boolean)
        : [seed.trim()].filter(Boolean);
    if (seeds.length === 0) return;
    setStarting(true);
    try {
      await sc.start(seeds, { mode });
    } finally {
      setStarting(false);
    }
  };

  return (
    // Task-ordered left to right (what to crawl → go → run controls), meta
    // pushed to the far right — one primary action per context (HeroUI #1).
    <div className="flex shrink-0 flex-wrap items-center gap-2">
      {/* The mode is a prefix of the address bar, so the pair sits tight
          (gap-1) and reads as one control, like a browser's scheme selector. */}
      <div className="flex min-w-0 flex-1 items-center gap-1">
        <ToggleGroup
          value={[mode]}
          onValueChange={(v) => setMode((v[0] as "spider" | "list") ?? "spider")}
          variant="outline"
          size="sm"
          spacing={0}
          disabled={busy}
          aria-label={t("siteCrawl.modeLabel")}
        >
          <ToggleGroupItem value="spider">
            <Waypoints className="size-3.5" />
            {t("siteCrawl.modeSpider")}
          </ToggleGroupItem>
          <ToggleGroupItem value="list">
            <ListPlus className="size-3.5" />
            {t("siteCrawl.modeList")}
          </ToggleGroupItem>
        </ToggleGroup>

        {mode === "spider" ? (
          <InputGroup className="w-full max-w-xl min-w-56 flex-1">
            <InputGroupAddon>
              <Globe />
            </InputGroupAddon>
            <InputGroupInput
              placeholder={t("siteCrawl.urlPlaceholder")}
              value={seed}
              disabled={busy}
              onChange={(e) => setSeed(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void startCrawl();
              }}
            />
          </InputGroup>
        ) : (
          <Button variant="outline" disabled={busy} onClick={() => setListOpen(true)}>
            <ListPlus data-icon="inline-start" />
            {listCount > 0
              ? t("siteCrawl.listCount", { count: listCount })
              : t("siteCrawl.listPaste")}
          </Button>
        )}
      </div>

      {running ? (
        <RunProgressButton
          done={progress?.crawled ?? 0}
          total={progress?.found ?? 0}
          posting={!progress}
          i18nPrefix="siteCrawl"
        />
      ) : (
        <RunLock locked={runLocked}>
        <Button
          onClick={() => void startCrawl()}
          disabled={
            busy || starting || runLocked || (mode === "spider" ? !seed.trim() : listCount === 0)
          }
        >
          {starting ? <Spinner data-icon="inline-start" /> : <Play data-icon="inline-start" />}
          {t("siteCrawl.start")}
        </Button>
        </RunLock>
      )}

      {running && (
        <Button variant="outline" onClick={() => void sc.pause()}>
          <Pause data-icon="inline-start" />
          {t("siteCrawl.pause")}
        </Button>
      )}
      {paused && (
        <Button disabled={runLocked} onClick={() => void sc.resume()}>
          <Play data-icon="inline-start" />
          {t("siteCrawl.resume")}
        </Button>
      )}
      {busy && (
        <Tooltip>
          <TooltipTrigger render={<Button variant="outline" onClick={() => void sc.stop()} />}>
            <Square data-icon="inline-start" />
            {t("siteCrawl.stop")}
          </TooltipTrigger>
          <TooltipContent>{t("siteCrawl.stopHint")}</TooltipContent>
        </Tooltip>
      )}

      {/* Meta cluster, far right: things about the tool, not about this run. */}
      <div className="ml-auto flex shrink-0 items-center gap-2">
        <Button variant="outline" disabled={busy} onClick={() => setConfigOpen(true)}>
          <Settings2 data-icon="inline-start" />
          {t("siteCrawl.configuration")}
        </Button>

        {/* This screen builds its own header, so it cannot take PageHeader's
            DocsButton — same icon, same position, wired by hand. */}
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="outline"
                size="icon"
                aria-label={t("common.openDocs", "Open docs")}
                onClick={() => void Browser.OpenURL(docsUrl("tools/site-crawl"))}
              />
            }
          >
            <CircleHelp />
          </TooltipTrigger>
          <TooltipContent>{t("common.openDocs", "Open docs")}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="outline"
                size="icon"
                aria-label={t("siteCrawl.historyTitle")}
                render={<Link to="/tools/site-crawl/runs" />}
              />
            }
          >
            <History />
          </TooltipTrigger>
          <TooltipContent>{t("siteCrawl.historyTitle")}</TooltipContent>
        </Tooltip>

        {run && !busy && (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t("siteCrawl.clear")}
                  onClick={onClear}
                />
              }
            >
              <Trash2 className="text-destructive" />
            </TooltipTrigger>
            <TooltipContent>{t("siteCrawl.clearHint")}</TooltipContent>
          </Tooltip>
        )}
      </div>

      <ConfigDialog open={configOpen} onOpenChange={setConfigOpen} />

      <Dialog open={listOpen} onOpenChange={setListOpen}>
        <DialogContent className="sm:max-w-[600px]">
          <DialogHeader>
            <DialogTitle>{t("siteCrawl.listTitle")}</DialogTitle>
            <DialogDescription>{t("siteCrawl.listHint")}</DialogDescription>
          </DialogHeader>
          <Textarea
            // text-xs alone loses to the base md:text-sm at desktop widths.
            className="h-64 field-sizing-fixed resize-y overflow-y-auto text-xs md:text-xs"
            placeholder={"https://example.com/page-1\nhttps://example.com/page-2"}
            value={listUrls}
            onChange={(e) => setListUrls(e.target.value)}
          />
          <DialogFooter>
            <Button onClick={() => setListOpen(false)}>
              {t("siteCrawl.listDone", { count: listCount })}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
