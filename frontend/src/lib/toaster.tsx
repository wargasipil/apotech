import {
  Toast,
  Toaster as ChakraToaster,
  Stack,
  createToaster,
} from "@chakra-ui/react";
import { ConnectError } from "@connectrpc/connect";

export const toaster = createToaster({
  placement: "top-end",
  overlap: true,
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
    <ChakraToaster toaster={toaster}>
      {(t) => (
        <Toast.Root>
          <Toast.Indicator />
          <Stack gap={0} flex="1" maxW="100%">
            {t.title && <Toast.Title>{t.title}</Toast.Title>}
            {t.description && <Toast.Description>{t.description}</Toast.Description>}
          </Stack>
          <Toast.CloseTrigger />
        </Toast.Root>
      )}
    </ChakraToaster>
  );
}
