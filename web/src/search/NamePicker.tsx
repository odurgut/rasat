import { useEffect, useMemo, useRef, useState } from "react";
import { serviceRampIndex } from "./color";
import { Trunc } from "./trunc";

export type NamePickerProps = {
  label: string;
  name: string;
  value: string;
  names: string[];
  disabled: boolean;
  placeholder?: string;
  menuId: string;
  swatch?: boolean;
  onChange: (next: string) => void;
  onPick: (next: string) => void;
};

export function NamePicker({
  label,
  name,
  value,
  names,
  disabled,
  placeholder = "any",
  menuId,
  swatch = false,
  onChange,
  onPick,
}: NamePickerProps) {
  const [open, setOpen] = useState(false);
  const [hi, setHi] = useState(0);
  const wrap = useRef<HTMLLabelElement>(null);
  const options = useMemo(() => menuOptions(value, names), [value, names]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const onDoc = (e: MouseEvent) => {
      if (wrap.current && !wrap.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  useEffect(() => {
    setHi(0);
  }, [value, open]);

  function pick(next: string) {
    onPick(next);
    setOpen(false);
  }

  return (
    <label ref={wrap} className={open ? "field is-open" : "field"}>
      <span className="field-label">{label}</span>
      <input
        className="field-input"
        name={name}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        autoComplete="off"
        spellCheck={false}
        role="combobox"
        aria-expanded={open}
        aria-controls={menuId}
        aria-autocomplete="list"
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          onChange(e.target.value);
          setOpen(true);
        }}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            e.preventDefault();
            setOpen(false);
            return;
          }
          if (e.key === "ArrowDown") {
            e.preventDefault();
            if (!open) {
              setOpen(true);
              return;
            }
            setHi((i) => Math.min(i + 1, Math.max(options.length - 1, 0)));
            return;
          }
          if (e.key === "ArrowUp") {
            e.preventDefault();
            setHi((i) => Math.max(i - 1, 0));
            return;
          }
          if (e.key === "Enter" && open && options[hi]) {
            e.preventDefault();
            pick(options[hi].value);
          }
        }}
      />
      {open && !disabled ? (
        <div id={menuId} className="field-menu" role="listbox">
          {options.map((opt, i) => (
            <button
              key={opt.value || "any"}
              type="button"
              role="option"
              aria-selected={opt.value === value}
              className={rowClass(i === hi, opt.value === value, !opt.value)}
              onMouseEnter={() => setHi(i)}
              onClick={() => pick(opt.value)}
            >
              {swatch && opt.value ? (
                <span className={`svc-swatch svc-${serviceRampIndex(opt.value)}`} aria-hidden="true" />
              ) : null}
              <Trunc text={opt.label} />
            </button>
          ))}
        </div>
      ) : null}
    </label>
  );
}

type MenuOpt = { value: string; label: string };

function menuOptions(value: string, names: string[]): MenuOpt[] {
  const q = value.trim().toLowerCase();
  const exact = names.some((n) => n.toLowerCase() === q);
  const shown = !q || exact ? names : names.filter((n) => n.toLowerCase().includes(q));
  const out: MenuOpt[] = [{ value: "", label: "any" }];
  const seen = new Set<string>();
  for (const n of shown) {
    if (seen.has(n)) {
      continue;
    }
    seen.add(n);
    out.push({ value: n, label: n });
  }
  return out;
}

function rowClass(hi: boolean, selected: boolean, empty: boolean): string {
  const cls = ["field-menu-row"];
  if (hi) {
    cls.push("is-hi");
  }
  if (selected) {
    cls.push("is-active");
  }
  if (empty) {
    cls.push("is-any");
  }
  return cls.join(" ");
}
