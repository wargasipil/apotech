import { Box, Button, Heading, Stack } from "@chakra-ui/react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { Pill } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { Navigate, useNavigate } from "react-router-dom";
import { z } from "zod";

import FormField from "../components/FormField";
import { useAuth } from "../lib/auth";
import { toast } from "../lib/toaster";

const Schema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});
type FormValues = z.infer<typeof Schema>;

export default function Login() {
  const { t } = useTranslation();
  const { user, login } = useAuth();
  const navigate = useNavigate();

  const form = useForm<FormValues>({
    resolver: zodResolver(Schema),
    defaultValues: { email: "", password: "" },
  });

  const mutation = useMutation({
    mutationFn: ({ email, password }: FormValues) => login(email, password),
    onSuccess: () => navigate("/", { replace: true }),
    onError: (err) => toast.fromError(err, t("auth.invalidCredentials")),
    meta: { silentError: true },
  });

  if (user) return <Navigate to="/" replace />;

  return (
    <Box maxW="sm" mx="auto" mt={20} p={6} bg="bg.subtle" borderWidth="1px" borderRadius="lg" shadow="sm">
      <Stack gap={5}>
        <Stack gap={2} align="center">
          <Box color="blue.solid">
            <Pill size={32} />
          </Box>
          <Heading size="lg">{t("auth.signInTitle")}</Heading>
        </Stack>
        <form onSubmit={form.handleSubmit((v) => mutation.mutate(v))}>
          <Stack gap={4}>
            <FormField
              control={form.control}
              name="email"
              label={t("auth.email")}
              type="email"
              required
              autoFocus
            />
            <FormField
              control={form.control}
              name="password"
              label={t("auth.password")}
              type="password"
              required
              passwordToggle
            />
            <Button type="submit" colorPalette="blue" loading={mutation.isPending}>
              {t("auth.signIn")}
            </Button>
          </Stack>
        </form>
      </Stack>
    </Box>
  );
}
