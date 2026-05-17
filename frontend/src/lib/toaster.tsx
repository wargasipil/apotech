import {
  Portal,
  Stack,
  Toast,
  Toaster as ChakraToaster,
  createToaster,
} from "@chakra-ui/react";
import { ConnectError } from "@connectrpc/connect";

export const toaster = createToaster({
  placement: "top-end",
  pauseOnPageIdle: true,
});

function describe(err: unknown): string {
  if (err instanceof ConnectError) return err.message;
  if (err instanceof Error) return err.message;
  return String(err);
}

export const toast = {
  success(title: string, description?: string) {
    toaster.create({ type: "success", title, description });
  },
  info(title: string, description?: string) {
    toaster.create({ type: "info", title, description });
  },
  error(title: string, description?: string) {
    toaster.create({ type: "error", title, description });
  },
  fromError(err: unknown, fallbackTitle = "Error") {
    toaster.create({
      type: "error",
      title: fallbackTitle,
      description: describe(err),
    });
  },
};

export function AppToaster() {
  return (
    <Portal>
      <ChakraToaster toaster={toaster} insetInline={{ mdDown: "4" }}>
        {(t) => (
          <Toast.Root width={{ md: "sm" }}>
            <Toast.Indicator />
            <Stack gap="1" flex="1" maxWidth="100%">
              {t.title && <Toast.Title>{t.title}</Toast.Title>}
              {t.description && <Toast.Description>{t.description}</Toast.Description>}
            </Stack>
            <Toast.CloseTrigger />
          </Toast.Root>
        )}
      </ChakraToaster>
    </Portal>
  );
}
