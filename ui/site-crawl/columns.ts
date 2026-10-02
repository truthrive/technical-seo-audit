// Single source of truth for the grid's shape: every tab Screaming Frog has,
// declared up front so the tab strip, the grid, export and the i18n key list
// all derive from one table. Phase 1 wires Internal / External / Response
// Codes; the rest are declared (the strip shows where the product is going)
// but disabled until their phase lands.
//
// Column ids must match Go's cellColumns / sortColumns whitelists in
// internal/tools/sitecrawl/query.go — an unknown id renders empty rather than
// erroring, which is what makes this list safe to drift ahead of Go.

export interface ColumnDef {
  id: string;
  /** Default width, px. Cells are px-3, so width = content + 24 (design.md). */
  width: number;
  minWidth?: number;
  /** Exactly one per tab. */
  grow?: boolean;
  align?: "left" | "right";
  sortable?: boolean;
}

export interface TabDef {
  id: string;
  /** grid | graph — Visualization replaces the grid area entirely (phase 3). */
  kind: "grid" | "graph";
  /** What one row is. Links/Images/Issues rows are not URLs, and the detail
   *  pane (phase 2) changes shape with this. */
  rowKind: "page" | "link" | "image" | "issue";
  /** Filter slugs understood by Go's filterClause for this tab. */
  filters: string[];
  columns: ColumnDef[];
  /** False until the phase that ships this tab wires it up. */
  wired: boolean;
}

export interface TabGroup {
  id: string;
  tabs: string[];
}

// --- shared column shapes ---

const url: ColumnDef = { id: "url", width: 320, minWidth: 200, grow: true, sortable: true };
const status: ColumnDef = { id: "status", width: 84, align: "right", sortable: true };
const contentType: ColumnDef = { id: "contentType", width: 130, sortable: true };
// Screaming Frog's split: "indexable" is the yes/no badge (rendered from row
// flags — Go has no cell for it, and its sort key sorts the boolean column),
// "indexability" is the reason ("canonicalised", "noindex"…) in its own
// column, readable without dragging anything wider.
const indexable: ColumnDef = { id: "indexable", width: 118, sortable: true };
const indexability: ColumnDef = { id: "indexability", width: 140, sortable: true };
const crawledAt: ColumnDef = { id: "crawledAt", width: 150, sortable: true };
const num = (id: string, width = 88): ColumnDef => ({ id, width, align: "right", sortable: true });

// --- the 17 tabs ---

export const TABS: Record<string, TabDef> = {
  internal: {
    id: "internal",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "html", "css", "js", "images", "pdf", "other", "indexable", "nonIndexable", "issues"],
    columns: [
      url,
      contentType,
      status,
      indexable,
      indexability,
      { id: "title", width: 220, sortable: true },
      num("titleLen", 76),
      { id: "metaDesc", width: 220, sortable: true },
      num("metaDescLen", 76),
      { id: "h1", width: 200, sortable: true },
      num("wordCount", 92),
      num("depth", 76),
      num("inlinks", 84),
      num("outlinks", 88),
      num("size", 96),
      num("responseMs", 96),
      crawledAt,
    ],
  },
  external: {
    id: "external",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "html", "css", "js", "images", "pdf", "other", "issues"],
    columns: [url, contentType, status, num("inlinks", 84), { id: "discoveredBy", width: 110 }, crawledAt],
  },
  response: {
    id: "response",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "success", "redirect", "clientError", "serverError", "noResponse", "blocked"],
    columns: [
      url,
      contentType,
      status,
      indexable,
      indexability,
      { id: "redirectTo", width: 260, sortable: true },
      num("redirectHops", 76),
      { id: "errorType", width: 140 },
      num("responseMs", 96),
      crawledAt,
    ],
  },
  titles: {
    id: "titles",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "missing", "duplicate", "long", "short", "multiple", "sameAsH1"],
    columns: [url, { id: "title", width: 300, sortable: true }, num("titleLen", 76), status, indexable],
  },
  meta: {
    id: "meta",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "missing", "duplicate", "long", "short", "multiple"],
    columns: [url, { id: "metaDesc", width: 320, sortable: true }, num("metaDescLen", 76), status, indexable],
  },
  h1: {
    id: "h1",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "missing", "duplicate", "multiple", "long"],
    columns: [url, { id: "h1", width: 300, sortable: true }, num("h1Len", 76), num("h1Count", 76), status],
  },
  h2: {
    id: "h2",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "missing", "multiple"],
    columns: [url, { id: "h2", width: 300, sortable: true }, num("h2Count", 76), status],
  },
  images: {
    id: "images",
    kind: "grid",
    rowKind: "image",
    wired: true,
    filters: ["all"],
    columns: [url, contentType, status, num("size", 96), num("inlinks", 84)],
  },
  canonicals: {
    id: "canonicals",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "missing", "self", "canonicalised"],
    columns: [url, { id: "canonical", width: 300, sortable: true }, indexable, indexability, status],
  },
  directives: {
    id: "directives",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "noindex", "nofollow", "refresh"],
    columns: [url, { id: "metaRobots", width: 180 }, { id: "xRobots", width: 140 }, indexable, indexability, status],
  },
  hreflang: {
    id: "hreflang",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "noReturn", "missingSelf", "invalidCode", "nonOK", "nonCanonical"],
    columns: [url, { id: "lang", width: 90, sortable: true }, status, indexable],
  },
  schema: {
    id: "schema",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "missing"],
    columns: [url, status, indexable, num("wordCount", 92)],
  },
  links: {
    id: "links",
    kind: "grid",
    rowKind: "link",
    wired: true,
    filters: ["all", "internal", "external", "nofollow"],
    // Positional: must match Go linkTabRows' fixed cell order
    // [source, target, anchor, placement, targetStatus]. Sort is server-side
    // crawl order, so no column is sortable here.
    columns: [
      { id: "source", width: 300, minWidth: 200, grow: true },
      { id: "target", width: 300 },
      { id: "anchor", width: 200 },
      { id: "placement", width: 100 },
      { id: "targetStatus", width: 96, align: "right" },
    ],
  },
  sitemaps: {
    id: "sitemaps",
    kind: "grid",
    rowKind: "page",
    wired: true,
    filters: ["all", "orphan", "nonIndexable"],
    columns: [url, status, indexable, indexability, num("inlinks", 84), crawledAt],
  },
  issues: {
    id: "issues",
    kind: "grid",
    rowKind: "issue",
    wired: true,
    filters: ["all", "critical", "warning", "notice"],
    columns: [
      { id: "code", width: 260, minWidth: 160, grow: true },
      { id: "category", width: 130 },
      { id: "severity", width: 100 },
      num("urls", 90),
      num("pct", 80),
    ],
  },
  // Both replace the grid area with their own panel — kind "graph" here means
  // "not a row window", and the page special-cases them by id.
  pagespeed: {
    id: "pagespeed",
    kind: "graph",
    rowKind: "page",
    wired: true,
    filters: [],
    columns: [],
  },
  // Like pagespeed and visualization: a report panel that replaces the grid
  // area entirely, so it declares no columns of its own.
  opportunities: {
    id: "opportunities",
    kind: "graph",
    rowKind: "page",
    wired: true,
    filters: [],
    columns: [],
  },
  visualization: {
    id: "visualization",
    kind: "graph",
    rowKind: "page",
    wired: true,
    filters: [],
    columns: [],
  },
};

