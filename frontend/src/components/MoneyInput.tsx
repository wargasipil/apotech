import { useLayoutEffect, useRef } from "react";
import type { ChangeEvent } from "react";
import { Input } from "@chakra-ui/react";

import { formatThousands, parseThousands } from "../lib/format";

type Props = {
  value: number | bigint | string | null | undefined;
  onChange: (raw: string) => void;
  placeholder?: string;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
  disabled?: boolean;
  autoFocus?: boolean;
  onBlur?: () => void;
  "aria-label"?: string;
};

// MoneyInput — controlled integer-money input. Composes Chakra <Input>
// (type=text + inputMode=numeric) and shows the value grouped by the active
// locale's thousands separator (id -> "200.000"). onChange emits the UNFORMATTED
// digit string ("" for empty/zero) so z.coerce.bigint() and Number(raw || 0)
// consumers parse cleanly. A zero value renders empty (placeholder), and typed
// leading zeros are stripped — so "0" + "9000" never becomes "09000".
export default function MoneyInput({
  value,
  onChange,
  placeholder = "0",
  size,
  width,
  disabled,
  autoFocus,
  onBlur,
  "aria-label": ariaLabel,
}: Props) {
  const ref = useRef<HTMLInputElement>(null);
  // Digits that should sit left of the caret after reformat; set on each edit,
  // applied + cleared in the layout effect (null = external value change, skip).
  const pendingCaretDigits = useRef<number | null>(null);

  const display = formatThousands(value ?? "");

  useLayoutEffect(() => {
    const el = ref.current;
    const want = pendingCaretDigits.current;
    if (!el || want === null) return;
    pendingCaretDigits.current = null;
    let pos = 0;
    if (want > 0) {
      let seen = 0;
      pos = display.length;
      for (let i = 0; i < display.length; i++) {
        const c = display.charCodeAt(i);
        if (c >= 48 && c <= 57) {
          seen++;
          if (seen === want) {
            pos = i + 1;
            break;
          }
        }
      }
    }
    el.setSelectionRange(pos, pos);
  }, [display]);

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    const typed = e.target.value;
    const caret = e.target.selectionStart ?? typed.length;
    let digitsLeft = 0;
    for (let i = 0; i < caret; i++) {
      const c = typed.charCodeAt(i);
      if (c >= 48 && c <= 57) digitsLeft++;
    }
    const raw = parseThousands(typed);
    pendingCaretDigits.current = Math.min(digitsLeft, raw.length);
    onChange(raw);
  };

  return (
    <Input
      ref={ref}
      type="text"
      inputMode="numeric"
      value={display}
      placeholder={placeholder}
      size={size}
      width={width}
      disabled={disabled}
      autoFocus={autoFocus}
      onChange={handleChange}
      onBlur={onBlur}
      aria-label={ariaLabel}
    />
  );
}
