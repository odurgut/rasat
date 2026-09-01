import type { LogForm } from "./query";
import { PrimaryButton } from "../chrome/PrimaryButton";
import { GhostButton } from "../chrome/GhostButton";
import { ServicePicker } from "./ServicePicker";

type LogFilterProps = {
  form: LogForm;
  disabled: boolean;
  onChange: (next: LogForm) => void;
  onSubmit: () => void;
  onReset: () => void;
};

const fields: { key: Exclude<keyof LogForm, "service">; label: string; placeholder: string }[] = [
  { key: "level", label: "level", placeholder: "ERROR" },
  { key: "trace_id", label: "trace", placeholder: "abc123" },
  { key: "start", label: "start", placeholder: "2026-08-26T00:00:00Z" },
  { key: "end", label: "end", placeholder: "2026-08-27T00:00:00Z" },
  { key: "limit", label: "limit", placeholder: "50" },
];

export function LogFilterFields({ form, disabled, onChange, onSubmit, onReset }: LogFilterProps) {
  return (
    <form
      className="search-form"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <ServicePicker
        form={{
          service: form.service,
          op: "",
          min: "",
          status: "",
          start: form.start,
          end: form.end,
          limit: form.limit,
        }}
        disabled={disabled}
        onChange={(service) => onChange({ ...form, service })}
        onPick={(service) => onChange({ ...form, service })}
      />
      {fields.map((f) => (
        <label key={f.key} className="field">
          <span className="field-label">{f.label}</span>
          <input
            className="field-input"
            name={f.key}
            value={form[f.key]}
            placeholder={f.placeholder}
            disabled={disabled}
            autoComplete="off"
            spellCheck={false}
            onChange={(e) => onChange({ ...form, [f.key]: e.target.value })}
          />
        </label>
      ))}
      <p className="search-actions">
        <PrimaryButton label="search" type="submit" disabled={disabled} />
        <GhostButton label="reset" disabled={disabled} onClick={onReset} />
      </p>
    </form>
  );
}
