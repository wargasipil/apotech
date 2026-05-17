import { useState } from "react";
import { Field, IconButton, Input, InputGroup } from "@chakra-ui/react";
import { Eye, EyeOff } from "lucide-react";
import {
  Controller,
  type Control,
  type FieldPath,
  type FieldValues,
} from "react-hook-form";
import { useTranslation } from "react-i18next";
import type { InputHTMLAttributes } from "react";

type Props<TForm extends FieldValues> = {
  control: Control<TForm>;
  name: FieldPath<TForm>;
  label: string;
  helperText?: string;
  required?: boolean;
  type?: InputHTMLAttributes<HTMLInputElement>["type"];
  inputMode?: InputHTMLAttributes<HTMLInputElement>["inputMode"];
  placeholder?: string;
  autoFocus?: boolean;
  // When type="password", render a show/hide eye toggle as the input's end element.
  passwordToggle?: boolean;
};

export default function FormField<TForm extends FieldValues>(props: Props<TForm>) {
  const {
    control,
    name,
    label,
    helperText,
    required,
    type = "text",
    inputMode,
    placeholder,
    autoFocus,
    passwordToggle,
  } = props;
  const { t } = useTranslation();
  const [visible, setVisible] = useState(false);

  const showToggle = passwordToggle && type === "password";
  const effectiveType = showToggle && visible ? "text" : type;

  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => {
        const input = (
          <Input
            {...field}
            value={field.value ?? ""}
            type={effectiveType}
            inputMode={inputMode}
            placeholder={placeholder}
            autoFocus={autoFocus}
          />
        );
        return (
          <Field.Root required={required} invalid={!!fieldState.error}>
            <Field.Label>
              {label}
              {required && <Field.RequiredIndicator />}
            </Field.Label>
            {showToggle ? (
              <InputGroup
                endElement={
                  <IconButton
                    aria-label={
                      visible ? t("auth.hidePassword") : t("auth.showPassword")
                    }
                    variant="ghost"
                    size="sm"
                    onClick={() => setVisible((v) => !v)}
                    tabIndex={-1}
                  >
                    {visible ? <EyeOff size={16} /> : <Eye size={16} />}
                  </IconButton>
                }
              >
                {input}
              </InputGroup>
            ) : (
              input
            )}
            {fieldState.error ? (
              <Field.ErrorText>{fieldState.error.message}</Field.ErrorText>
            ) : helperText ? (
              <Field.HelperText>{helperText}</Field.HelperText>
            ) : null}
          </Field.Root>
        );
      }}
    />
  );
}
