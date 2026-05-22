import { DatePicker, IconButton, Portal, parseDate } from "@chakra-ui/react";
import { Calendar, ChevronLeft, ChevronRight, X } from "lucide-react";
import { useTranslation } from "react-i18next";

// Calendar-popover date picker for the apotech UI. Wraps Chakra v3's
// `DatePicker` namespace (a real calendar, not the OS-native `<input type="date">`
// chrome) so every date field across the app shares one consistent look —
// the same reasoning behind <SearchableSelect>/<EnumSelect> replacing the
// native `<select>`. See the "ChakraUI-first" rule in CLAUDE.md.
//
// Value contract matches the rest of the app: a `YYYY-MM-DD` string in,
// a `YYYY-MM-DD` string (or "" when cleared) out — so it's a drop-in for
// the previous `<Input type="date">` call sites and needs no backend change.
//
// The popover content is wrapped in <Portal> so it escapes EntityDrawer /
// Dialog stacking contexts (this component is mounted inside both).
export type DatePickerProps = {
  value: string | null;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  size?: "xs" | "sm" | "md" | "lg";
  width?: string | number;
  /** Optional lower bound, `YYYY-MM-DD`. */
  min?: string;
  /** Optional upper bound, `YYYY-MM-DD`. */
  max?: string;
  /** Show the inline clear ("×") button. Defaults to true. */
  clearable?: boolean;
};

// `parseDate` throws on empty / malformed input; guard it so a blank or
// half-typed value just renders as "no selection" instead of crashing.
function toDateValue(s?: string | null) {
  if (!s) return undefined;
  try {
    return parseDate(s);
  } catch {
    return undefined;
  }
}

export default function DatePickerField({
  value,
  onChange,
  placeholder,
  disabled,
  size = "md",
  width,
  min,
  max,
  clearable = true,
}: DatePickerProps) {
  const { t, i18n } = useTranslation();

  const selected = toDateValue(value);
  const minValue = toDateValue(min);
  const maxValue = toDateValue(max);

  // The day / month / year grids share the same nav header; factor it out so
  // the three <DatePicker.View> blocks below stay readable.
  const viewControl = (label: string) => (
    <DatePicker.ViewControl>
      <DatePicker.PrevTrigger asChild>
        <IconButton aria-label={t("datepicker.prev")} variant="ghost" size="sm">
          <ChevronLeft size={16} />
        </IconButton>
      </DatePicker.PrevTrigger>
      <DatePicker.ViewTrigger asChild>
        <IconButton aria-label={label} variant="ghost" size="sm" width="auto" px={3}>
          <DatePicker.RangeText />
        </IconButton>
      </DatePicker.ViewTrigger>
      <DatePicker.NextTrigger asChild>
        <IconButton aria-label={t("datepicker.next")} variant="ghost" size="sm">
          <ChevronRight size={16} />
        </IconButton>
      </DatePicker.NextTrigger>
    </DatePicker.ViewControl>
  );

  return (
    <DatePicker.Root
      value={selected ? [selected] : []}
      // Derive the ISO `YYYY-MM-DD` from the DateValue (CalendarDate.toString()),
      // NOT from `valueAsString` — the latter is locale-formatted (e.g. "05/20/2026")
      // and would fail to round-trip back through parseDate, blanking the field.
      onValueChange={(d) => onChange(d.value[0]?.toString() ?? "")}
      min={minValue}
      max={maxValue}
      locale={i18n.language}
      disabled={disabled}
      size={size}
      width={width}
    >
      <DatePicker.Control>
        <DatePicker.Input placeholder={placeholder} />
        <DatePicker.IndicatorGroup>
          {clearable && (
            <DatePicker.ClearTrigger asChild>
              <IconButton
                aria-label={t("datepicker.clear")}
                variant="ghost"
                size="xs"
              >
                <X size={14} />
              </IconButton>
            </DatePicker.ClearTrigger>
          )}
          <DatePicker.Trigger asChild>
            <IconButton
              aria-label={t("datepicker.open")}
              variant="ghost"
              size="xs"
            >
              <Calendar size={16} />
            </IconButton>
          </DatePicker.Trigger>
        </DatePicker.IndicatorGroup>
      </DatePicker.Control>
      <Portal>
        <DatePicker.Positioner>
          <DatePicker.Content>
            {/* Day view */}
            <DatePicker.View view="day">
              <DatePicker.Context>
                {(api) => (
                  <>
                    {viewControl(t("datepicker.switchMonth"))}
                    <DatePicker.Table>
                      <DatePicker.TableHead>
                        <DatePicker.TableRow>
                          {api.weekDays.map((weekDay, i) => (
                            <DatePicker.TableHeader key={i}>
                              {weekDay.narrow}
                            </DatePicker.TableHeader>
                          ))}
                        </DatePicker.TableRow>
                      </DatePicker.TableHead>
                      <DatePicker.TableBody>
                        {api.weeks.map((week, i) => (
                          <DatePicker.TableRow key={i}>
                            {week.map((day, j) => (
                              <DatePicker.TableCell key={j} value={day}>
                                <DatePicker.TableCellTrigger>
                                  {day.day}
                                </DatePicker.TableCellTrigger>
                              </DatePicker.TableCell>
                            ))}
                          </DatePicker.TableRow>
                        ))}
                      </DatePicker.TableBody>
                    </DatePicker.Table>
                  </>
                )}
              </DatePicker.Context>
            </DatePicker.View>

            {/* Month view */}
            <DatePicker.View view="month">
              <DatePicker.Context>
                {(api) => (
                  <>
                    {viewControl(t("datepicker.switchYear"))}
                    <DatePicker.Table>
                      <DatePicker.TableBody>
                        {api
                          .getMonthsGrid({ columns: 4, format: "short" })
                          .map((months, i) => (
                            <DatePicker.TableRow key={i}>
                              {months.map((month, j) => (
                                <DatePicker.TableCell key={j} value={month.value}>
                                  <DatePicker.TableCellTrigger>
                                    {month.label}
                                  </DatePicker.TableCellTrigger>
                                </DatePicker.TableCell>
                              ))}
                            </DatePicker.TableRow>
                          ))}
                      </DatePicker.TableBody>
                    </DatePicker.Table>
                  </>
                )}
              </DatePicker.Context>
            </DatePicker.View>

            {/* Year view */}
            <DatePicker.View view="year">
              <DatePicker.Context>
                {(api) => (
                  <>
                    {viewControl(t("datepicker.switchDecade"))}
                    <DatePicker.Table>
                      <DatePicker.TableBody>
                        {api.getYearsGrid({ columns: 4 }).map((years, i) => (
                          <DatePicker.TableRow key={i}>
                            {years.map((year, j) => (
                              <DatePicker.TableCell key={j} value={year.value}>
                                <DatePicker.TableCellTrigger>
                                  {year.label}
                                </DatePicker.TableCellTrigger>
                              </DatePicker.TableCell>
                            ))}
                          </DatePicker.TableRow>
                        ))}
                      </DatePicker.TableBody>
                    </DatePicker.Table>
                  </>
                )}
              </DatePicker.Context>
            </DatePicker.View>
          </DatePicker.Content>
        </DatePicker.Positioner>
      </Portal>
    </DatePicker.Root>
  );
}
