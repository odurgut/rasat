import type { LogRow, TraceListRow } from "./api";
import type { LogForm, SearchForm } from "./query";

const units: Record<string, number> = {
  ns: 1,
  us: 1_000,
  µs: 1_000,
  μs: 1_000,
  ms: 1_000_000,
  s: 1_000_000_000,
  m: 60_000_000_000,
  h: 3_600_000_000_000,
};

/** Go duration strings used by the search form (`50ms`, `1s`). */
export function parseGoDurationNs(raw: string): number | null {
  const s = raw.trim();
  if (!s) {
    return null;
  }
  const m = s.match(/^(\d+(?:\.\d+)?)(ns|us|µs|μs|ms|s|m|h)$/);
  if (!m) {
    return null;
  }
  const n = Number(m[1]);
  const unit = m[2];
  if (!Number.isFinite(n) || n < 0 || unit === undefined) {
    return null;
  }
  const mul = units[unit];
  if (mul === undefined) {
    return null;
  }
  return n * mul;
}

function statusCode(raw: string): number | null {
  switch (raw.trim().toLowerCase()) {
    case "":
      return null;
    case "unset":
    case "0":
      return 0;
    case "ok":
    case "1":
      return 1;
    case "error":
    case "err":
    case "2":
      return 2;
    default:
      return NaN;
  }
}

export function isLiveTail(endISO: string, now = Date.now()): boolean {
  const end = Date.parse(endISO);
  return Number.isFinite(end) && end >= now - 5_000;
}

/** Client-side match for a streamed list row against the current search form. */
export function rowMatchesForm(row: TraceListRow, form: SearchForm, liveTail: boolean): boolean {
  const service = form.service.trim();
  if (service && row.service !== service) {
    return false;
  }
  const op = form.op.trim();
  if (op && row.operation !== op) {
    return false;
  }
  const min = form.min.trim();
  if (min) {
    const ns = parseGoDurationNs(min);
    if (ns === null || row.duration_ns < ns) {
      return false;
    }
  }
  const wantStatus = statusCode(form.status);
  if (wantStatus !== null) {
    if (!Number.isFinite(wantStatus) || row.status_code !== wantStatus) {
      return false;
    }
  }
  const ts = Date.parse(row.timestamp);
  if (!Number.isFinite(ts)) {
    return false;
  }
  const start = Date.parse(form.start);
  if (Number.isFinite(start) && ts < start) {
    return false;
  }
  if (!liveTail) {
    const end = Date.parse(form.end);
    if (Number.isFinite(end) && ts >= end) {
      return false;
    }
  }
  return true;
}

export function tracesStreamURL(loc: Pick<Location, "protocol" | "host"> = window.location): string {
  const proto = loc.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${loc.host}/api/stream/traces`;
}

export function logsStreamURL(loc: Pick<Location, "protocol" | "host"> = window.location): string {
  const proto = loc.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${loc.host}/api/stream/logs`;
}

export function parseStreamRow(raw: string): TraceListRow | null {
  try {
    const v = JSON.parse(raw) as TraceListRow;
    if (!v || typeof v.trace_id !== "string" || !v.trace_id) {
      return null;
    }
    return v;
  } catch {
    return null;
  }
}

export function prependLiveRow(rows: TraceListRow[], row: TraceListRow, limit: number): TraceListRow[] {
  const rest = rows.filter((r) => r.trace_id !== row.trace_id);
  const cap = Number.isFinite(limit) && limit > 0 ? limit : 50;
  return [row, ...rest].slice(0, cap);
}

/** pending is newest-first. Apply oldest first so prepend keeps newest at top. */
export function flushPending(rows: TraceListRow[], pending: TraceListRow[], limit: number): TraceListRow[] {
  let next = rows;
  for (let i = pending.length - 1; i >= 0; i--) {
    const row = pending[i];
    if (row) {
      next = prependLiveRow(next, row, limit);
    }
  }
  return next;
}

export function isListAtTop(el: HTMLElement | null, slop = 8): boolean {
  return !el || el.scrollTop <= slop;
}

export function parseStreamLog(raw: string): LogRow | null {
  try {
    const v = JSON.parse(raw) as LogRow;
    if (!v || typeof v.message !== "string" || typeof v.service !== "string") {
      return null;
    }
    return v;
  } catch {
    return null;
  }
}

export function logMatchesForm(row: LogRow, form: LogForm, liveTail: boolean): boolean {
  const service = form.service.trim();
  if (service && row.service !== service) {
    return false;
  }
  const level = form.level.trim().toUpperCase();
  if (level && (row.level || "").toUpperCase() !== level) {
    return false;
  }
  const traceID = form.trace_id.trim();
  if (traceID && row.trace_id !== traceID) {
    return false;
  }
  const ts = Date.parse(row.timestamp);
  if (!Number.isFinite(ts)) {
    return false;
  }
  const start = Date.parse(form.start);
  if (Number.isFinite(start) && ts < start) {
    return false;
  }
  if (!liveTail) {
    const end = Date.parse(form.end);
    if (Number.isFinite(end) && ts >= end) {
      return false;
    }
  }
  return true;
}

export function prependLiveLog(rows: LogRow[], row: LogRow, limit: number): LogRow[] {
  const cap = Number.isFinite(limit) && limit > 0 ? limit : 50;
  return [row, ...rows].slice(0, cap);
}

export function flushPendingLogs(rows: LogRow[], pending: LogRow[], limit: number): LogRow[] {
  let next = rows;
  for (let i = pending.length - 1; i >= 0; i--) {
    const row = pending[i];
    if (row) {
      next = prependLiveLog(next, row, limit);
    }
  }
  return next;
}