// 17 flat tabs need ~1330px; the space between the sidebar and the Overview
// panel is 714–970px. Six group buttons fit — the dropdown carries the rest.
export const TAB_GROUPS: TabGroup[] = [
  { id: "crawl", tabs: ["internal", "external", "response"] },
  { id: "content", tabs: ["titles", "meta", "h1", "h2", "images"] },
  { id: "technical", tabs: ["canonicals", "directives", "hreflang", "schema"] },
  { id: "links", tabs: ["links"] },
  { id: "sitemaps", tabs: ["sitemaps"] },
  { id: "reports", tabs: ["issues", "pagespeed", "opportunities", "visualization"] },
];

export const DEFAULT_TAB = "internal";

export function tabDef(id: string): TabDef {
  return TABS[id] ?? TABS[DEFAULT_TAB];
}

// Where activating an issue lands (Issues tab rows, Overview tree, the
// detail pane's "Go to affected URLs"). Codes with a named filter get the
// nicer chip; everything else uses the generic issue:<code> filter, which Go
// resolves on any page tab — so every code has a working destination.
const ISSUE_TARGETS: Record<string, [string, string]> = {
  "title-missing": ["titles", "missing"],
  "title-long": ["titles", "long"],
  "title-short": ["titles", "short"],
  "title-multiple": ["titles", "multiple"],
  "title-duplicate": ["titles", "duplicate"],
  "title-same-as-h1": ["titles", "sameAsH1"],
  "meta-missing": ["meta", "missing"],
  "meta-long": ["meta", "long"],
  "meta-short": ["meta", "short"],
  "meta-multiple": ["meta", "multiple"],
  "meta-duplicate": ["meta", "duplicate"],
  "h1-missing": ["h1", "missing"],
  "h1-multiple": ["h1", "multiple"],
  "h1-long": ["h1", "long"],
  "h1-duplicate": ["h1", "duplicate"],
  "h2-missing": ["h2", "missing"],
  "client-error": ["response", "clientError"],
  "server-error": ["response", "serverError"],
  "canonical-missing": ["canonicals", "missing"],
  noindex: ["directives", "noindex"],
  nofollow: ["directives", "nofollow"],
  "meta-refresh": ["directives", "refresh"],
  "robots-blocked": ["response", "blocked"],
  "orphan-page": ["sitemaps", "orphan"],
  "hreflang-no-return-tag": ["hreflang", "noReturn"],
  "hreflang-missing-self": ["hreflang", "missingSelf"],
  "hreflang-invalid-code": ["hreflang", "invalidCode"],
  "hreflang-to-non-200": ["hreflang", "nonOK"],
  "hreflang-non-canonical": ["hreflang", "nonCanonical"],
  "no-structured-data": ["schema", "missing"],
};

export function issueTarget(code: string): { tab: string; filter: string } {
  const direct = ISSUE_TARGETS[code];
  if (direct) return { tab: direct[0], filter: direct[1] };
  return { tab: "internal", filter: `issue:${code}` };
}
