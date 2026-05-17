import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";

import { toast } from "./toaster";

// Proto int64 fields land in queryKeys as BigInt; JSON.stringify can't
// serialize them, which breaks TanStack Query's default hashing. Stringify
// BigInts explicitly here so every query key (analytics date ranges, IDs,
// etc.) hashes deterministically.
function hashWithBigInt(queryKey: readonly unknown[]): string {
  return JSON.stringify(queryKey, (_key, value) =>
    typeof value === "bigint" ? value.toString() : value,
  );
}

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      retry: 1,
      queryKeyHashFn: hashWithBigInt,
    },
    mutations: {
      retry: 0,
    },
  },
  queryCache: new QueryCache({
    onError: (err, query) => {
      // Only auto-toast on queries that haven't opted out via meta.
      if (query.meta?.silentError !== true) {
        toast.fromError(err);
      }
    },
  }),
  mutationCache: new MutationCache({
    onError: (err, _vars, _ctx, mutation) => {
      if (mutation.meta?.silentError !== true) {
        toast.fromError(err);
      }
    },
  }),
});

declare module "@tanstack/react-query" {
  interface Register {
    queryMeta: { silentError?: boolean };
    mutationMeta: { silentError?: boolean };
  }
}
