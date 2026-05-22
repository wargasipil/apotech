import { Box, Dialog, Heading, HStack, IconButton, Portal, Stack } from "@chakra-ui/react";
import { X } from "lucide-react";
import type { ReactNode } from "react";

// Centered modal counterpart of <EntityDrawer> — same prop API, so a caller can
// swap one for the other. Use for create/edit flows that should feel like a
// quick popup rather than a slide-over (e.g. the Obat medicine form).
type Props = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md" | "lg" | "xl";
};

export default function EntityDialog({
  open,
  onClose,
  title,
  children,
  footer,
  size = "md",
}: Props) {
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onClose();
      }}
      size={size}
    >
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header borderBottomWidth="1px">
              <HStack justify="space-between">
                <Heading size="lg">{title}</Heading>
                <IconButton aria-label="close" variant="ghost" size="sm" onClick={onClose}>
                  <X size={18} />
                </IconButton>
              </HStack>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap={4}>{children}</Stack>
            </Dialog.Body>
            {footer && (
              <Dialog.Footer borderTopWidth="1px">
                <Box w="full">{footer}</Box>
              </Dialog.Footer>
            )}
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
