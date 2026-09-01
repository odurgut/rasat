import { useState, type ReactNode } from "react";
import { serviceRampIndex } from "./color";
import { Trunc } from "./trunc";

export type TabRow = {
  key: string;
  name: string;
  value: ReactNode;
  onClick: () => void;
};

export type CardTab = {
  id: string;
  label: string;
  empty: string;
  swatch?: boolean;
  rows: TabRow[];
};

export function TabCard({ tabs }: { tabs: CardTab[] }) {
  const [id, setId] = useState(tabs[0]?.id ?? "");
  const active = tabs.find((t) => t.id === id) ?? tabs[0];
  if (!active) {
    return null;
  }
  return (
    <section className="card">
      <div className="card-tabs" role="tablist">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={t.id === active.id}
            className={t.id === active.id ? "card-tab is-active" : "card-tab"}
            onClick={() => setId(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div className="card-tab-panels">
        {tabs.map((t) => {
          const on = t.id === active.id;
          const swatch = t.swatch !== false;
          return (
            <div
              key={t.id}
              className={on ? "card-tab-panel" : "card-tab-panel is-idle"}
              role="tabpanel"
              aria-hidden={!on}
            >
              {t.rows.length === 0 ? (
                <p className="empty">{t.empty}</p>
              ) : (
                t.rows.map((row) => (
                  <button key={row.key} type="button" className="insight-row" onClick={row.onClick} tabIndex={on ? 0 : -1}>
                    {swatch ? <span className={`svc-swatch svc-${serviceRampIndex(row.name)}`} aria-hidden="true" /> : null}
                    <span className="insight-op">
                      <Trunc text={row.name} />
                    </span>
                    <span className="insight-dur">{row.value}</span>
                  </button>
                ))
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}
