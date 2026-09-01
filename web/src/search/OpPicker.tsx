import { useEffect, useState } from "react";
import { listOperations } from "./api";
import { NamePicker } from "./NamePicker";
import type { SearchForm } from "./query";

type OpPickerProps = {
  form: SearchForm;
  disabled: boolean;
  onChange: (op: string) => void;
};

export function OpPicker({ form, disabled, onChange }: OpPickerProps) {
  const [names, setNames] = useState<string[]>([]);
  const service = form.service.trim();

  useEffect(() => {
    if (!service) {
      setNames([]);
      return;
    }
    const ac = new AbortController();
    void (async () => {
      try {
        const rows = await listOperations(service, form.start, form.end, ac.signal);
        if (ac.signal.aborted) {
          return;
        }
        setNames(rows.map((r) => r.operation).filter(Boolean));
      } catch {
        if (ac.signal.aborted) {
          return;
        }
        setNames([]);
      }
    })();
    return () => ac.abort();
  }, [service, form.start, form.end]);

  return (
    <NamePicker
      label="op"
      name="op"
      menuId="op-menu"
      value={form.op}
      names={names}
      disabled={disabled}
      placeholder={service ? "any" : "select a service"}
      onChange={onChange}
      onPick={onChange}
    />
  );
}
