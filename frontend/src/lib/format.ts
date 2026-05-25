import i18n from "./i18n";

function currentLocale(): string {
  // i18next stores BCP-47 in `language`; we use "id" or "en".
  return i18n.language || "id";
}

const idrMap = new Map<string, Intl.NumberFormat>();

export function formatMoney(amount: number | bigint): string {
  const locale = currentLocale();
  let fmt = idrMap.get(locale);
  if (!fmt) {
    fmt = new Intl.NumberFormat(locale, {
      style: "currency",
      currency: "IDR",
      maximumFractionDigits: 0,
    });
    idrMap.set(locale, fmt);
  }
  return fmt.format(Number(amount));
}

const groupMap = new Map<string, Intl.NumberFormat>();

// parseThousands reduces any typed/formatted money string to canonical digits:
// keeps [0-9] and drops leading zeros. Returns "" for empty OR zero, so callers
// can show a placeholder instead of a stuck "0" (e.g. "0" -> "", "09000" ->
// "9000", "200.000" -> "200000"). Integer money only (no decimal point).
export function parseThousands(input: string): string {
  return input.replace(/\D/g, "").replace(/^0+/, "");
}

// formatThousands groups an integer's digits with the active locale's thousands
// separator (id -> "200.000", en -> "200,000"). Accepts a raw digit string,
// number, or bigint. Returns "" for empty input or 0. Formats from BigInt so
// large amounts don't lose precision.
export function formatThousands(value: number | bigint | string): string {
  const digits = parseThousands(typeof value === "string" ? value : String(value));
  if (digits === "") return "";
  const locale = currentLocale();
  let fmt = groupMap.get(locale);
  if (!fmt) {
    fmt = new Intl.NumberFormat(locale, {
      useGrouping: true,
      maximumFractionDigits: 0,
    });
    groupMap.set(locale, fmt);
  }
  return fmt.format(BigInt(digits));
}

export function formatDate(input: Date | string | number): string {
  const date = input instanceof Date ? input : new Date(input);
  return new Intl.DateTimeFormat(currentLocale(), { dateStyle: "medium" }).format(date);
}

export function formatDateTime(input: Date | string | number): string {
  const date = input instanceof Date ? input : new Date(input);
  return new Intl.DateTimeFormat(currentLocale(), {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

// For unix seconds (proto-wire timestamps).
export function formatUnix(sec: number | bigint): string {
  return formatDateTime(Number(sec) * 1000);
}
