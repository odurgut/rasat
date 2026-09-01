import { formatTimestamp } from "./format";
import { serviceRampIndex } from "./color";
import type { ServiceRow } from "./api";
import { Trunc } from "./trunc";

type ServiceListProps = {
  rows: ServiceRow[];
  selected: string;
  onSelect: (service: string) => void;
};

export function ServiceList({ rows, selected, onSelect }: ServiceListProps) {
  if (rows.length === 0) {
    return <p className="empty">no data</p>;
  }
  return (
    <div className="list">
      {rows.map((row) => (
        <button
          key={row.service}
          type="button"
          className={selected === row.service ? "list-row is-active" : "list-row"}
          onClick={() => onSelect(row.service)}
        >
          <span className="list-title">
            <span className={`svc-swatch svc-${serviceRampIndex(row.service)}`} aria-hidden="true" />
            <Trunc text={row.service} />
            {row.errors > 0 ? <span className="list-err">ERR</span> : null}
          </span>
          <span className="list-dur">{row.spans}</span>
          <span className="list-meta">{formatTimestamp(row.last_seen)}</span>
        </button>
      ))}
    </div>
  );
}
