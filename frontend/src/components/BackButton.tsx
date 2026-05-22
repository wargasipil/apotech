import { Button } from "@chakra-ui/react";
import { ArrowLeft } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

// Reusable back-navigation affordance for detail pages. HARD RULE: every
// single-record detail page renders this at the top. Pass `to` (the parent
// list) for predictable, deep-link-safe navigation; omit it to fall back to
// browser history (navigate(-1)). Always client-side (no full reload).
type Props = {
  to?: string;
  label?: string;
};

export default function BackButton({ to, label }: Props) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  return (
    <Button
      variant="ghost"
      size="sm"
      alignSelf="flex-start"
      onClick={() => (to ? navigate(to) : navigate(-1))}
    >
      <ArrowLeft size={16} />
      {label ?? t("common.back")}
    </Button>
  );
}
