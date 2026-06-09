import {
  Box,
  Button,
  Heading,
  HStack,
  IconButton,
  Input,
  Spinner,
  Stack,
  Table,
  Text,
} from "@chakra-ui/react";
import { Check, Pencil, Plus, Trash2, X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { toast } from "../../lib/toaster";
import {
  useArchiveUnitBaseMutation,
  useArchiveUnitDerivativeMutation,
  useCreateUnitBaseMutation,
  useCreateUnitDerivativeMutation,
  useUnitBasesQuery,
  useUpdateUnitBaseMutation,
  useUpdateUnitDerivativeMutation,
} from "../../queries/units";

// SettingsUnits — owner manages the global unit catalog (base + derivatives).
// Bases listed as cards; each card holds a sub-table of derivatives + an inline
// "Add derivative" form. Archive flips active=false (records preserved).
export default function SettingsUnits() {
  const { t } = useTranslation();
  const basesQ = useUnitBasesQuery();
  const createBase = useCreateUnitBaseMutation();
  const updateBase = useUpdateUnitBaseMutation();
  const archiveBase = useArchiveUnitBaseMutation();
  const createDeriv = useCreateUnitDerivativeMutation();
  const updateDeriv = useUpdateUnitDerivativeMutation();
  const archiveDeriv = useArchiveUnitDerivativeMutation();

  // Inline-edit state: which base or derivative is being edited, plus the
  // draft value for each kind.
  const [editingBase, setEditingBase] = useState<{ id: string; name: string } | null>(null);
  const [editingDeriv, setEditingDeriv] = useState<{ id: string; name: string; factor: string } | null>(null);

  const onSaveBase = async () => {
    if (!editingBase) return;
    const name = editingBase.name.trim();
    if (!name) return;
    try {
      await updateBase.mutateAsync({ id: editingBase.id, name });
      setEditingBase(null);
      toast.success(t("common.save") + " ✓");
    } catch {
      /* */
    }
  };
  const onSaveDeriv = async () => {
    if (!editingDeriv) return;
    const name = editingDeriv.name.trim();
    const factor = BigInt(Math.trunc(Number(editingDeriv.factor) || 0));
    if (!name || factor <= 1n) {
      toast.error(t("settings.units.factorMustBeMoreThanOne"));
      return;
    }
    try {
      await updateDeriv.mutateAsync({
        id: editingDeriv.id,
        name,
        factor,
        sortOrder: 0,
      });
      setEditingDeriv(null);
      toast.success(t("common.save") + " ✓");
    } catch {
      /* */
    }
  };

  const [newBase, setNewBase] = useState("");
  const [newDeriv, setNewDeriv] = useState<Record<string, { name: string; factor: string }>>({});

  const onAddBase = async () => {
    const name = newBase.trim();
    if (!name) return;
    try {
      await createBase.mutateAsync({ name });
      setNewBase("");
      toast.success(t("common.create") + " ✓");
    } catch {
      /* */
    }
  };

  const onAddDeriv = async (baseId: string) => {
    const d = newDeriv[baseId];
    if (!d?.name.trim() || !d.factor) return;
    const f = BigInt(Math.trunc(Number(d.factor) || 0));
    if (f <= 1n) {
      toast.error(t("settings.units.factorMustBeMoreThanOne"));
      return;
    }
    try {
      await createDeriv.mutateAsync({
        baseUnitId: baseId,
        name: d.name.trim(),
        factor: f,
        sortOrder: 0,
      });
      setNewDeriv((cur) => ({ ...cur, [baseId]: { name: "", factor: "" } }));
      toast.success(t("common.create") + " ✓");
    } catch {
      /* */
    }
  };

  const bases = basesQ.data ?? [];

  return (
    <Box maxW="3xl">
      <Text fontSize="xs" color="fg.muted" mb={3}>
        {t("settings.units.help")}
      </Text>

      <HStack gap={2} mb={3}>
        <Input
          width="200px"
          size="sm"
          placeholder={t("settings.units.basePlaceholder")}
          value={newBase}
          onChange={(e) => setNewBase(e.target.value)}
        />
        <Button
          size="sm"
          colorPalette="blue"
          onClick={onAddBase}
          loading={createBase.isPending}
        >
          <Plus size={14} />
          {t("settings.units.addBase")}
        </Button>
      </HStack>

      {basesQ.isLoading ? (
        <Box p={6} textAlign="center">
          <Spinner size="sm" />
        </Box>
      ) : bases.length === 0 ? (
        <Box p={6} borderWidth="1px" borderRadius="md" textAlign="center">
          <Text fontSize="sm" color="fg.muted">
            {t("settings.units.empty")}
          </Text>
        </Box>
      ) : (
        <Stack gap={4}>
          {bases.map((b) => {
            const draft = newDeriv[b.id] ?? { name: "", factor: "" };
            return (
              <Box key={b.id} borderWidth="1px" borderRadius="md" p={3}>
                <HStack justify="space-between" mb={2}>
                  {editingBase?.id === b.id ? (
                    <HStack gap={1}>
                      <Input
                        size="sm"
                        width="180px"
                        value={editingBase.name}
                        onChange={(e) =>
                          setEditingBase({ ...editingBase, name: e.target.value })
                        }
                      />
                      <IconButton
                        aria-label={t("common.save")}
                        size="xs"
                        variant="ghost"
                        onClick={onSaveBase}
                        loading={updateBase.isPending}
                      >
                        <Check size={14} />
                      </IconButton>
                      <IconButton
                        aria-label={t("common.cancel")}
                        size="xs"
                        variant="ghost"
                        onClick={() => setEditingBase(null)}
                      >
                        <X size={14} />
                      </IconButton>
                    </HStack>
                  ) : (
                    <Heading size="sm">{b.name}</Heading>
                  )}
                  <HStack gap={1}>
                    {editingBase?.id !== b.id && (
                      <IconButton
                        aria-label={t("common.edit")}
                        size="xs"
                        variant="ghost"
                        onClick={() => setEditingBase({ id: b.id, name: b.name })}
                      >
                        <Pencil size={14} />
                      </IconButton>
                    )}
                    <IconButton
                      aria-label={t("common.archive")}
                      size="xs"
                      variant="ghost"
                      onClick={() => archiveBase.mutate({ id: b.id })}
                    >
                      <Trash2 size={14} />
                    </IconButton>
                  </HStack>
                </HStack>

                {b.derivatives.length === 0 ? (
                  <Text fontSize="xs" color="fg.muted" mb={2}>
                    {t("settings.units.derivativesEmpty")}
                  </Text>
                ) : (
                  <Table.Root size="sm" mb={2}>
                    <Table.Header>
                      <Table.Row>
                        <Table.ColumnHeader>{t("settings.units.derivative")}</Table.ColumnHeader>
                        <Table.ColumnHeader>{t("settings.units.factor")}</Table.ColumnHeader>
                        <Table.ColumnHeader textAlign="end" />
                      </Table.Row>
                    </Table.Header>
                    <Table.Body>
                      {b.derivatives.map((d) => {
                        const isEditing = editingDeriv?.id === d.id;
                        return (
                          <Table.Row key={d.id}>
                            <Table.Cell>
                              {isEditing ? (
                                <Input
                                  size="xs"
                                  width="120px"
                                  value={editingDeriv!.name}
                                  onChange={(e) =>
                                    setEditingDeriv({ ...editingDeriv!, name: e.target.value })
                                  }
                                />
                              ) : (
                                d.name
                              )}
                            </Table.Cell>
                            <Table.Cell>
                              {isEditing ? (
                                <HStack gap={1}>
                                  <Input
                                    size="xs"
                                    type="number"
                                    width="80px"
                                    value={editingDeriv!.factor}
                                    onChange={(e) =>
                                      setEditingDeriv({ ...editingDeriv!, factor: e.target.value })
                                    }
                                  />
                                  <Text fontSize="xs">× {b.name}</Text>
                                </HStack>
                              ) : (
                                <>
                                  {d.factor.toString()} × {b.name}
                                </>
                              )}
                            </Table.Cell>
                            <Table.Cell textAlign="end">
                              <HStack gap={1} justify="flex-end">
                                {isEditing ? (
                                  <>
                                    <IconButton
                                      aria-label={t("common.save")}
                                      size="xs"
                                      variant="ghost"
                                      onClick={onSaveDeriv}
                                      loading={updateDeriv.isPending}
                                    >
                                      <Check size={14} />
                                    </IconButton>
                                    <IconButton
                                      aria-label={t("common.cancel")}
                                      size="xs"
                                      variant="ghost"
                                      onClick={() => setEditingDeriv(null)}
                                    >
                                      <X size={14} />
                                    </IconButton>
                                  </>
                                ) : (
                                  <>
                                    <IconButton
                                      aria-label={t("common.edit")}
                                      size="xs"
                                      variant="ghost"
                                      onClick={() =>
                                        setEditingDeriv({
                                          id: d.id,
                                          name: d.name,
                                          factor: d.factor.toString(),
                                        })
                                      }
                                    >
                                      <Pencil size={14} />
                                    </IconButton>
                                    <IconButton
                                      aria-label={t("common.archive")}
                                      size="xs"
                                      variant="ghost"
                                      onClick={() => archiveDeriv.mutate({ id: d.id })}
                                    >
                                      <Trash2 size={14} />
                                    </IconButton>
                                  </>
                                )}
                              </HStack>
                            </Table.Cell>
                          </Table.Row>
                        );
                      })}
                    </Table.Body>
                  </Table.Root>
                )}

                <HStack gap={2}>
                  <Input
                    size="sm"
                    width="140px"
                    placeholder={t("settings.units.derivPlaceholder")}
                    value={draft.name}
                    onChange={(e) =>
                      setNewDeriv((cur) => ({
                        ...cur,
                        [b.id]: { ...draft, name: e.target.value },
                      }))
                    }
                  />
                  <Input
                    size="sm"
                    width="100px"
                    type="number"
                    placeholder={t("settings.units.factor")}
                    value={draft.factor}
                    onChange={(e) =>
                      setNewDeriv((cur) => ({
                        ...cur,
                        [b.id]: { ...draft, factor: e.target.value },
                      }))
                    }
                  />
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => onAddDeriv(b.id)}
                    loading={createDeriv.isPending}
                  >
                    <Plus size={12} />
                    {t("settings.units.addDerivative")}
                  </Button>
                </HStack>
              </Box>
            );
          })}
        </Stack>
      )}
    </Box>
  );
}
