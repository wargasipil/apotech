import { useState } from "react";
import {
  Badge,
  Box,
  Button,
  Heading,
  HStack,
  SimpleGrid,
  Spinner,
  Stack,
  Table,
  Tabs,
  Text,
} from "@chakra-ui/react";
import { Archive, Pencil } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";

import BackButton from "../../components/BackButton";
import ExpiryBadge from "../../components/ExpiryBadge";
import PageHeader from "../../components/PageHeader";
import { MovementType } from "../../gen/inventory_iface/v1/stock_pb";
import { formatMoney, formatUnix } from "../../lib/format";
import { marginPct } from "../../lib/pricing";
import { ALL_LIMIT } from "../../lib/pagination";
import { toast } from "../../lib/toaster";
import { useBatchesQuery } from "../../queries/batches";
import {
  useArchiveMedicineMutation,
  useMedicineUnitPricesQuery,
  useMedicineQuery,
} from "../../queries/medicines";
import { useMovementsQuery } from "../../queries/stock";
import { EditMedicineDialog } from "./medicineDrawers";

function movementTypeKey(type: MovementType): string {
  switch (type) {
    case MovementType.PURCHASE:
      return "purchase";
    case MovementType.SALE:
      return "sale";
    case MovementType.ADJUSTMENT:
      return "adjustment";
    case MovementType.WRITE_OFF:
      return "writeOff";
    default:
      return "unspecified";
  }
}

