import { Box, Button, HStack, Input, Spinner, Stack, Text } from "@chakra-ui/react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { toast } from "../../lib/toaster";
import { useSettingsQuery, useUpdateSettingsMutation } from "../../queries/settings";

export default function SettingsGeneral() {
  const { t } = useTranslation();
  const q = useSettingsQuery();
  const save = useUpdateSettingsMutation();

  const [threshold, setThreshold] = useState<string>("");
  useEffect(() => {
    if (q.data) setThreshold(String(q.data.lowStockThreshold));
  }, [q.data]);

  const onSave = async () => {
    const n = Number(threshold);
    if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0) {
      toast.error(t("settings.invalidThreshold"));
      return;
    }
    try {
      await save.mutateAsync({ lowStockThreshold: n });
      toast.success(t("common.save") + " ✓");
    } catch {
      /* toast handled globally */
    }
  };

  if (q.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }

  return (
    <Stack gap={4} maxW="md">
      <Stack gap={1}>
        <Text fontSize="sm" fontWeight="medium">
          {t("settings.lowStockThreshold")}
        </Text>
        <HStack gap={2}>
          <Input
            type="number"
            inputMode="numeric"
            min={0}
            width="120px"
            value={threshold}
            onChange={(e) => setThreshold(e.target.value)}
          />
          <Button
            colorPalette="blue"
            onClick={onSave}
            loading={save.isPending}
          >
            {t("common.save")}
          </Button>
        </HStack>
        <Text fontSize="xs" color="fg.muted">
          {t("settings.lowStockHelp")}
        </Text>
      </Stack>
    </Stack>
  );
}
