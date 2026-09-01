export type SearchForm = {
  service: string;
  op: string;
  min: string;
  status: string;
  start: string;
  end: string;
  limit: string;
};

export function toRFC3339(d: Date): string {
  return d.toISOString().replace(/\.\d{3}Z$/, "Z");
}

/** Same default as RASAT_QUERY_MAX_WINDOW. GetTrace rejects a wider span. */
const queryMaxWindowMs = 168 * 60 * 60 * 1000;
const traceGetSlackMs = 60_000;

/**
 * Window for GET /api/traces/:id. Search `end` is a snapshot from last submit;
 * a live row can have timestamp >= that end, which ClickHouse treats as not found.
 */
export function coverTraceWindow(
  form: SearchForm,
  row?: { timestamp: string; duration_ns?: number },
  now = new Date(),
): { start: string; end: string } {
  const nowMs = now.getTime();
  let start = Date.parse(form.start.trim());
  let end = Date.parse(form.end.trim());
  if (!Number.isFinite(start) || !Number.isFinite(end)) {
    return { start: form.start.trim(), end: form.end.trim() };
  }
  if (row) {
    const ts = Date.parse(row.timestamp);
    if (Number.isFinite(ts)) {
      if (ts < start) {
        start = ts;
      }
      const durMs =
        typeof row.duration_ns === "number" && Number.isFinite(row.duration_ns) ? row.duration_ns / 1e6 : 0;
      const rowEnd = ts + Math.max(durMs, 0) + traceGetSlackMs;
      if (rowEnd > end) {
        end = rowEnd;
      }
    }
  }
  if (nowMs >= start && nowMs + traceGetSlackMs > end) {
    end = nowMs + traceGetSlackMs;
  }
  if (end - start > queryMaxWindowMs) {
    start = end - queryMaxWindowMs;
  }
  if (end <= start) {
    end = start + traceGetSlackMs;
  }
  return { start: toRFC3339(new Date(start)), end: toRFC3339(new Date(end)) };
}

export function defaultForm(now = new Date()): SearchForm {
  const end = now;
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000);
  return {
    service: "",
    op: "",
    min: "",
    status: "",
    start: toRFC3339(start),
    end: toRFC3339(end),
    limit: "50",
  };
}

export function formFromSearchParams(q: URLSearchParams, now = new Date()): SearchForm {
  const d = defaultForm(now);
  return {
    service: q.get("service") ?? d.service,
    op: q.get("op") ?? d.op,
    min: q.get("min") ?? d.min,
    status: q.get("status") ?? d.status,
    start: q.get("start") ?? d.start,
    end: q.get("end") ?? d.end,
    limit: q.get("limit") ?? d.limit,
  };
}

/** Jump to traces for a service: keep the time window, drop op / min / status / trace. */
export function formForService(name: string, q: URLSearchParams, now = new Date()): SearchForm {
  const current = formFromSearchParams(q, now);
  return {
    ...defaultForm(now),
    start: current.start,
    end: current.end,
    limit: current.limit || "50",
    service: name.trim(),
  };
}

/** Query string for GET /api/traces. Empty filters are omitted. */
export function buildSearchParams(form: SearchForm): URLSearchParams {
  const p = new URLSearchParams();
  p.set("start", form.start.trim());
  p.set("end", form.end.trim());
  p.set("limit", form.limit.trim() || "50");
  const service = form.service.trim();
  if (service) {
    p.set("service", service);
  }
  const op = form.op.trim();
  if (op) {
    p.set("op", op);
  }
  const min = form.min.trim();
  if (min) {
    p.set("min_duration", min);
  }
  const status = form.status.trim();
  if (status) {
    p.set("status", status);
  }
  return p;
}

/** Browser URL: same fields, plus min as `min` for the form. */
export function buildPageParams(form: SearchForm): URLSearchParams {
  const p = new URLSearchParams();
  p.set("start", form.start.trim());
  p.set("end", form.end.trim());
  p.set("limit", form.limit.trim() || "50");
  const service = form.service.trim();
  if (service) {
    p.set("service", service);
  }
  const op = form.op.trim();
  if (op) {
    p.set("op", op);
  }
  const min = form.min.trim();
  if (min) {
    p.set("min", min);
  }
  const status = form.status.trim();
  if (status) {
    p.set("status", status);
  }
  return p;
}

export function pageTraceID(q: URLSearchParams = new URLSearchParams(window.location.search)): string {
  return (q.get("trace") ?? "").trim().toLowerCase();
}

export function writePageURL(form: SearchForm, traceID = ""): void {
  const p = buildPageParams(form);
  const id = traceID.trim().toLowerCase();
  if (id) {
    p.set("trace", id);
  }
  const qs = p.toString();
  const next = qs ? `${window.location.pathname}?${qs}` : window.location.pathname;
  window.history.replaceState(null, "", next);
}

export type LogForm = {
  service: string;
  level: string;
  trace_id: string;
  start: string;
  end: string;
  limit: string;
};

export function defaultLogForm(now = new Date(), traceID = ""): LogForm {
  const d = defaultForm(now);
  return {
    service: "",
    level: "",
    trace_id: traceID.trim(),
    start: d.start,
    end: d.end,
    limit: d.limit,
  };
}
