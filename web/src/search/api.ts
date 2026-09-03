import { buildSearchParams, coverTraceWindow, type LogForm, type SearchForm } from "./query";

export type TraceListRow = {
  trace_id: string;
  service: string;
  operation: string;
  duration_ns: number;
  span_count: number;
  timestamp: string;
  status_code: number;
};

export type SpanEvent = {
  time: string;
  name: string;
  attributes: Record<string, string>;
};

export type SpanLink = {
  trace_id: string;
  span_id: string;
  attributes: Record<string, string>;
};

export type SpanDetail = {
  timestamp: string;
  span_id: string;
  parent_span_id: string;
  service: string;
  operation: string;
  kind: number;
  duration_ns: number;
  /** Nanoseconds from the trace start. Preferred for waterfall placement. */
  start_offset_ns?: number;
  status_code: number;
  status_message: string;
  scope_name: string;
  scope_version: string;
  resource_attributes: Record<string, string>;
  span_attributes: Record<string, string>;
  events: SpanEvent[];
  links: SpanLink[];
};

export type CriticalPathStep = {
  span_id: string;
  service: string;
  operation: string;
  duration_ns: number;
};

export type Bottleneck = {
  span_id: string;
  service: string;
  operation: string;
  exclusive_ns: number;
};

export type TraceDetail = {
  trace_id: string;
  timestamp: string;
  duration_ns: number;
  span_count: number;
  critical_path?: CriticalPathStep[];
  critical_path_ns?: number;
  bottlenecks?: Bottleneck[];
  spans: SpanDetail[];
};

type errorBody = {
  error?: string;
};

export class SearchError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SearchError";
  }
}

export async function searchTraces(form: SearchForm, signal?: AbortSignal): Promise<TraceListRow[]> {
  const body = await fetchJSON<{ traces?: TraceListRow[] }>(`/api/traces?${buildSearchParams(form).toString()}`, signal);
  return body.traces ?? [];
}

export async function getTrace(
  id: string,
  form: SearchForm,
  signal?: AbortSignal,
  row?: Pick<TraceListRow, "timestamp" | "duration_ns">,
): Promise<TraceDetail> {
  const w = coverTraceWindow(form, row);
  const p = new URLSearchParams();
  p.set("start", w.start);
  p.set("end", w.end);
  return fetchJSON<TraceDetail>(`/api/traces/${encodeURIComponent(id)}?${p.toString()}`, signal);
}

export type ServiceRow = {
  service: string;
  last_seen: string;
  spans: number;
  errors: number;
};

export async function listServices(form: SearchForm, signal?: AbortSignal): Promise<ServiceRow[]> {
  const p = new URLSearchParams();
  p.set("start", form.start.trim());
  p.set("end", form.end.trim());
  p.set("limit", form.limit.trim() || "100");
  const body = await fetchJSON<{ services?: ServiceRow[] }>(`/api/services?${p.toString()}`, signal);
  return body.services ?? [];
}

export type OperationRow = {
  operation: string;
  spans: number;
  errors: number;
  error_rate: number;
  p50_ns: number;
  p95_ns: number;
};

export async function listOperations(
  service: string,
  start: string,
  end: string,
  signal?: AbortSignal,
  limit = "100",
): Promise<OperationRow[]> {
  const p = new URLSearchParams();
  p.set("service", service.trim());
  p.set("start", start.trim());
  p.set("end", end.trim());
  p.set("limit", limit.trim() || "100");
  const body = await fetchJSON<{ operations?: OperationRow[] }>(`/api/operations?${p.toString()}`, signal);
  return (body.operations ?? []).map((r) => ({
    operation: r.operation,
    spans: r.spans ?? 0,
    errors: r.errors ?? 0,
    error_rate: r.error_rate ?? 0,
    p50_ns: r.p50_ns ?? 0,
    p95_ns: r.p95_ns ?? 0,
  }));
}

export type ServiceMapNode = {
  service: string;
  spans: number;
  errors: number;
};

export type ServiceMapEdge = {
  from: string;
  to: string;
  calls: number;
  errors: number;
  avg_duration_ns: number;
};

export type ServiceMapGraph = {
  nodes: ServiceMapNode[];
  edges: ServiceMapEdge[];
};

export async function getServiceMap(form: SearchForm, signal?: AbortSignal): Promise<ServiceMapGraph> {
  const p = new URLSearchParams();
  p.set("start", form.start.trim());
  p.set("end", form.end.trim());
  p.set("limit", form.limit.trim() || "100");
  const body = await fetchJSON<ServiceMapGraph>(`/api/service-map?${p.toString()}`, signal);
  return { nodes: body.nodes ?? [], edges: body.edges ?? [] };
}

