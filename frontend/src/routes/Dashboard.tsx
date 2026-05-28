import { Box, Grid, Heading, Stack } from "@chakra-ui/react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import DashboardTile from "../components/DashboardTile";
import PageHeader from "../components/PageHeader";
import { Role } from "../gen/auth_iface/v1/policy_pb";
import {
  Granularity,
  MetricType,
} from "../gen/analytics_iface/v1/analytics_pb";
import { useAuth } from "../lib/auth";
import { formatMoney, formatUnix } from "../lib/format";
import { useDailyMetricQuery } from "../queries/analytics";
import { useExpiringSoonCountQuery } from "../queries/batches";
import { useLowStockQuery } from "../queries/medicines";
import { useActiveRxCountQuery } from "../queries/prescriptions";
import { useTodaySnapshotQuery } from "../queries/sales";

function roleKey(role: Role): string {
  switch (role) {
    case Role.OWNER:
      return "owner";
    case Role.PHARMACIST:
      return "pharmacist";
    case Role.CASHIER:
      return "cashier";
    default:
      return "unknown";
  }
}

export default function Dashboard() {
  const { t } = useTranslation();
  const { user } = useAuth();

  return (
    <Box>
      <PageHeader
        title={`${t("dashboard.welcome")}${user?.name ? `, ${user.name}` : ""}`}
        description={
          user
            ? `${t("dashboard.signedInAs")} ${user.email} (${t(
                `dashboard.roles.${roleKey(user.role)}`,
              )})`
            : undefined
        }
      />
      {user?.role === Role.OWNER && <OwnerHealth />}
      {user?.role === Role.PHARMACIST && <InventoryHealth />}
      {user?.role === Role.CASHIER && <MyShift cashierUserId={user.id} />}
    </Box>
  );
}

// ---------- OWNER: business health ----------

function OwnerHealth() {
  const { t } = useTranslation();
  const snapQ = useTodaySnapshotQuery();
  const lowStockQ = useLowStockQuery();
  const expiringQ = useExpiringSoonCountQuery(30);

  // Today's profit comes from the new Analytics RPC: ask for a single-day
  // window at DAY granularity, read the first row's `profit`. Reuses the
  // already-warehouse-scoped DailyMetric handler.
  const todayBounds = useMemo(() => {
    const now = new Date();
    const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0, 0);
    const end = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, 0, 0, 0, 0);
    return {
      fromUnix: BigInt(Math.floor(start.getTime() / 1000)),
      toUnix: BigInt(Math.floor(end.getTime() / 1000)),
    };
  }, []);
  const todayMetricQ = useDailyMetricQuery({
    metricTypes: [MetricType.ORDER],
    filter: todayBounds,
    granularity: Granularity.DAY,
  });
  const todayKey = new Date().toISOString().slice(0, 10);
  const profitToday = Number(todayMetricQ.data?.order?.data[todayKey]?.profit ?? 0n);

  // 7-day revenue mini-trend (excluding today is OK; include for completeness).
  const last7Bounds = useMemo(() => {
    const now = new Date();
    const end = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, 0, 0, 0, 0);
    const start = new Date(end.getTime() - 7 * 86400 * 1000);
    return {
      fromUnix: BigInt(Math.floor(start.getTime() / 1000)),
      toUnix: BigInt(Math.floor(end.getTime() / 1000)),
    };
  }, []);
  const trendQ = useDailyMetricQuery({
    metricTypes: [MetricType.ORDER],
    filter: last7Bounds,
    granularity: Granularity.DAY,
  });
  const trendData = (trendQ.data?.days ?? []).map((d) => ({
    day: d,
    revenue: Number(trendQ.data?.order?.data[d]?.terjual ?? 0n),
  }));

  const revenue = Number(snapQ.data?.revenue ?? 0n);
  const saleCount = Number(snapQ.data?.saleCount ?? 0n);
  const itemsSold = Number(snapQ.data?.itemsSold ?? 0n);
  const avgBasket = saleCount > 0 ? Math.round(revenue / saleCount) : 0;

  const lowStockCount = lowStockQ.data?.total ?? 0;
  const expiringCount = expiringQ.count;

  return (
    <Stack gap={5}>
      <Heading size="md">{t("dashboard.section.ownerHealth")}</Heading>
      <Grid templateColumns={{ base: "1fr", md: "repeat(3, 1fr)" }} gap={4}>
        <DashboardTile label={t("dashboard.tile.revenueToday")} value={formatMoney(revenue)} to="/orders" />
        <DashboardTile label={t("dashboard.tile.profitToday")} value={formatMoney(profitToday)} to="/analytics/daily" />
        <DashboardTile label={t("dashboard.tile.salesToday")} value={String(saleCount)} to="/orders" />
        <DashboardTile label={t("dashboard.tile.itemsToday")} value={String(itemsSold)} to="/orders" />
        <DashboardTile label={t("dashboard.tile.avgBasket")} value={formatMoney(avgBasket)} to="/orders" />
        <DashboardTile
          label={t("dashboard.tile.lowStock")}
          value={String(lowStockCount)}
          to="/medicines"
          tone={lowStockCount > 0 ? "danger" : "default"}
        />
        <DashboardTile
          label={t("dashboard.tile.expiring30d")}
          value={String(expiringCount)}
          to="/inventory/batches"
          tone={expiringCount > 0 ? "warning" : "default"}
        />
      </Grid>
      <Box>
        <Heading size="sm" mb={3} color="fg.muted">
          {t("dashboard.trend.last7d")}
        </Heading>
        <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={4} h="240px">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={trendData}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--chakra-colors-border)" />
              <XAxis dataKey="day" fontSize={11} />
              <YAxis fontSize={11} tickFormatter={(v) => formatMoney(v).replace("Rp", "")} />
              <Tooltip formatter={(v: number) => formatMoney(v)} />
              <Line type="monotone" dataKey="revenue" stroke="#3B82F6" strokeWidth={2} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </Box>
      </Box>
    </Stack>
  );
}

