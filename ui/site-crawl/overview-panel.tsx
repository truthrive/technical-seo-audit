import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Search } from "lucide-react";

import { IssueInfo } from "@/../bindings/onescout/desktop/internal/tools/sitecrawl/models";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { TAB_GROUPS, TABS, issueTarget } from "@/features/site-crawl/columns";

// The right-hand tree — the FULL table of contents (the 6-group tab strip is
// only the compact form). Every row navigates: tab rows open the tab, filter
// rows open tab+filter, issue rows jump to the affected URLs. Counts come
// from Facets(), refreshed by the hook whenever the run's revision moves.
//
// Numbers are plain text, not Badge — a hundred coloured chips would drown
// the 7-hue system (design.md).
export function OverviewPanel({
  counts,
  issues,
  activeTab,
  activeFilter,
  onNavigate,
}: {
  counts: Record<string, number>;
  issues: IssueInfo[];
  activeTab: string;
  activeFilter: string;
  onNavigate: (tab: string, filter: string) => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [hideEmpty, setHideEmpty] = useState(true);

  const q = query.trim().toLowerCase();
  const matches = (label: string) => q === "" || label.toLowerCase().includes(q);

  const issueRows = useMemo(
    () =>
      [...issues]
        .map((i) => ({ ...i, count: counts[`issue.${i.code}`] ?? 0 }))
        .sort((a, b) => b.severity - a.severity || b.count - a.count),
    [issues, counts],
  );

  const row = (
    label: string,
    count: number | undefined,
    active: boolean,
    depth: number,
    onClick: () => void,
    tone?: string,
  ) => {
    if (hideEmpty && (count ?? 0) === 0 && depth > 0) return null;
    if (!matches(label)) return null;
    return (
      <button
        key={`${depth}:${label}`}
        type="button"
        onClick={onClick}
        className={cn(
          "flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-sm transition-colors",
          depth > 0 && "pl-6 text-[13px]",
          active ? "bg-primary/10 text-primary" : "hover:bg-muted",
        )}
      >
        {tone && <span className={cn("size-1.5 shrink-0 rounded-full", tone)} />}
        <span className="min-w-0 flex-1 truncate">{label}</span>
        {count !== undefined && (
          <span className="text-xs tabular-nums text-muted-foreground">{count}</span>
        )}
      </button>
    );
  };

  return (
    <div className="flex h-full min-h-0 flex-col gap-2 rounded-xl bg-card p-2 shadow-soft">
      <div className="flex items-center gap-2">
        <InputGroup className="min-w-0 flex-1">
          <InputGroupAddon>
            <Search />
          </InputGroupAddon>
          <InputGroupInput
            placeholder={t("siteCrawl.overviewSearch")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </InputGroup>
        <Switch
          size="default"
          checked={hideEmpty}
          onCheckedChange={setHideEmpty}
          aria-label={t("siteCrawl.hideEmpty")}
        />
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {TAB_GROUPS.map((group) => {
          const wiredTabs = group.tabs.filter((id) => TABS[id].wired && TABS[id].kind === "grid");
          if (wiredTabs.length === 0) return null;
          return (
            <div key={group.id} className="mb-2">
              <div className="px-2 py-1 text-xs font-medium text-muted-foreground uppercase">
                {t(`siteCrawl.group_${group.id}`, {
                  defaultValue: t(`siteCrawl.tab_${group.tabs[0]}`),
                })}
              </div>
              {wiredTabs.map((tabId) => (
                <div key={tabId}>
                  {row(
                    t(`siteCrawl.tab_${tabId}`),
                    counts[tabId],
                    activeTab === tabId && activeFilter === "all",
                    0,
                    () => onNavigate(tabId, "all"),
                  )}
                  {TABS[tabId].filters
                    .filter((f) => f !== "all")
                    .map((f) =>
                      row(
                        t(`siteCrawl.filter_${f}`),
                        counts[`${tabId}.${f}`],
                        activeTab === tabId && activeFilter === f,
                        1,
                        () => onNavigate(tabId, f),
                      ),
                    )}
                </div>
              ))}
            </div>
          );
        })}

        <div className="mb-2">
          <div className="px-2 py-1 text-xs font-medium text-muted-foreground uppercase">
            {t("siteCrawl.tab_issues")}
          </div>
          {issueRows.map((i) => {
            const target = issueTarget(i.code);
            return row(
              t(`siteCrawl.issue_${i.code}`, { defaultValue: i.code }),
              i.count,
              activeTab === target.tab && activeFilter === target.filter,
              1,
              () => onNavigate(target.tab, target.filter),
              i.severity >= 3 ? "bg-destructive" : i.severity === 2 ? "bg-warning" : "bg-primary",
            );
          })}
        </div>
      </div>
    </div>
  );
}
