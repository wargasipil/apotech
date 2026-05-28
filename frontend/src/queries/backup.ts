import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { backupClient } from "../lib/clients";

export const backupKeys = {
  all: ["backups"] as const,
};

// Refetches every minute so a fresh backup created via the button shows up
// promptly even without manual interaction.
export function useBackupsQuery() {
  return useQuery({
    queryKey: backupKeys.all,
    queryFn: async () => {
      const res = await backupClient.listBackups({});
      return res.backups;
    },
    staleTime: 60_000,
    refetchInterval: 60_000,
  });
}

export function useCreateBackupMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => backupClient.createBackup({}),
    onSuccess: () => qc.invalidateQueries({ queryKey: backupKeys.all }),
  });
}

export function useDeleteBackupMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => backupClient.deleteBackup({ name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: backupKeys.all }),
  });
}
