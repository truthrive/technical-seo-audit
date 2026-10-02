import { useTranslation } from "react-i18next";
import { ChevronDown } from "lucide-react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { TAB_GROUPS, TABS } from "@/features/site-crawl/columns";

// 17 flat tabs need ~1330px and the strip has 714–970px, so tabs live in six
// groups with dropdowns. NOT ui/tabs.tsx: Base UI unmounts panels on switch,
// which would tear down the scroll container, the virtualizer's measurement
// cache and the DOM-held column widths every time. This is a plain tablist of
// buttons — one grid instance lives for the page's whole life, and a tab
// switch only changes its column set and backend filter.

const tabButton =
  "flex h-8 shrink-0 items-center gap-1 whitespace-nowrap rounded-full px-3 text-sm transition-colors";
const tabIdle = "text-foreground/60 hover:bg-accent hover:text-foreground";
const tabActive = "bg-card font-medium text-foreground shadow-soft";

export function TabStrip({
  active,
  onSelect,
}: {
  active: string;
  onSelect: (tabId: string) => void;
}) {
  const { t } = useTranslation();

  return (
    <div
      role="tablist"
      aria-label={t("siteCrawl.tabsLabel")}
      className="no-scrollbar flex h-8 shrink-0 items-center gap-1 overflow-x-auto"
    >
      {TAB_GROUPS.map((group) => {
        const activeChild = group.tabs.includes(active) ? active : null;

        if (group.tabs.length === 1) {
          const tab = TABS[group.tabs[0]];
          return (
            <button
              key={group.id}
              type="button"
              role="tab"
              aria-selected={activeChild !== null}
              disabled={!tab.wired}
              onClick={() => onSelect(tab.id)}
              className={cn(
                tabButton,
                activeChild ? tabActive : tabIdle,
                !tab.wired && "cursor-default opacity-50 hover:bg-transparent",
              )}
            >
              {t(`siteCrawl.tab_${tab.id}`)}
            </button>
          );
        }

        return (
          <DropdownMenu key={group.id}>
            <DropdownMenuTrigger
              role="tab"
              aria-selected={activeChild !== null}
              className={cn(tabButton, activeChild ? tabActive : tabIdle)}
            >
              {/* The group button wears the active child's name, so the strip
                  always says which tab is on screen. */}
              {activeChild ? t(`siteCrawl.tab_${activeChild}`) : t(`siteCrawl.group_${group.id}`)}
              <ChevronDown className="size-3.5 opacity-60" />
            </DropdownMenuTrigger>
            {/* w-auto: the popup defaults to the anchor width and clips text. */}
            <DropdownMenuContent align="start" className="w-auto">
              {group.tabs.map((tabId) => {
                const tab = TABS[tabId];
                return (
                  <DropdownMenuItem
                    key={tabId}
                    disabled={!tab.wired}
                    onClick={() => onSelect(tabId)}
                    className={cn(
                      "whitespace-nowrap",
                      tabId === active && "bg-primary/10 text-primary",
                    )}
                  >
                    {t(`siteCrawl.tab_${tabId}`)}
                    {!tab.wired && (
                      <span className="ml-auto pl-3 text-xs text-muted-foreground">
                        {t("siteCrawl.soon")}
                      </span>
                    )}
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuContent>
          </DropdownMenu>
        );
      })}
    </div>
  );
}
