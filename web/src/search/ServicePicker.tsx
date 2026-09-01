import { useEffect, useState } from "react";
import { listServices } from "./api";
import { NamePicker } from "./NamePicker";
import type { SearchForm } from "./query";

type ServicePickerProps = {
  form: SearchForm;
  disabled: boolean;
  onChange: (service: string) => void;
  onPick: (service: string) => void;
};

export function ServicePicker({ form, disabled, onChange, onPick }: ServicePickerProps) {
  const [names, setNames] = useState<string[]>([]);

  useEffect(() => {
    const ac = new AbortController();
    void (async () => {
      try {
        const rows = await listServices(
          {
            service: "",
            op: "",
            min: "",
            status: "",
            start: form.start,
            end: form.end,
            limit: "100",
          },
          ac.signal,
        );
        if (ac.signal.aborted) {
          return;
        }
        setNames(rows.map((r) => r.service).filter(Boolean));
      } catch {
        if (ac.signal.aborted) {
          return;
        }
        setNames([]);
      }
    })();
    return () => ac.abort();
  }, [form.start, form.end]);

  return (
    <NamePicker
      label="service"
      name="service"
      menuId="service-menu"
      value={form.service}
      names={names}
      disabled={disabled}
      swatch
      onChange={onChange}
      onPick={onPick}
    />
  );
}
