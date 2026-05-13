import { Box, Drawer, Heading, HStack, IconButton, Portal, Stack } from "@chakra-ui/react";
import { X } from "lucide-react";
import type { ReactNode } from "react";

type Props = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md" | "lg" | "xl";
};

export default function EntityDrawer({
  open,
  onClose,
  title,
  children,
  footer,
  size = "md",
}: Props) {
  return (
    <Drawer.Root
      open={open}
      onOpenChange={(d) => {
        if (!d.open) onClose();
      }}
      placement="end"
      size={size}
    >
      <Portal>
        <Drawer.Backdrop />
        <Drawer.Positioner>
          <Drawer.Content>
            <Drawer.Header borderBottomWidth="1px">
              <HStack justify="space-between">
                <Heading size="lg">{title}</Heading>
                <IconButton aria-label="close" variant="ghost" size="sm" onClick={onClose}>
                  <X size={18} />
                </IconButton>
              </HStack>
            </Drawer.Header>
            <Drawer.Body>
              <Stack gap={4}>{children}</Stack>
            </Drawer.Body>
            {footer && (
              <Drawer.Footer borderTopWidth="1px">
                <Box w="full">{footer}</Box>
              </Drawer.Footer>
            )}
          </Drawer.Content>
        </Drawer.Positioner>
      </Portal>
    </Drawer.Root>
  );
}
