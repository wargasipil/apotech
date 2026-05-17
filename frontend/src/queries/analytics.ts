import { useQuery } from "@tanstack/react-query";
import type { PartialMessage } from "@bufbuild/protobuf";

import {
  inventoryAnalyticsClient,
  marginAnalyticsClient,
  salesAnalyticsClient,
} from "../lib/clients";
import type {
  GetRevenueTrendRequest,
  GetTopSellersRequest,
  GetPaymentMixRequest,
  GetSalesByCashierRequest,
  GetHourOfDayHeatmapRequest,
} from "../gen/analytics_iface/v1/sales_pb";
import type {
  GetTurnoverRequest,
  GetDeadStockRequest,
  GetDaysOfStockRemainingRequest,
} from "../gen/analytics_iface/v1/inventory_pb";
import type {
  GetMarginPerMedicineRequest,
  GetTopMarginRequest,
  GetSupplierCostTrendRequest,
} from "../gen/analytics_iface/v1/margin_pb";

export const analyticsKeys = {
  all: ["analytics"] as const,
  sales: (k: string, p: object) => [...analyticsKeys.all, "sales", k, p] as const,
  inventory: (k: string, p: object) => [...analyticsKeys.all, "inventory", k, p] as const,
  margin: (k: string, p: object) => [...analyticsKeys.all, "margin", k, p] as const,
};

// ---------- Sales ----------
export function useRevenueTrendQuery(req: PartialMessage<GetRevenueTrendRequest>) {
  return useQuery({
    queryKey: analyticsKeys.sales("revenueTrend", req),
    queryFn: () => salesAnalyticsClient.getRevenueTrend(req),
  });
}
export function useTopSellersQuery(req: PartialMessage<GetTopSellersRequest>) {
  return useQuery({
    queryKey: analyticsKeys.sales("topSellers", req),
    queryFn: () => salesAnalyticsClient.getTopSellers(req),
  });
}
export function usePaymentMixQuery(req: PartialMessage<GetPaymentMixRequest>) {
  return useQuery({
    queryKey: analyticsKeys.sales("paymentMix", req),
    queryFn: () => salesAnalyticsClient.getPaymentMix(req),
  });
}
export function useSalesByCashierQuery(req: PartialMessage<GetSalesByCashierRequest>) {
  return useQuery({
    queryKey: analyticsKeys.sales("byCashier", req),
    queryFn: () => salesAnalyticsClient.getSalesByCashier(req),
  });
}
export function useHourOfDayQuery(req: PartialMessage<GetHourOfDayHeatmapRequest>) {
  return useQuery({
    queryKey: analyticsKeys.sales("hourHeatmap", req),
    queryFn: () => salesAnalyticsClient.getHourOfDayHeatmap(req),
  });
}

// ---------- Inventory ----------
export function useTurnoverQuery(req: PartialMessage<GetTurnoverRequest>) {
  return useQuery({
    queryKey: analyticsKeys.inventory("turnover", req),
    queryFn: () => inventoryAnalyticsClient.getTurnover(req),
  });
}
export function useDeadStockQuery(req: PartialMessage<GetDeadStockRequest>) {
  return useQuery({
    queryKey: analyticsKeys.inventory("deadStock", req),
    queryFn: () => inventoryAnalyticsClient.getDeadStock(req),
  });
}
export function useDaysOfStockQuery(req: PartialMessage<GetDaysOfStockRemainingRequest>) {
  return useQuery({
    queryKey: analyticsKeys.inventory("daysRemaining", req),
    queryFn: () => inventoryAnalyticsClient.getDaysOfStockRemaining(req),
  });
}
export function useExpiryRiskQuery() {
  return useQuery({
    queryKey: analyticsKeys.inventory("expiryRisk", {}),
    queryFn: () => inventoryAnalyticsClient.getExpiryRiskForecast({}),
  });
}

// ---------- Margin ----------
export function useMarginPerMedicineQuery(req: PartialMessage<GetMarginPerMedicineRequest>) {
  return useQuery({
    queryKey: analyticsKeys.margin("perMedicine", req),
    queryFn: () => marginAnalyticsClient.getMarginPerMedicine(req),
  });
}
export function useTopMarginQuery(req: PartialMessage<GetTopMarginRequest>) {
  return useQuery({
    queryKey: analyticsKeys.margin("topMargin", req),
    queryFn: () => marginAnalyticsClient.getTopMargin(req),
  });
}
export function useSupplierCostTrendQuery(
  req: PartialMessage<GetSupplierCostTrendRequest>,
  enabled = true,
) {
  return useQuery({
    queryKey: analyticsKeys.margin("supplierCost", req),
    queryFn: () => marginAnalyticsClient.getSupplierCostTrend(req),
    enabled: enabled && !!req.supplierId,
  });
}
