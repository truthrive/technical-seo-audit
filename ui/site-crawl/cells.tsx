import type { TFunction } from "i18next";

import { Badge } from "@/components/ui/badge";
import type { GridRowData } from "@/components/ui/data-grid";

// Row flag bits — mirrors Go's Row flag consts (types.go). Kept out of Cells
// so badge colour decisions stay typed and every display string stays here.
export const ROW_INTERNAL = 1 << 0;
export const ROW_INDEXABLE = 1 << 1;
export const ROW_HAS_ISSUES = 1 << 2;
export const ROW_RENDERED = 1 << 3;
export const ROW_ORPHAN = 1 << 4;

// 429 is amber, not red: the site is throttling us, which says nothing about
// the page (same reading as index-check's results panel).
function statusVariant(status: number): "success" | "warning" | "destructive" {
  if (status >= 200 && status < 300) return "success";
  if ((status >= 300 && status < 400) || status === 429) return "warning";
  return "destructive";
}

function formatBytes(raw: string): string {
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return "";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
}

function formatTime(raw: string): string {
  if (!raw) return "";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  // "en-US": the machine locale turned this cell Vietnamese on a Vietnamese Mac.
  // hour12: false to match the rest of the app — without it this cell printed
  // "09:43 PM", which is neither clock.
  return d.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

/**
 * One renderer for every column id, built once per language. Cells arrive as
 * positional strings straight from SQL; every human-facing transformation
 * (units, labels, badge tone) happens here and only here — Go never emits a
 * display string (design.md §8).
 */
export function makeRenderCell(t: TFunction) {
  return function renderCell(row: GridRowData, colId: string, colIndex: number) {
    const value = row.cells[colIndex] ?? "";

    switch (colId) {
      case "status": {
        if (row.status === 0) {
          // Never fetched successfully — the error class is the story.
          return (
            <Badge variant="destructive" className="tabular-nums">
              {t("siteCrawl.statusNone")}
            </Badge>
          );
        }
        return (
          <Badge variant={statusVariant(row.status)} className="tabular-nums">
            {row.status}
          </Badge>
        );
      }

      // Two columns, Screaming Frog's split: the badge answers yes/no, the
      // status column carries the reason in full. Cramming the reason next to
      // the badge truncated it invisible at the default width — a value the
      // user can only see by dragging a column wider may as well not exist.
      case "indexable":
        return row.flags & ROW_INDEXABLE ? (
          <Badge variant="success">{t("siteCrawl.indexable")}</Badge>
        ) : (
          // Deliberately neutral — canonicalised/noindex is usually intentional.
          <Badge variant="secondary">{t("siteCrawl.nonIndexable")}</Badge>
        );

      case "indexability":
        return value ? (
          <span className="truncate">{t(`siteCrawl.reason_${value}`, { defaultValue: value })}</span>
        ) : null;

      case "errorType":
        return value ? (
          <Badge variant="destructive">{t(`siteCrawl.err_${value}`, { defaultValue: value })}</Badge>
        ) : null;

      case "discoveredBy":
        return value ? (
          <span className="text-xs text-muted-foreground">
            {t(`siteCrawl.source_${value}`, { defaultValue: value })}
          </span>
        ) : null;

      case "size":
        return <span className="truncate">{formatBytes(value)}</span>;

      case "responseMs":
        return value && value !== "0" ? <span className="truncate">{value} ms</span> : null;

      case "crawledAt":
        return <span className="truncate text-muted-foreground">{formatTime(value)}</span>;

      case "contentType":
        // Lowercase text, no badge: 50k coloured chips in a dense grid is the
        // exact noise design.md's 7-hue table warns about.
        return <span className="truncate text-muted-foreground">{value}</span>;

      case "url":
      case "source":
      case "target":
        return <span className="block w-full truncate">{value}</span>;

      // --- Links tab ---
      case "placement":
        return (
          <span className="text-xs text-muted-foreground">
            {t(`siteCrawl.placement_${value}`, { defaultValue: value })}
          </span>
        );
      case "targetStatus": {
        const st = Number(value);
        if (!st) return <Badge variant="destructive">{t("siteCrawl.statusNone")}</Badge>;
        return (
          <Badge variant={statusVariant(st)} className="tabular-nums">
            {st}
          </Badge>
        );
      }

      // --- Issues tab ---
      case "code":
        return (
          <span className="block w-full truncate">
            {t(`siteCrawl.issue_${value}`, { defaultValue: value })}
          </span>
        );
      case "category":
        return (
          <span className="truncate text-muted-foreground">
            {t(`siteCrawl.cat_${value}`, { defaultValue: value })}
          </span>
        );
      case "severity": {
        const sev = Number(value);
        const variant = sev >= 3 ? "destructive" : sev === 2 ? "warning" : "primary";
        const label =
          sev >= 3 ? t("siteCrawl.sevCritical") : sev === 2 ? t("siteCrawl.sevWarning") : t("siteCrawl.sevNotice");
        return <Badge variant={variant}>{label}</Badge>;
      }
      case "pct":
        return <span className="truncate">{value}%</span>;

      default:
        return <span className="truncate">{value}</span>;
    }
  };
}
