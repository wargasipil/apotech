import {
  Box,
  Button,
  Dialog,
  HStack,
  IconButton,
  Input,
  Portal,
  Stack,
  Text,
} from "@chakra-ui/react";
import { X } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { toast } from "../lib/toaster";
import { useChangePasswordMutation } from "../queries/users";

// Shared dialog for both "change my password" (TopBar menu) and "OWNER changes
// another user's password" (Users admin). The backend ChangePassword RPC
// already supports both paths: empty user_id => self (requires old_password),
// non-empty user_id => OWNER only (no old_password required).
export default function ChangePasswordDialog({
  open,
  onClose,
  userId,
  isSelf,
  userLabel,
}: {
  open: boolean;
  onClose: () => void;
  userId?: string;
  isSelf: boolean;
  userLabel?: string;
}) {
  const { t } = useTranslation();
  const change = useChangePasswordMutation();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");

  // Reset fields whenever the dialog reopens (different user, or after success).
  useEffect(() => {
    if (open) {
      setCurrent("");
      setNext("");
      setConfirm("");
    }
  }, [open]);

  const tooShort = next.length > 0 && next.length < 8;
  const mismatch = confirm.length > 0 && confirm !== next;
  const canSubmit =
    next.length >= 8 && confirm === next && (!isSelf || current.length > 0);

  const submit = async () => {
    try {
      await change.mutateAsync({
        userId: isSelf ? "" : userId ?? "",
        oldPassword: isSelf ? current : "",
        newPassword: next,
      });
      toast.success(t("users.changePassword") + " ✓");
      onClose();
    } catch {
      /* toast handled globally */
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={(d) => !d.open && onClose()} size="sm">
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>
                {isSelf ? t("users.changeMyPassword") : t("users.changePassword")}
              </Dialog.Title>
              <Dialog.CloseTrigger asChild>
                <IconButton aria-label="close" variant="ghost" size="sm">
                  <X size={16} />
                </IconButton>
              </Dialog.CloseTrigger>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={3}>
                {!isSelf && userLabel && (
                  <Text fontSize="sm" color="fg.muted">
                    {userLabel}
                  </Text>
                )}
                {isSelf && (
                  <Box>
                    <Text fontSize="sm" fontWeight="medium" mb={1} color="fg.muted">
                      {t("users.currentPassword")}
                    </Text>
                    <Input
                      type="password"
                      value={current}
                      onChange={(e) => setCurrent(e.target.value)}
                      autoFocus
                    />
                  </Box>
                )}
                <Box>
                  <Text fontSize="sm" fontWeight="medium" mb={1} color="fg.muted">
                    {t("users.newPassword")}
                  </Text>
                  <Input
                    type="password"
                    value={next}
                    onChange={(e) => setNext(e.target.value)}
                    autoFocus={!isSelf}
                  />
                  {tooShort && (
                    <Text fontSize="xs" color="red.500" mt={1}>
                      {t("users.passwordMinLength")}
                    </Text>
                  )}
                </Box>
                <Box>
                  <Text fontSize="sm" fontWeight="medium" mb={1} color="fg.muted">
                    {t("users.confirmPassword")}
                  </Text>
                  <Input
                    type="password"
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                  />
                  {mismatch && (
                    <Text fontSize="xs" color="red.500" mt={1}>
                      {t("users.passwordMismatch")}
                    </Text>
                  )}
                </Box>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <HStack justify="space-between" w="full">
                <Button variant="ghost" onClick={onClose}>
                  {t("common.cancel")}
                </Button>
                <Button
                  colorPalette="blue"
                  onClick={submit}
                  loading={change.isPending}
                  disabled={!canSubmit}
                >
                  {t("common.save")}
                </Button>
              </HStack>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
