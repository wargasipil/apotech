import { Box, Button, Code, Stack, Text } from "@chakra-ui/react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import PageHeader from "../components/PageHeader";
import { Role } from "../gen/auth_iface/v1/policy_pb";
import { healthClient } from "../lib/clients";
import { useAuth } from "../lib/auth";
import { toast } from "../lib/toaster";

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

  const ping = useMutation({
    mutationFn: () => healthClient.ping({}),
    onError: (err) => toast.fromError(err),
    meta: { silentError: true },
  });

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
        actions={
          <Button colorPalette="blue" onClick={() => ping.mutate()} loading={ping.isPending}>
            {t("dashboard.pingApi")}
          </Button>
        }
      />
      <Stack gap={4} maxW="lg">
        {ping.data && (
          <Code as="pre" p={3} whiteSpace="pre-wrap" bg="bg.muted">
            {JSON.stringify({ status: ping.data.status, db: ping.data.db }, null, 2)}
          </Code>
        )}
        {!ping.data && (
          <Text color="fg.muted" fontSize="sm">
            Phase 2 inventory is live · POS phase next.
          </Text>
        )}
      </Stack>
    </Box>
  );
}
