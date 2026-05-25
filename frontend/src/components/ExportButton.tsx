import { useState } from "react";
import { Button } from "@chakra-ui/react";
import { Download } from "lucide-react";
import { useTranslation } from "react-i18next";

import { toast } from "../lib/toaster";

type Props = {
  // Async exporter: fetch all matching rows, serialize, trigger the download.
  onExport: () => Promise<void>;
  disabled?: boolean;
  size?: "xs" | "sm" | "md" | "lg";
};

// Shared "Export CSV" button. Shows a loading state while the async exporter
// runs; surfaces failures via the global toast.
export default function ExportButton({ onExport, disabled, size = "sm" }: Props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const run = async () => {
    setLoading(true);
    try {
      await onExport();
    } catch (e) {
      toast.fromError(e);
    } finally {
      setLoading(false);
    }
  };
  return (
    <Button size={size} variant="outline" onClick={run} loading={loading} disabled={disabled}>
      <Download size={16} />
      {t("common.exportCsv")}
    </Button>
  );
}
