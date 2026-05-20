import { Tabs } from "@chakra-ui/react";
import { NavLink, useLocation } from "react-router-dom";

// URL-driven tabs that look and behave like Chakra v3's `Tabs.Root` (default
// `variant="line"` — underline on active). Each tab is a `react-router-dom`
// route under an `<Outlet/>` rendered by the parent page; clicking a tab
// updates the URL via `NavLink`, the active state is derived from
// `useLocation()`, and there's no internal state machine to keep in sync.
//
// Use this for page-level tab navigation where each panel has its own URL
// (Analytics, Inventory, Purchasing). For state-driven tabs that share one
// route (Tax: Issued invoices / NSFP pool), use Chakra `Tabs.Root` directly.
// Codified in CLAUDE.md → Frontend conventions → Tabs.
export type RouteTabItem = {
  /** Stable identifier for Chakra Tabs internals. Usually the last URL segment. */
  value: string;
  /** Already-localized label (caller passes `t("...")` result). */
  label: string;
  /** Absolute path the tab navigates to. */
  to: string;
};

export type RouteTabsProps = {
  items: RouteTabItem[];
};

export default function RouteTabs({ items }: RouteTabsProps) {
  const location = useLocation();

  // Pick the tab whose `to` is the LONGEST prefix of the current pathname.
  // This avoids the wrong tab lighting up when one tab's path is itself a
  // prefix of another's (or of an unrelated sub-route). Falls back to the
  // first item if nothing matches.
  const activeValue =
    items
      .filter((it) => location.pathname.startsWith(it.to))
      .sort((a, b) => b.to.length - a.to.length)[0]?.value ?? items[0]?.value;

  return (
    <Tabs.Root value={activeValue} variant="line">
      <Tabs.List>
        {items.map((it) => (
          <Tabs.Trigger key={it.value} value={it.value} asChild>
            <NavLink to={it.to}>{it.label}</NavLink>
          </Tabs.Trigger>
        ))}
      </Tabs.List>
    </Tabs.Root>
  );
}
