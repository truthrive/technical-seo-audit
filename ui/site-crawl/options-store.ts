import { useSyncExternalStore } from "react";

// One global crawl configuration (the user's ruling: one settings set, not
// per-site presets), persisted in localStorage and snapshotted into each run
// by Go. Numbers mirror the Go clamps in types.go — the source comment there
// names this file as the owner of the boolean defaults, since a Go bool's
// zero value cannot distinguish "off" from "unset".

export interface CrawlOptions {
  mode: string;
  maxDepth: number;
  maxURLs: number;
  crawlDelayMs: number;
  followRedirects: boolean;
  crawlExternal: boolean;
  crawlSubdomains: boolean;
  ignoreQueryParam: boolean;
  crawlImages: boolean;
  crawlCSS: boolean;
  crawlJS: boolean;
  userAgent: string;
  timeoutSec: number;
  retries: number;
  acceptLanguage: string;
  concurrency: number;
  respectRobots: boolean;
  respectCrawlDelay: boolean;
  discoverSitemaps: boolean;
  ignoreSSL: boolean;
  useProxy: boolean;
  customHeaders: Record<string, string>;
  includeExtensions: string[];
  excludeExtensions: string[];
  includePatterns: string[];
  excludePatterns: string[];
  maxFileSizeMB: number;
  issueExclusions: string[];
  useDefaultExcl: boolean;
  enableDuplication: boolean;
  duplicationThreshold: number;
  enableJavaScript: boolean;
  jsPatterns: string[];
  jsMaxPages: number;
  jsWaitMs: number;
  jsTimeoutSec: number;
  jsViewportWidth: number;
  jsViewportHeight: number;
  jsConcurrency: number;
  enablePageSpeed: boolean;
  psiStrategy: string;
}

export const DEFAULT_OPTIONS: CrawlOptions = {
  mode: "spider",
  maxDepth: 3,
  maxURLs: 50000,
  crawlDelayMs: 0,
  followRedirects: true,
  // Screaming Frog's defaults: external links and page resources are checked.
  // Off, the External / CSS / JS / Images tabs can never fill — which read as
  // bugs, not settings.
  crawlExternal: true,
  crawlSubdomains: false,
  ignoreQueryParam: false,
  crawlImages: true,
  crawlCSS: true,
  crawlJS: true,
  userAgent: "sitecrawl",
  timeoutSec: 10,
  retries: 1,
  acceptLanguage: "",
  concurrency: 5,
  respectRobots: true,
  // Off, like Screaming Frog. A Crawl-delay spaces every request, so honouring
  // it collapses the crawl to one request at a time whatever the thread count —
  // measured at 1.1 URL/s against 65 on the same site.
  respectCrawlDelay: false,
  discoverSitemaps: true,
  ignoreSSL: false,
  // Off, like Screaming Frog: a crawl goes direct until the user asks for the
  // proxy. Having one saved in Connections for another tool must not silently
  // re-route every crawl through it.
  useProxy: false,
  customHeaders: {},
  includeExtensions: [],
  excludeExtensions: [],
  includePatterns: [],
  excludePatterns: [],
  maxFileSizeMB: 50,
  issueExclusions: [],
  useDefaultExcl: true,
  enableDuplication: true,
  duplicationThreshold: 0.85,
  enableJavaScript: false,
  jsPatterns: [],
  jsMaxPages: 500,
  jsWaitMs: 3000,
  jsTimeoutSec: 30,
  jsViewportWidth: 1920,
  jsViewportHeight: 1080,
  jsConcurrency: 3,
  // Off by default: a measurement takes ~20s on Google's side, so a 1000-page
  // site adds half an hour to a crawl that otherwise takes minutes. The
  // Configuration tab says so rather than letting the user find out.
  enablePageSpeed: false,
  psiStrategy: "mobile",
};

const STORAGE_KEY = "sitecrawl.options.v2";
const STORAGE_KEY_V1 = "sitecrawl.options.v1";

function load(): CrawlOptions {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return { ...DEFAULT_OPTIONS, ...(JSON.parse(raw) as Partial<CrawlOptions>) };
    const v1 = localStorage.getItem(STORAGE_KEY_V1);
    if (v1) {
      // v1 persisted crawlExternal:false as a default, not a choice; dropping
      // it here lets the corrected default (true) win for everyone who never
      // deliberately flipped it OFF after having it on — indistinguishable, so
      // the SF-parity default wins. checkImageStatus was a dead option.
      const parsed = JSON.parse(v1) as Partial<CrawlOptions> & { checkImageStatus?: boolean };
      delete parsed.crawlExternal;
      delete parsed.checkImageStatus;
      return { ...DEFAULT_OPTIONS, ...parsed };
    }
  } catch {
    // Corrupt storage falls back to defaults.
  }
  return { ...DEFAULT_OPTIONS };
}

let state: CrawlOptions = load();
const listeners = new Set<() => void>();

function emit() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // Losing persistence is fine; losing the session's edits is not.
  }
  for (const l of listeners) l();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useCrawlOptions(): CrawlOptions {
  return useSyncExternalStore(subscribe, () => state);
}

export function currentOptions(): CrawlOptions {
  return state;
}

export function updateOptions(patch: Partial<CrawlOptions>) {
  state = { ...state, ...patch };
  emit();
}

export function resetOptions() {
  state = { ...DEFAULT_OPTIONS };
  emit();
}
