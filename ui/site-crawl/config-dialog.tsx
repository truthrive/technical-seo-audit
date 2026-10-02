import { useEffect, useState, useId } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { ProviderMissingAlert } from "@/features/providers/missing-key-alert";
import { useProviderStates } from "@/features/providers/store";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { CellSlider } from "@/components/cell-slider";
import { HelpTip } from "@/components/help-tip";
import { cn } from "@/lib/utils";
import {
  DEFAULT_OPTIONS,
  resetOptions,
  updateOptions,
  useCrawlOptions,
  type CrawlOptions,
} from "@/features/site-crawl/options-store";

const UA_PRESETS = ["sitecrawl", "googlebot", "googlebot-mobile", "bingbot", "chrome", "chrome-mobile"];
const PSI_STRATEGIES = ["mobile", "desktop"];
// Module-level so the provider store's id list keeps its identity across renders.
const CFG_PROVIDERS = ["pagespeed", "proxy"];

// The Screaming Frog-style Configuration dialog: every crawl option, grouped
// in tabs, editing the global options store directly (settings apply to the
// next crawl; a run snapshots them at Start). Ranges mirror Go's clamps.
//
// Options here are full-width label-left / control-right rows, NOT
// OptionCells: that language is white cells on the grey page background —
// on a white dialog it reads as ragged pills.
export function ConfigDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const uid = useId();
  const { t } = useTranslation();
  const opts = useCrawlOptions();
  const [tab, setTab] = useState("crawler");
  const [psi, proxy] = useProviderStates(CFG_PROVIDERS).states;

  const set = (patch: Partial<CrawlOptions>) => updateOptions(patch);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[850px]">
        <DialogHeader>
          <DialogTitle>{t("siteCrawl.configTitle")}</DialogTitle>
          <DialogDescription>{t("siteCrawl.configHint")}</DialogDescription>
        </DialogHeader>

        <Tabs value={tab} onValueChange={(v) => setTab(String(v))} className="min-w-0 gap-3">
          <TabsList>
            <TabsTrigger value="crawler">{t("siteCrawl.cfgCrawler")}</TabsTrigger>
            <TabsTrigger value="requests">{t("siteCrawl.cfgRequests")}</TabsTrigger>
            <TabsTrigger value="filters">{t("siteCrawl.cfgFilters")}</TabsTrigger>
            <TabsTrigger value="javascript">{t("siteCrawl.cfgJavaScript")}</TabsTrigger>
            <TabsTrigger value="pagespeed">{t("siteCrawl.cfgPageSpeed")}</TabsTrigger>
            <TabsTrigger value="issues">{t("siteCrawl.cfgIssues")}</TabsTrigger>
            <TabsTrigger value="duplicates">{t("siteCrawl.cfgDuplicates")}</TabsTrigger>
          </TabsList>

          <div className="max-h-[55vh] min-h-72 overflow-x-hidden overflow-y-auto pr-2">
            <TabsContent value="crawler" className="flex flex-col gap-3">
              <NumRow id={`${uid}-sc-depth`} label={t("siteCrawl.optMaxDepth")} help={t("siteCrawl.helpMaxDepth")}
                value={opts.maxDepth} min={1} max={30} onChange={(v) => set({ maxDepth: v })} />
              <NumRow id={`${uid}-sc-maxurls`} label={t("siteCrawl.optMaxURLs")} help={t("siteCrawl.helpMaxURLs")}
                value={opts.maxURLs} min={1} max={500000} w="w-28" onChange={(v) => set({ maxURLs: v })} />
              <NumRow id={`${uid}-sc-delay`} label={t("siteCrawl.optDelay")} help={t("siteCrawl.helpDelay")}
                value={opts.crawlDelayMs} min={0} max={60000} w="w-24" onChange={(v) => set({ crawlDelayMs: v })} />
              <SwitchRow id={`${uid}-sc-redirects`} label={t("siteCrawl.optFollowRedirects")}
                checked={opts.followRedirects} onChange={(v) => set({ followRedirects: v })} />
              <SwitchRow id={`${uid}-sc-external`} label={t("siteCrawl.optCrawlExternal")} help={t("siteCrawl.helpCrawlExternal")}
                checked={opts.crawlExternal} onChange={(v) => set({ crawlExternal: v })} />
              <SwitchRow id={`${uid}-sc-images`} label={t("siteCrawl.optCrawlImages")} help={t("siteCrawl.helpCrawlImages")}
                checked={opts.crawlImages} onChange={(v) => set({ crawlImages: v })} />
              <SwitchRow id={`${uid}-sc-css`} label={t("siteCrawl.optCrawlCSS")} help={t("siteCrawl.helpCrawlCSS")}
                checked={opts.crawlCSS} onChange={(v) => set({ crawlCSS: v })} />
              <SwitchRow id={`${uid}-sc-js`} label={t("siteCrawl.optCrawlJS")} help={t("siteCrawl.helpCrawlJS")}
                checked={opts.crawlJS} onChange={(v) => set({ crawlJS: v })} />
              <SwitchRow id={`${uid}-sc-subdomains`} label={t("siteCrawl.optSubdomains")} help={t("siteCrawl.helpSubdomains")}
                checked={opts.crawlSubdomains} onChange={(v) => set({ crawlSubdomains: v })} />
              <SwitchRow id={`${uid}-sc-query`} label={t("siteCrawl.optIgnoreQuery")} help={t("siteCrawl.helpIgnoreQuery")}
                checked={opts.ignoreQueryParam} onChange={(v) => set({ ignoreQueryParam: v })} />
            </TabsContent>

            <TabsContent value="requests" className="flex flex-col gap-3">
              <Field orientation="horizontal" className="min-h-8">
                <div className="flex flex-1 items-center gap-1.5">
                  <FieldLabel htmlFor={`${uid}-sc-ua`}>{t("siteCrawl.optUserAgent")}</FieldLabel>
                  <HelpTip>{t("siteCrawl.helpUserAgent")}</HelpTip>
                </div>
                <Select value={opts.userAgent} onValueChange={(v) => set({ userAgent: String(v) })}>
                  <SelectTrigger id={`${uid}-sc-ua`} className="w-44 shrink-0">
                    <SelectValue>{(v: string) => t(`siteCrawl.ua_${v}`, { defaultValue: v })}</SelectValue>
                  </SelectTrigger>
                  <SelectContent className="w-auto">
                    <SelectGroup>
                      {UA_PRESETS.map((p) => (
                        <SelectItem key={p} value={p}>
                          {t(`siteCrawl.ua_${p}`, { defaultValue: p })}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <div className="flex flex-col gap-1">
                <NumRow id={`${uid}-sc-threads`} label={t("siteCrawl.optThreads")} help={t("siteCrawl.helpThreads")}
                  value={opts.concurrency} min={1} max={50} onChange={(v) => set({ concurrency: v })} />
                {/* Every thread hits the same host, so this number IS the load on
                    the user's own server. Go used to clamp it to 8 in silence,
                    which made the field lie; it obeys now, so the caution is here. */}
                {opts.concurrency > 10 && (
                  <FieldDescription className="text-warning">{t("siteCrawl.warnThreads")}</FieldDescription>
                )}
              </div>
              <NumRow id={`${uid}-sc-timeout`} label={t("siteCrawl.optTimeout")}
                value={opts.timeoutSec} min={1} max={120} onChange={(v) => set({ timeoutSec: v })} />
              <NumRow id={`${uid}-sc-retries`} label={t("siteCrawl.optRetries")}
                value={opts.retries} min={0} max={5} onChange={(v) => set({ retries: v })} />
              <SwitchRow id={`${uid}-sc-robots`} label={t("siteCrawl.optRespectRobots")} help={t("siteCrawl.helpRespectRobots")}
                checked={opts.respectRobots} onChange={(v) => set({ respectRobots: v })} />
              {/* Disallow rules and Crawl-delay are separate choices: the first is
                  about what may be crawled, the second only about speed. Nested
                  under the robots switch because Go never reads the file without it. */}
              {opts.respectRobots && (
                <SwitchRow id={`${uid}-sc-crawl-delay`} label={t("siteCrawl.optRespectCrawlDelay")}
                  help={t("siteCrawl.helpRespectCrawlDelay")}
                  checked={opts.respectCrawlDelay} onChange={(v) => set({ respectCrawlDelay: v })} />
              )}
              <SwitchRow id={`${uid}-sc-sitemaps`} label={t("siteCrawl.optSitemaps")} help={t("siteCrawl.helpSitemaps")}
                checked={opts.discoverSitemaps} onChange={(v) => set({ discoverSitemaps: v })} />
              <SwitchRow id={`${uid}-sc-ssl`} label={t("siteCrawl.optIgnoreSSL")} help={t("siteCrawl.helpIgnoreSSL")}
                checked={opts.ignoreSSL} onChange={(v) => set({ ignoreSSL: v })} />
              {/* Opt-in, like Screaming Frog's proxy setting. Saving a proxy in
                  Connections for another tool must not re-route crawls. */}
              <SwitchRow id={`${uid}-sc-proxy`} label={t("siteCrawl.optUseProxy")} help={t("siteCrawl.helpUseProxy")}
                checked={opts.useProxy} onChange={(v) => set({ useProxy: v })} />
              {opts.useProxy && proxy?.loaded && !proxy.configured && (
                <ProviderMissingAlert
                  id="proxy"
                  severity="warning"
                  title={t("siteCrawl.proxyMissingTitle")}
                  body={t("siteCrawl.proxyMissingBody")}
                />
              )}
              <TextRow id={`${uid}-sc-lang`} label={t("siteCrawl.optAcceptLanguage")} placeholder="vi-VN,vi;q=0.9"
                value={opts.acceptLanguage} onChange={(v) => set({ acceptLanguage: v })} />
              <LinesArea id={`${uid}-sc-headers`} label={t("siteCrawl.optHeaders")} help={t("siteCrawl.helpHeaders")}
                placeholder={"X-Example: value"}
                value={headerLines(opts.customHeaders)}
                onChange={(lines) => set({ customHeaders: parseHeaders(lines) })} />
            </TabsContent>

            <TabsContent value="filters" className="flex flex-col gap-3">
              <TextRow id={`${uid}-sc-incext`} label={t("siteCrawl.optIncludeExt")} placeholder="html, php"
                value={opts.includeExtensions.join(", ")}
                onChange={(v) => set({ includeExtensions: csv(v) })} />
              <TextRow id={`${uid}-sc-excext`} label={t("siteCrawl.optExcludeExt")} placeholder="pdf, zip"
                value={opts.excludeExtensions.join(", ")}
                onChange={(v) => set({ excludeExtensions: csv(v) })} />
              <NumRow id={`${uid}-sc-maxsize`} label={t("siteCrawl.optMaxSize")}
                value={opts.maxFileSizeMB} min={1} max={1000} onChange={(v) => set({ maxFileSizeMB: v })} />
              <LinesArea id={`${uid}-sc-incpat`} label={t("siteCrawl.optIncludePatterns")} help={t("siteCrawl.helpPatterns")}
                placeholder="^https://example\\.com/blog/"
                value={opts.includePatterns} onChange={(lines) => set({ includePatterns: lines })} />
              <LinesArea id={`${uid}-sc-excpat`} label={t("siteCrawl.optExcludePatterns")} help={t("siteCrawl.helpPatterns")}
                placeholder="\\?replytocom="
                value={opts.excludePatterns} onChange={(lines) => set({ excludePatterns: lines })} />
            </TabsContent>

            <TabsContent value="javascript" className="flex flex-col gap-3">
              <SwitchRow id={`${uid}-sc-jsrender`} label={t("siteCrawl.optEnableJS")} help={t("siteCrawl.helpEnableJS")}
                checked={opts.enableJavaScript} onChange={(v) => set({ enableJavaScript: v })} />
              <NumRow id={`${uid}-sc-jsmax`} label={t("siteCrawl.optJSMaxPages")} help={t("siteCrawl.helpJSMaxPages")}
                value={opts.jsMaxPages} min={1} max={10000} w="w-24" onChange={(v) => set({ jsMaxPages: v })} />
              <NumRow id={`${uid}-sc-jswait`} label={t("siteCrawl.optJSWait")} help={t("siteCrawl.helpJSWait")}
                value={opts.jsWaitMs} min={0} max={30000} w="w-24" onChange={(v) => set({ jsWaitMs: v })} />
              <NumRow id={`${uid}-sc-jstimeout`} label={t("siteCrawl.optJSTimeout")}
                value={opts.jsTimeoutSec} min={5} max={120} onChange={(v) => set({ jsTimeoutSec: v })} />
              <NumRow id={`${uid}-sc-jsconc`} label={t("siteCrawl.optJSConcurrency")} help={t("siteCrawl.helpJSConcurrency")}
                value={opts.jsConcurrency} min={1} max={10} onChange={(v) => set({ jsConcurrency: v })} />
              <NumRow id={`${uid}-sc-jsvw`} label={t("siteCrawl.optJSViewportW")}
                value={opts.jsViewportWidth} min={320} max={4000} w="w-24" onChange={(v) => set({ jsViewportWidth: v })} />
              <NumRow id={`${uid}-sc-jsvh`} label={t("siteCrawl.optJSViewportH")}
                value={opts.jsViewportHeight} min={320} max={3000} w="w-24" onChange={(v) => set({ jsViewportHeight: v })} />
              <LinesArea id={`${uid}-sc-jspat`} label={t("siteCrawl.optJSPatterns")} help={t("siteCrawl.helpJSPatterns")}
                placeholder="/products/"
                value={opts.jsPatterns} onChange={(lines) => set({ jsPatterns: lines })} />
            </TabsContent>

            <TabsContent value="pagespeed" className="flex flex-col gap-3">
              <SwitchRow id={`${uid}-sc-psi`} label={t("siteCrawl.optEnablePageSpeed")} help={t("siteCrawl.helpEnablePageSpeed")}
                checked={opts.enablePageSpeed} onChange={(v) => set({ enablePageSpeed: v })} />
              <Field orientation="horizontal" className="min-h-8">
                <div className="flex flex-1 items-center gap-1.5">
                  <FieldLabel htmlFor={`${uid}-sc-psi-device`}>{t("siteCrawl.optPSIStrategy")}</FieldLabel>
                  <HelpTip>{t("siteCrawl.helpPSIStrategy")}</HelpTip>
                </div>
                <Select value={opts.psiStrategy} onValueChange={(v) => set({ psiStrategy: String(v) })}>
                  <SelectTrigger id={`${uid}-sc-psi-device`} className="w-44 shrink-0">
                    <SelectValue>{(v: string) => t(`siteCrawl.psi_${v}`, { defaultValue: v })}</SelectValue>
                  </SelectTrigger>
                  <SelectContent className="w-auto">
                    <SelectGroup>
                      {PSI_STRATEGIES.map((p) => (
                        <SelectItem key={p} value={p}>
                          {t(`siteCrawl.psi_${p}`, { defaultValue: p })}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              {/* The cost is minutes-to-hours and it comes from Google, not from
                  anything the crawler can speed up. Saying it here is the whole
                  reason this tab exists rather than one more switch on Crawler. */}
              {opts.enablePageSpeed && (
                <FieldDescription className="text-warning">{t("siteCrawl.psiSlowWarn")}</FieldDescription>
              )}
              {opts.enablePageSpeed && psi.loaded && !psi.configured && (
                <ProviderMissingAlert
                  id="pagespeed"
                  severity="warning"
                  title={t("siteCrawl.psiKeyTitle")}
                  body={t("siteCrawl.psiConfigNeedsKey")}
                />
              )}
            </TabsContent>

            <TabsContent value="issues" className="flex flex-col gap-3">
              <SwitchRow id={`${uid}-sc-defexcl`} label={t("siteCrawl.optDefaultExcl")} help={t("siteCrawl.helpDefaultExcl")}
                checked={opts.useDefaultExcl} onChange={(v) => set({ useDefaultExcl: v })} />
              <LinesArea id={`${uid}-sc-exclusions`} label={t("siteCrawl.optExclusions")} help={t("siteCrawl.helpExclusions")}
                placeholder="/wp-admin/*"
                value={opts.issueExclusions} onChange={(lines) => set({ issueExclusions: lines })} />
            </TabsContent>

            <TabsContent value="duplicates" className="flex flex-col gap-3">
              <SwitchRow id={`${uid}-sc-dup`} label={t("siteCrawl.optDuplication")} help={t("siteCrawl.helpDuplication")}
                checked={opts.enableDuplication} onChange={(v) => set({ enableDuplication: v })} />
              <CellSlider
                className="w-full"
                label={t("siteCrawl.optDupThreshold")}
                help={t("siteCrawl.helpDupThreshold")}
                min={0.5}
                max={1}
                step={0.01}
                value={opts.duplicationThreshold}
                onValueChange={(v) => set({ duplicationThreshold: Math.round(v * 100) / 100 })}
              />
            </TabsContent>
          </div>
        </Tabs>

        <DialogFooter className="justify-between sm:justify-between">
          <Button variant="ghost" onClick={() => resetOptions()}>
            {t("siteCrawl.configReset")}
          </Button>
          <Button onClick={() => onOpenChange(false)}>{t("common.done")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function csv(v: string): string[] {
  return v
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

function headerLines(h: Record<string, string>): string[] {
  return Object.entries(h).map(([k, v]) => `${k}: ${v}`);
}

function parseHeaders(lines: string[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const line of lines) {
    const idx = line.indexOf(":");
    if (idx <= 0) continue;
    const k = line.slice(0, idx).trim();
    const v = line.slice(idx + 1).trim();
    if (k) out[k] = v;
  }
  return out;
}

/** Number option row: free typing, clamped on blur (house rule for threads). */
function NumRow({
  id,
  label,
  help,
  value,
  min,
  max,
  w = "w-20",
  onChange,
}: {
  id: string;
  label: string;
  help?: string;
  value: number;
  min: number;
  max: number;
  w?: string;
  onChange: (v: number) => void;
}) {
  const [raw, setRaw] = useState(String(value));
  useEffect(() => setRaw(String(value)), [value]);
  const commit = () => {
    const n = Number.parseInt(raw, 10);
    const v = Number.isNaN(n) ? value : Math.min(max, Math.max(min, n));
    setRaw(String(v));
    if (v !== value) onChange(v);
  };
  return (
    <Field orientation="horizontal" className="min-h-8">
      <div className="flex flex-1 items-center gap-1.5">
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        {help && <HelpTip>{help}</HelpTip>}
      </div>
      <Input
        id={id}
        type="number"
        min={min}
        max={max}
        className={cn("shrink-0", w)}
        value={raw}
        onChange={(e) => setRaw(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => e.key === "Enter" && commit()}
      />
    </Field>
  );
}

function SwitchRow({
  id,
  label,
  help,
  checked,
  onChange,
}: {
  id: string;
  label: string;
  help?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <Field orientation="horizontal" className="min-h-8">
      <div className="flex flex-1 items-center gap-1.5">
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        {help && <HelpTip>{help}</HelpTip>}
      </div>
      <Switch id={id} checked={checked} onCheckedChange={onChange} />
    </Field>
  );
}

function TextRow({
  id,
  label,
  placeholder,
  value,
  onChange,
}: {
  id: string;
  label: string;
  placeholder?: string;
  value: string;
  onChange: (v: string) => void;
}) {
  const [raw, setRaw] = useState(value);
  useEffect(() => setRaw(value), [value]);
  return (
    <Field orientation="horizontal" className="min-h-8">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Input
        id={id}
        className="w-44 shrink-0"
        placeholder={placeholder}
        value={raw}
        onChange={(e) => setRaw(e.target.value)}
        onBlur={() => onChange(raw)}
      />
    </Field>
  );
}

/** Full-width textarea for line-per-entry lists (patterns, headers, globs). */
function LinesArea({
  id,
  label,
  help,
  placeholder,
  value,
  onChange,
}: {
  id: string;
  label: string;
  help?: string;
  placeholder?: string;
  value: string[];
  onChange: (lines: string[]) => void;
}) {
  const [raw, setRaw] = useState(value.join("\n"));
  useEffect(() => setRaw(value.join("\n")), [value]);
  return (
    <div className="flex w-full flex-col gap-1.5">
      <div className="flex items-center gap-1.5">
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        {help && <HelpTip>{help}</HelpTip>}
      </div>
      <Textarea
        id={id}
        // text-xs alone loses to the base md:text-sm at desktop widths.
        className="h-24 field-sizing-fixed resize-y overflow-y-auto text-xs md:text-xs"
        placeholder={placeholder}
        value={raw}
        onChange={(e) => setRaw(e.target.value)}
        onBlur={() => onChange(raw.split("\n").map((s) => s.trim()).filter(Boolean))}
      />
    </div>
  );
}

// Re-exported so the CrawlBar's Configuration button and the page share one
// default set without importing the store twice.
export { DEFAULT_OPTIONS };
