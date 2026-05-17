import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { branchClient } from "../lib/clients";
import type {
  CreateBranchRequest,
  GrantBranchAccessRequest,
  ListBranchesRequest,
  SetDefaultBranchRequest,
  UpdateBranchRequest,
} from "../gen/branch_iface/v1/branch_pb";

export const branchKeys = {
  all: ["branches"] as const,
  list: (filters: object) => [...branchKeys.all, "list", filters] as const,
  user: (userId: string) => [...branchKeys.all, "user", userId] as const,
};

export function useBranchesQuery(req: PartialMessage<ListBranchesRequest> = {}) {
  return useQuery({
    queryKey: branchKeys.list(req),
    queryFn: async () => {
      const res = await branchClient.listBranches(req);
      return res.branches;
    },
  });
}

export function useMyBranchesQuery() {
  return useQuery({
    queryKey: branchKeys.user("self"),
    queryFn: async () => branchClient.listUserBranches({ userId: "" }),
  });
}

export function useCreateBranchMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateBranchRequest>) => branchClient.createBranch(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: branchKeys.all }),
  });
}

export function useUpdateBranchMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<UpdateBranchRequest>) => branchClient.updateBranch(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: branchKeys.all }),
  });
}

export function useGrantBranchAccessMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<GrantBranchAccessRequest>) =>
      branchClient.grantBranchAccess(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: branchKeys.all }),
  });
}

export function useSetDefaultBranchMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<SetDefaultBranchRequest>) =>
      branchClient.setDefaultBranch(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: branchKeys.all }),
  });
}
