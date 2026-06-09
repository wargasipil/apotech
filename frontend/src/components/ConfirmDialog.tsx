import type { ReactNode } from "react";
import {
  Button,
  Dialog,
  Heading,
  HStack,
  IconButton,
  Portal,
  Text,
} from "@chakra-ui/react";
import { X } from "lucide-react";
import { useTranslation } from "react-i18next";

// ConfirmDialog — shared destructive-action confirmation. Replaces native
// browser window.confirm() per the HARD RULE: never use native confirm/alert/
// prompt; use this or an inline Chakra Dialog.Root.
//
// Pattern mirrors the inline Dialog in routes/settings/SettingsBackups.tsx.
type Props = {
  open: boolean;
  title: string;
  body: string | ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  loading?: boolean;
  onConfirm: () => void | Promise<void>;
  onClose: () => void;
};

export default function ConfirmDialog({
  open,
  title,
  body,
  confirmLabel,
  cancelLabel,
  destructive,
  loading,
  onConfirm,
  onClose,
}: Props) {
  const { t } = useTranslation();
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onClose();
      }}
      size="sm"
    >
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header borderBottomWidth="1px">
              <HStack justify="space-between">
                <Heading size="md">{title}</Heading>
                <IconButton
                  aria-label="close"
                  variant="ghost"
                  size="sm"
                  onClick={onClose}
                >
                  <X size={18} />
                </IconButton>
              </HStack>
            </Dialog.Header>
            <Dialog.Body>
              {typeof body === "string" ? <Text fontSize="sm">{body}</Text> : body}
            </Dialog.Body>
            <Dialog.Footer borderTopWidth="1px">
              <HStack justify="flex-end" w="full" gap={2}>
                <Button variant="ghost" onClick={onClose} disabled={loading}>
                  {cancelLabel ?? t("common.cancel")}
                </Button>
                <Button
                  colorPalette={destructive ? "red" : "blue"}
                  onClick={onConfirm}
                  loading={loading}
                >
                  {confirmLabel ?? t("common.confirm")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
