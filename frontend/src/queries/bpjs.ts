import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { bpjsClaimClient } from "../lib/clients";
import type {
  CreateClaimRequest,
  ListClaimsRequest,
  ResolveClaimRequest,
} from "../gen/bpjs_iface/v1/bpjs_pb";

export const bpjsKeys = {
  all: ["bpjs"] as const,
  list: (filters: object) => [...bpjsKeys.all, "list", filters] as const,
};

export function useBpjsClaimsQuery(req: PartialMessage<ListClaimsRequest> = {}) {
  return useQuery({
    queryKey: bpjsKeys.list(req),
    queryFn: async () => {
      const res = await bpjsClaimClient.listClaims(req);
      return res.claims;
    },
  });
}

export function useCreateBpjsClaimMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<CreateClaimRequest>) => bpjsClaimClient.createClaim(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: bpjsKeys.all }),
  });
}

export function useSubmitBpjsClaimMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => bpjsClaimClient.submitClaim({ id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: bpjsKeys.all }),
  });
}

export function useResolveBpjsClaimMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: PartialMessage<ResolveClaimRequest>) => bpjsClaimClient.resolveClaim(req),
    onSuccess: () => qc.invalidateQueries({ queryKey: bpjsKeys.all }),
  });
}
