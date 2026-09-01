import { formatClock, formatDuration, formatShortID } from "./format";
import { serviceRampIndex } from "./color";
import type { TraceListRow } from "./api";
import { Trunc } from "./trunc";

type TraceListProps = {
  rows: TraceListRow[];
  selectedID: string;
  enterIDs?: ReadonlySet<string>;
  onOpen: (row: TraceListRow) => void;
};

export function TraceList({ rows, selectedID, enterIDs, onOpen }: TraceListProps) {
  if (rows.length === 0) {
    return <p className="empty">no data</p>;
  }

  return (
    <div className="data-table">
      <div className="data-table-head">
        <span>time</span>
        <span>service</span>
        <span>operation</span>
        <span>spans</span>
        <span>duration</span>
        <span>id</span>
      </div>
      {rows.map((row, i) => (
        <TraceRow
          key={row.trace_id || `${row.timestamp}-${i}`}
          row={row}
          selected={row.trace_id === selectedID}
          enter={Boolean(enterIDs?.has(row.trace_id))}
          onOpen={() => onOpen(row)}
        />
      ))}
    </div>
  );
}

function TraceRow({
  row,
  selected,
  enter,
  onOpen,
}: {
  row: TraceListRow;
  selected: boolean;
  enter: boolean;
  onOpen: () => void;
}) {
  const err = row.status_code === 2;
  const title = row.operation || "(no op)";
  const service = row.service || "(unknown)";
  return (
    <div className={enter ? "row-enter is-enter" : "row-enter"}>
      <button
        type="button"
        className={["data-table-row", selected ? "is-active" : "", err ? "is-err" : ""].filter(Boolean).join(" ")}
        onClick={onOpen}
      >
      <span className="data-table-num is-muted">{formatClock(row.timestamp)}</span>
      <span className="data-table-svc">
        <span className={`svc-swatch svc-${serviceRampIndex(service)}`} aria-hidden="true" />
        <Trunc text={service} />
      </span>
      <span className="data-table-op">
        {err ? <span className="list-err">ERR</span> : null}
        <Trunc text={title} />
      </span>
      <span className="data-table-num">{row.span_count}</span>
      <span className="data-table-num">{formatDuration(row.duration_ns)}</span>
        <span className="data-table-id">{formatShortID(row.trace_id)}</span>
      </button>
    </div>
  );
}
