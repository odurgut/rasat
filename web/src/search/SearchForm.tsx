import type { SearchForm } from "./query";
import { PrimaryButton } from "../chrome/PrimaryButton";
import { GhostButton } from "../chrome/GhostButton";
import { ServicePicker } from "./ServicePicker";
import { OpPicker } from "./OpPicker";

type SearchFormProps = {
  form: SearchForm;
  disabled: boolean;
  onChange: (next: SearchForm) => void;
  onSubmit: () => void;
  onReset: () => void;
  wide?: boolean;
};

const fields: { key: Exclude<keyof SearchForm, "service" | "op">; label: string; placeholder: string }[] = [
  { key: "min", label: "min", placeholder: "50ms" },
  { key: "status", label: "status", placeholder: "error" },
  { key: "start", label: "start", placeholder: "2026-08-26T00:00:00Z" },
  { key: "end", label: "end", placeholder: "2026-08-27T00:00:00Z" },
  { key: "limit", label: "limit", placeholder: "50" },
];

export function SearchFormFields({ form, disabled, onChange, onSubmit, onReset, wide = false }: SearchFormProps) {
  return (
    <form
      className={wide ? "search-form is-wide" : "search-form"}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <ServicePicker
        form={form}
        disabled={disabled}
        onChange={(service) => onChange({ ...form, service })}
        onPick={(service) => onChange({ ...form, service, op: "" })}
      />
      <OpPicker form={form} disabled={disabled} onChange={(op) => onChange({ ...form, op })} />
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
