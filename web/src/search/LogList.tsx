import { formatClock } from "./format";
import type { LogRow } from "./api";
import { Trunc } from "./trunc";

export function logRowKey(row: LogRow): string {
  return `${row.timestamp}\t${row.trace_id}\t${row.service}\t${row.message}`;
}

type LogListProps = {
  rows: LogRow[];
  selected: LogRow | null;
  enterKeys?: ReadonlySet<string>;
  onOpen: (row: LogRow) => void;
};

export function levelClass(level: string): string {
  switch ((level || "").toUpperCase()) {
    case "ERROR":
    case "FATAL":
    case "PANIC":
      return "tok-err";
    case "WARN":
    case "WARNING":
      return "tok-warn";
    default:
      return "tok-info";
  }
}

export function sameLog(a: LogRow | null, b: LogRow): boolean {
  return Boolean(
    a &&
      a.timestamp === b.timestamp &&
      a.message === b.message &&
      a.trace_id === b.trace_id &&
      a.service === b.service,
  );
}

export function LogList({ rows, selected, enterKeys, onOpen }: LogListProps) {
  if (rows.length === 0) {
    return <p className="empty">no data</p>;
  }
  return (
    <div className="log-stream">
      {rows.map((row, i) => (
        <LogStreamRow
          key={`${logRowKey(row)}-${i}`}
          row={row}
          selected={sameLog(selected, row)}
          enter={Boolean(enterKeys?.has(logRowKey(row)))}
          onOpen={() => onOpen(row)}
        />
      ))}
    </div>
  );
}

function LogStreamRow({
  row,
  selected,
  enter,
  onOpen,
}: {
  row: LogRow;
  selected: boolean;
  enter: boolean;
  onOpen: () => void;
}) {
  const level = (row.level || "INFO").toUpperCase();
  return (
    <div className={enter ? "row-enter is-enter" : "row-enter"}>
      <button
        type="button"
        className={selected ? "log-stream-row is-active" : "log-stream-row"}
        onClick={onOpen}
      >
      <span className="log-stream-time">{formatClock(row.timestamp)}</span>
      <span className={levelClass(level)}>{level}</span>
      <span className="log-stream-msg">
        <Trunc text={row.message || "(empty)"} />
      </span>
    </button>
    </div>
  );
}
