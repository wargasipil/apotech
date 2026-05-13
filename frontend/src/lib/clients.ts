import { createPromiseClient } from "@connectrpc/connect";

import { HealthService } from "../gen/health_iface/v1/health_connect";
import { AuthService } from "../gen/user_iface/v1/auth_connect";
import { UserService } from "../gen/user_iface/v1/users_connect";
import { SupplierService } from "../gen/inventory_iface/v1/supplier_connect";
import { MedicineService } from "../gen/inventory_iface/v1/medicine_connect";
import { BatchService } from "../gen/inventory_iface/v1/batch_connect";
import { StockMovementService } from "../gen/inventory_iface/v1/stock_connect";
import { transport } from "./transport";

export const healthClient = createPromiseClient(HealthService, transport);
export const authClient = createPromiseClient(AuthService, transport);
export const userClient = createPromiseClient(UserService, transport);
export const supplierClient = createPromiseClient(SupplierService, transport);
export const medicineClient = createPromiseClient(MedicineService, transport);
export const batchClient = createPromiseClient(BatchService, transport);
export const stockClient = createPromiseClient(StockMovementService, transport);
