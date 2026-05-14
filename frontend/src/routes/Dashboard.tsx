import { Box, Grid, Heading, Stack, Text } from "@chakra-ui/react";
import { useTranslation } from "react-i18next";

import PageHeader from "../components/PageHeader";
import { Role } from "../gen/auth_iface/v1/policy_pb";
import { formatMoney } from "../lib/format";
import { useAuth } from "../lib/auth";
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
  const snapQ = useTodaySnapshotQuery();

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

      <Heading size="md" mb={3}>
        {t("pos.dashboardTitle")}
      </Heading>
      <Grid templateColumns={{ base: "1fr", md: "repeat(3, 1fr)" }} gap={4}>
        <Tile
          label={t("pos.dashboardRevenue")}
          value={formatMoney(Number(snapQ.data?.revenue ?? 0n))}
        />
        <Tile
          label={t("pos.dashboardSales")}
          value={String(snapQ.data?.saleCount ?? 0n)}
        />
        <Tile
          label={t("pos.dashboardItems")}
          value={String(snapQ.data?.itemsSold ?? 0n)}
        />
      </Grid>
    </Box>
  );
}

function Tile({ label, value }: { label: string; value: string }) {
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" p={5}>
      <Stack gap={1}>
        <Text fontSize="sm" color="fg.muted">
          {label}
        </Text>
        <Text fontSize="2xl" fontWeight="semibold" fontFamily="mono">
          {value}
        </Text>
      </Stack>
    </Box>
  );
}
