import { useQuery } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import { analyticsClient } from "../lib/clients";
import { DEFAULT_PAGE_SIZE } from "../lib/pagination";
import type {
  DailyMetricRequest,
  Granularity,
  MetricType,
  ProductMetricRequest,
  Sort,
  UserMetricRequest,
} from "../gen/analytics_iface/v1/analytics_pb";

// Three thin hooks, one per dimension. Each returns the typed response shape
// directly so the page can pass `ids` / `order` / `stock` to <MetricTable> +
// <MetricGraphs> unchanged. Names are NOT in the metric payload — resolve via
// the existing Resolve<Domain>(ids) hooks (HARD RULE).

type Filter = { fromUnix: bigint; toUnix: bigint };

type DailyOpts = {
  metricTypes: MetricType[];
  filter: Filter;
  sort?: PartialMessage<Sort>;
  granularity?: Granularity;
};

export function useDailyMetricQuery(opts: DailyOpts) {
  const req: PartialMessage<DailyMetricRequest> = {
    metricTypes: opts.metricTypes,
    filter: opts.filter,
    sort: opts.sort,
    granularity: opts.granularity,
  };
  return useQuery({
    queryKey: ["analytics", "daily", req],
    queryFn: async () => {
      const res = await analyticsClient.dailyMetric(req);
      return {
        days: res.days,
        order: res.order,
        stock: res.stock,
      };
    },
    staleTime: 30_000,
  });
}

type ProductOpts = {
  metricTypes: MetricType[];
  filter: Filter;
  sort?: PartialMessage<Sort>;
  page?: number;
  pageSize?: number;
};

export function useProductMetricQuery(opts: ProductOpts) {
  const page = opts.page ?? 0;
  const pageSize = opts.pageSize ?? DEFAULT_PAGE_SIZE;
  const req: PartialMessage<ProductMetricRequest> = {
    metricTypes: opts.metricTypes,
    filter: opts.filter,
    sort: opts.sort,
    limit: pageSize,
    offset: page * pageSize,
  };
  return useQuery({
    queryKey: ["analytics", "product", req],
    queryFn: async () => {
      const res = await analyticsClient.productMetric(req);
      return {
        ids: res.medicineIds,
        order: res.order,
        stock: res.stock,
        total: res.total,
      };
    },
    staleTime: 30_000,
  });
}

type UserOpts = {
  metricTypes: MetricType[];
  filter: Filter;
  sort?: PartialMessage<Sort>;
  page?: number;
  pageSize?: number;
};

export function useUserMetricQuery(opts: UserOpts) {
  const page = opts.page ?? 0;
  const pageSize = opts.pageSize ?? DEFAULT_PAGE_SIZE;
  const req: PartialMessage<UserMetricRequest> = {
    metricTypes: opts.metricTypes,
    filter: opts.filter,
    sort: opts.sort,
    limit: pageSize,
    offset: page * pageSize,
  };
  return useQuery({
    queryKey: ["analytics", "user", req],
    queryFn: async () => {
      const res = await analyticsClient.userMetric(req);
      return {
        ids: res.userIds,
        order: res.order,
        total: res.total,
      };
    },
    staleTime: 30_000,
  });
}