export type LogRow = {
  timestamp: string;
  service: string;
  level: string;
  message: string;
  trace_id: string;
  span_id: string;
};

export function buildLogParams(form: LogForm): URLSearchParams {
  const p = new URLSearchParams();
  p.set("start", form.start.trim());
  p.set("end", form.end.trim());
  p.set("limit", form.limit.trim() || "50");
  const service = form.service.trim();
  if (service) {
    p.set("service", service);
  }
  const level = form.level.trim();
  if (level) {
    p.set("level", level);
  }
  const traceID = form.trace_id.trim();
  if (traceID) {
    p.set("trace_id", traceID);
  }
  return p;
}

export async function searchLogs(form: LogForm, signal?: AbortSignal): Promise<LogRow[]> {
  const body = await fetchJSON<{ logs?: LogRow[] }>(`/api/logs?${buildLogParams(form).toString()}`, signal);
  return body.logs ?? [];
}

export type ServiceMetrics = {
  service: string;
  spans: number;
  errors: number;
  rate: number;
  error_rate: number;
  avg_ns: number;
  p50_ns: number;
  p95_ns: number;
  p99_ns: number;
};

export type MetricPoint = {
  t: string;
  spans: number;
  errors: number;
  rate: number;
  error_rate: number;
  avg_ns: number;
  p50_ns: number;
  p95_ns: number;
  p99_ns: number;
};

export type ServiceSeries = {
  service: string;
  points: MetricPoint[];
};

export type MetricsResponse = {
  window_s: number;
  step_s: number;
  metrics: ServiceMetrics[];
  series: ServiceSeries[];
};

export async function getMetrics(
  opts: { start: string; end: string; limit: string; service?: string; step?: string },
  signal?: AbortSignal,
): Promise<MetricsResponse> {
  const p = new URLSearchParams();
  p.set("start", opts.start.trim());
  p.set("end", opts.end.trim());
  p.set("limit", opts.limit.trim() || "50");
  const service = (opts.service ?? "").trim();
  if (service) {
    p.set("service", service);
  }
  const step = (opts.step ?? "").trim();
  if (step) {
    p.set("step", step);
  }
  const body = await fetchJSON<MetricsResponse>(`/api/metrics?${p.toString()}`, signal);
  return {
    window_s: body.window_s ?? 0,
    step_s: body.step_s ?? 0,
    metrics: body.metrics ?? [],
    series: body.series ?? [],
  };
}

export type ErrorCause = {
  cause: string;
  count: number;
  first_seen?: string;
};

export async function getErrorCauses(
  opts: { start: string; end: string; limit: string; service?: string },
  signal?: AbortSignal,
): Promise<ErrorCause[]> {
  const p = new URLSearchParams();
  p.set("start", opts.start.trim());
  p.set("end", opts.end.trim());
  p.set("limit", opts.limit.trim() || "5");
  const service = (opts.service ?? "").trim();
  if (service) {
    p.set("service", service);
  }
  const body = await fetchJSON<{ causes?: ErrorCause[] }>(`/api/error-causes?${p.toString()}`, signal);
  return body.causes ?? [];
}

export type BuildInfo = {
  version: string;
  commit: string;
};

export async function getBuild(signal?: AbortSignal): Promise<BuildInfo | null> {
  try {
    const res = await fetch("/version", { signal });
    if (!res.ok) {
      return null;
    }
    const body = (await res.json()) as { version?: unknown; commit?: unknown };
    if (typeof body.version !== "string" || body.version === "") {
      return null;
    }
    return {
      version: body.version,
      commit: typeof body.commit === "string" ? body.commit : "",
    };
  } catch {
    return null;
  }
}

async function fetchJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  const ac = new AbortController();
  const timer = window.setTimeout(() => ac.abort(), 15_000);
  const onAbort = () => ac.abort();
  signal?.addEventListener("abort", onAbort, { once: true });
  try {
    const res = await fetch(url, { signal: ac.signal });
    let body: T & errorBody = {} as T & errorBody;
    try {
      body = (await res.json()) as T & errorBody;
    } catch {
      if (!res.ok) {
        throw new SearchError(res.statusText || "storage unavailable");
      }
      throw new SearchError("invalid response");
    }
    if (!res.ok) {
      throw new SearchError(body.error || res.statusText || "request failed");
    }
    return body;
  } catch (e) {
    if (e instanceof SearchError) {
      throw e;
    }
    if (e instanceof DOMException && e.name === "AbortError") {
      if (signal?.aborted) {
        throw e;
      }
      throw new SearchError("request timed out");
    }
    throw new SearchError(e instanceof Error ? e.message : "request failed");
  } finally {
    window.clearTimeout(timer);
    signal?.removeEventListener("abort", onAbort);
  }
}
