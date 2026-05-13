import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";

import { toast } from "./toaster";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      retry: 1,
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
