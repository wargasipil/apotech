import { Field, Input } from "@chakra-ui/react";
import {
  Controller,
  type Control,
  type FieldPath,
  type FieldValues,
} from "react-hook-form";
import type { InputHTMLAttributes, ReactNode } from "react";

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
  rightElement?: ReactNode;
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
  } = props;
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <Field.Root required={required} invalid={!!fieldState.error}>
          <Field.Label>
            {label}
            {required && <Field.RequiredIndicator />}
          </Field.Label>
          <Input
            {...field}
            value={field.value ?? ""}
            type={type}
            inputMode={inputMode}
            placeholder={placeholder}
            autoFocus={autoFocus}
          />
          {fieldState.error ? (
            <Field.ErrorText>{fieldState.error.message}</Field.ErrorText>
          ) : helperText ? (
            <Field.HelperText>{helperText}</Field.HelperText>
          ) : null}
        </Field.Root>
      )}
    />
  );
}
