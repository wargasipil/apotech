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
