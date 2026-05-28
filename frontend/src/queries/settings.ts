import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { settingsClient } from "../lib/clients";

export const settingsKeys = {
  all: ["settings"] as const,
};

export function useSettingsQuery() {
  const q = useQuery({
    queryKey: settingsKeys.all,
    queryFn: async () => {
      const res = await settingsClient.getSettings({});
      return res.settings;
    },
    staleTime: 60_000,
  });
  return q;
}

export function useUpdateSettingsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { lowStockThreshold: number }) =>
      settingsClient.updateSettings(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKeys.all });
      // Threshold change → bell badge / dropdown re-evaluate.
      qc.invalidateQueries({ queryKey: ["lowStock"] });
    },
  });
}