export default function MedicineDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id = "" } = useParams();
  const [editing, setEditing] = useState(false);

  const medQ = useMedicineQuery(id);
  const archive = useArchiveMedicineMutation();
  const unitPricesQ = useMedicineUnitPricesQuery(id, !!id);
  const batchesQ = useBatchesQuery({ medicineId: id, onlyInStock: true, pageSize: ALL_LIMIT });
  const movementsQ = useMovementsQuery({ medicineId: id, pageSize: 10 });

  if (medQ.isLoading) {
    return (
      <Box p={8} textAlign="center">
        <Spinner />
      </Box>
    );
  }
  const med = medQ.data;
  if (!med) {
    return (
      <Box p={8}>
        <Text color="fg.muted">{t("common.noResults")}</Text>
      </Box>
    );
  }

  const onArchive = async () => {
    if (!window.confirm(t("inventory.medicines.confirmArchive"))) return;
    try {
      await archive.mutateAsync({ id: med.id });
      toast.success(t("common.archive") + " ✓");
      navigate("/medicines");
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Box>
      <BackButton to="/medicines" />
      <PageHeader
        breadcrumbs={[{ label: t("nav.medicines"), to: "/medicines" }, { label: med.name }]}
        title={med.name}
        actions={
          <HStack>
            <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
              <Pencil size={14} />
              {t("common.edit")}
            </Button>
            {med.active && (
              <Button size="sm" variant="outline" colorPalette="red" onClick={onArchive}>
                <Archive size={14} />
                {t("common.archive")}
              </Button>
            )}
          </HStack>
        }
      />

      <Stack gap={6}>
        {/* Info + current stock */}
        <Box>
          <Heading size="sm" mb={3}>
            {t("inventory.medicines.infoSection")}
          </Heading>
          <SimpleGrid columns={{ base: 2, md: 4 }} gap={3}>
            <Field label={t("inventory.medicines.sku")} value={med.sku} mono />
            <Field label={t("inventory.medicines.manufacturer")} value={med.manufacturer || "—"} />
            <Field label={t("inventory.medicines.unit")} value={med.unit} />
            <Field label={t("inventory.medicines.unitPrice")} value={formatMoney(med.unitPrice)} />
            <Field
              label={t("inventory.medicines.lastCost")}
              value={med.referenceCost > 0n ? formatMoney(med.referenceCost) : "—"}
            />
            <Field
              label={t("inventory.medicines.lastRestock")}
              value={
                med.lastRestockDate
                  ? med.lastRestockDate +
                    (med.lastRestockSupplier ? ` · ${med.lastRestockSupplier}` : "")
                  : "—"
              }
            />
            <Tile label={t("inventory.medicines.readyStock")} value={med.readyStock.toString()} />
            <Tile
              label={t("inventory.medicines.onOrder")}
              value={med.onOrderStock > 0n ? med.onOrderStock.toString() : "—"}
              muted
            />
            <Tile
              label={t("inventory.medicines.stockValuation")}
              value={formatMoney(med.stockValuation)}
            />
            <Box>
              <Text fontSize="xs" color="fg.muted" mb={1}>
                {t("inventory.medicines.rx")}
              </Text>
              <Badge colorPalette={med.prescriptionRequired ? "purple" : "gray"}>
                {med.prescriptionRequired ? t("common.yes") : t("common.no")}
              </Badge>
            </Box>
            <Box>
              <Text fontSize="xs" color="fg.muted" mb={1}>
                {t("common.active")}
              </Text>
              <Badge colorPalette={med.active ? "green" : "gray"}>
                {med.active ? t("common.active") : t("common.inactive")}
              </Badge>
            </Box>
          </SimpleGrid>

          {/* Units of measure (base + larger packs). */}
          <Box mt={4}>
            <Text fontSize="xs" color="fg.muted" mb={2}>
              {t("inventory.medicines.unitsSection")}
            </Text>
            <HStack gap={2} wrap="wrap">
              {med.units.map((u) => {
                const m = marginPct(u.sellPrice, med.referenceCost * u.factor);
                return (
                  <HStack
                    key={u.id}
                    gap={2}
                    borderWidth="1px"
                    borderRadius="md"
                    px={3}
                    py={1.5}
                    bg="bg.subtle"
                  >
                    <Text fontWeight="medium">{u.name}</Text>
                    {u.isBase ? (
                      <Badge size="sm" colorPalette="blue">
                        {t("inventory.medicines.baseUnit")}
                      </Badge>
                    ) : (
                      <Text fontSize="xs" color="fg.muted">
                        ×{u.factor.toString()}
                      </Text>
                    )}
                    <Text fontSize="sm" color="fg.muted">
                      {formatMoney(u.sellPrice)}
                    </Text>
                    {m != null && (
                      <Text fontSize="xs" color="fg.muted">
                        · {t("inventory.medicines.marginPct", { pct: m.toFixed(0) })}
                      </Text>
                    )}
                  </HStack>
                );
              })}
            </HStack>
          </Box>
        </Box>

        {/* Batches / Price history / Movements as tabs */}
        <Tabs.Root defaultValue="batches" variant="line">
          <Tabs.List>
            <Tabs.Trigger value="batches">{t("inventory.medicines.batchesSection")}</Tabs.Trigger>
            <Tabs.Trigger value="prices">{t("inventory.medicines.priceHistory")}</Tabs.Trigger>
            <Tabs.Trigger value="movements">{t("inventory.medicines.movementsSection")}</Tabs.Trigger>
          </Tabs.List>

          <Tabs.Content value="batches">
            <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>{t("inventory.batches.batchNumber")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.batches.expiry")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.batches.cost")}</Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">{t("inventory.batches.qty")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {batchesQ.rows.map((b) => (
                <Table.Row key={b.id}>
                  <Table.Cell>{b.batchNumber || "—"}</Table.Cell>
                  <Table.Cell>
                    <HStack gap={2}>
                      <Text>{b.expiryDate}</Text>
                      <ExpiryBadge expiry={b.expiryDate} />
                    </HStack>
                  </Table.Cell>
                  <Table.Cell>{formatMoney(b.costPrice)}</Table.Cell>
                  <Table.Cell textAlign="end">{b.currentQuantity.toString()}</Table.Cell>
                </Table.Row>
              ))}
              {batchesQ.rows.length === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={4}>
                    <Text color="fg.muted" textAlign="center" py={4}>
                      {t("common.noResults")}
                    </Text>
                  </Table.Cell>
                </Table.Row>
              )}
            </Table.Body>
          </Table.Root>
          </Tabs.Content>

          <Tabs.Content value="prices">
          {!unitPricesQ.data || unitPricesQ.data.length === 0 ? (
            <Text fontSize="sm" color="fg.muted">
              {t("inventory.medicines.priceHistoryEmpty")}
            </Text>
          ) : (
            <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
              <Table.Header bg="bg.muted">
                <Table.Row>
                  <Table.ColumnHeader>{t("inventory.medicines.priceHistoryUnitCol")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.medicines.priceFrom")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.medicines.priceTo")}</Table.ColumnHeader>
                  <Table.ColumnHeader>{t("inventory.medicines.pricePrice")}</Table.ColumnHeader>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {unitPricesQ.data.map((p) => (
                  <Table.Row key={p.id}>
                    <Table.Cell>{p.unitName}</Table.Cell>
                    <Table.Cell>{formatUnix(p.effectiveFrom)}</Table.Cell>
                    <Table.Cell>
                      {p.effectiveTo > 0n
                        ? formatUnix(p.effectiveTo)
                        : t("inventory.medicines.priceCurrent")}
                    </Table.Cell>
                    <Table.Cell>{formatMoney(p.unitSellPrice)}</Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Root>
          )}
          </Tabs.Content>

          <Tabs.Content value="movements">
            <Table.Root size="sm" bg="bg.subtle" borderWidth="1px" borderRadius="lg">
            <Table.Header bg="bg.muted">
              <Table.Row>
                <Table.ColumnHeader>{t("inventory.movements.when")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.movements.type")}</Table.ColumnHeader>
                <Table.ColumnHeader textAlign="end">{t("inventory.movements.qty")}</Table.ColumnHeader>
                <Table.ColumnHeader>{t("inventory.movements.reason")}</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {movementsQ.rows.map((m) => (
                <Table.Row key={m.id}>
                  <Table.Cell>{formatUnix(m.createdAt)}</Table.Cell>
                  <Table.Cell>{t(`inventory.movements.types.${movementTypeKey(m.type)}`)}</Table.Cell>
                  <Table.Cell textAlign="end">{m.qty > 0 ? `+${m.qty}` : m.qty}</Table.Cell>
                  <Table.Cell>{m.reason}</Table.Cell>
                </Table.Row>
              ))}
              {movementsQ.rows.length === 0 && (
                <Table.Row>
                  <Table.Cell colSpan={4}>
                    <Text color="fg.muted" textAlign="center" py={4}>
                      {t("common.noResults")}
                    </Text>
                  </Table.Cell>
                </Table.Row>
              )}
            </Table.Body>
          </Table.Root>
          </Tabs.Content>
        </Tabs.Root>
      </Stack>

      <EditMedicineDialog medicine={editing ? med : null} onClose={() => setEditing(false)} />
    </Box>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Box>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontFamily={mono ? "mono" : undefined}>{value}</Text>
    </Box>
  );
}

function Tile({ label, value, muted }: { label: string; value: string; muted?: boolean }) {
  return (
    <Box bg="bg.subtle" borderWidth="1px" borderRadius="lg" px={3} py={2}>
      <Text fontSize="xs" color="fg.muted" mb={1}>
        {label}
      </Text>
      <Text fontSize="lg" fontWeight="semibold" color={muted ? "fg.muted" : undefined}>
        {value}
      </Text>
    </Box>
  );
}