// ---------- PHARMACIST: inventory health ----------

function InventoryHealth() {
  const { t } = useTranslation();
  const lowStockQ = useLowStockQuery();
  const expiringQ = useExpiringSoonCountQuery(30);
  const rxQ = useActiveRxCountQuery();

  const lowStockCount = lowStockQ.data?.total ?? 0;

  return (
    <Stack gap={5}>
      <Heading size="md">{t("dashboard.section.inventoryHealth")}</Heading>
      <Grid templateColumns={{ base: "1fr", md: "repeat(3, 1fr)" }} gap={4}>
        <DashboardTile
          label={t("dashboard.tile.lowStock")}
          value={String(lowStockCount)}
          to="/medicines"
          tone={lowStockCount > 0 ? "danger" : "default"}
        />
        <DashboardTile
          label={t("dashboard.tile.expiring30d")}
          value={String(expiringQ.count)}
          to="/inventory/batches"
          tone={expiringQ.count > 0 ? "warning" : "default"}
        />
        <DashboardTile
          label={t("dashboard.tile.activeRx")}
          value={String(rxQ.count)}
          to="/prescriptions"
        />
      </Grid>
    </Stack>
  );
}

// ---------- CASHIER: my shift ----------

function MyShift({ cashierUserId }: { cashierUserId: string }) {
  const { t } = useTranslation();
  const snapQ = useTodaySnapshotQuery({ cashierUserId });

  const revenue = Number(snapQ.data?.revenue ?? 0n);
  const saleCount = Number(snapQ.data?.saleCount ?? 0n);
  const itemsSold = Number(snapQ.data?.itemsSold ?? 0n);
  const lastUnix = Number(snapQ.data?.lastSaleUnix ?? 0n);

  return (
    <Stack gap={5}>
      <Heading size="md">{t("dashboard.section.myShift")}</Heading>
      <Grid templateColumns={{ base: "1fr", md: "repeat(2, 1fr)", lg: "repeat(4, 1fr)" }} gap={4}>
        <DashboardTile label={t("dashboard.tile.myRevenue")} value={formatMoney(revenue)} to="/orders" />
        <DashboardTile label={t("dashboard.tile.mySales")} value={String(saleCount)} to="/orders" />
        <DashboardTile label={t("dashboard.tile.myItems")} value={String(itemsSold)} to="/orders" />
        <DashboardTile
          label={t("dashboard.tile.myLastSale")}
          value={lastUnix > 0 ? formatUnix(lastUnix) : t("dashboard.tile.lastSaleNone")}
          to="/orders"
        />
      </Grid>
    </Stack>
  );
}
