import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { SearchError, getErrorCauses, getMetrics, type ErrorCause, type MetricPoint, type ServiceMetrics } from "./api";
import { formatClock, formatDeltaPct, formatDuration, formatPct, formatRate } from "./format";
import { GhostButton } from "../chrome/GhostButton";
import { defaultForm, formFromSearchParams, slidingWindow, toRFC3339, writePageURL, type SearchForm } from "./query";
import { serviceRampIndex } from "./color";
import { Trunc } from "./trunc";
import { DashChart } from "./DashChart";
import { ActivityFeed } from "./ActivityFeed";
import type { ChartSeries } from "./chart";
import { TabCard } from "./TabCard";

type OverviewViewProps = {
  active: boolean;
  onOpenService: (service: string) => void;
  onOpenTraces: (form: SearchForm) => void;
  onOpenTrace: (traceID: string) => void;
};

const RANGE_PRESETS = [
  { label: "1h", ms: 60 * 60 * 1000, step: "1m" },
  { label: "6h", ms: 6 * 60 * 60 * 1000, step: "5m" },
  { label: "24h", ms: 24 * 60 * 60 * 1000, step: "10m" },
  { label: "7d", ms: 7 * 24 * 60 * 60 * 1000, step: "1h" },
] as const;

type RangePreset = (typeof RANGE_PRESETS)[number];

const minRise = 1.1;
const METRICS_POLL_MS = 15_000;

export function OverviewView({ active, onOpenService, onOpenTraces, onOpenTrace }: OverviewViewProps) {
  const initial = useMemo(() => windowFromSearch(), []);
  const [range, setRange] = useState<RangePreset>(initial.preset);
  const [start, setStart] = useState(initial.start);
  const [end, setEnd] = useState(initial.end);
  const [now, setNow] = useState<ServiceMetrics[]>([]);
  const [firstHalf, setFirstHalf] = useState<ServiceMetrics[]>([]);
  const [secondHalf, setSecondHalf] = useState<ServiceMetrics[]>([]);
  const [causes, setCauses] = useState<ErrorCause[]>([]);
  const [points, setPoints] = useState<MetricPoint[]>([]);
  const [windowS, setWindowS] = useState(0);
  const [status, setStatus] = useState<"loading" | "ok" | "error">("loading");
  const [error, setError] = useState("");
  const seq = useRef(0);
  const statusRef = useRef(status);
  const rangeRef = useRef(range);
  const rangeLabelRef = useRef(range.label);
  const fetching = useRef(false);
  const wasActive = useRef(active);
  statusRef.current = status;
  rangeRef.current = range;

  useEffect(() => {
    const ac = new AbortController();
    const n = ++seq.current;
    fetching.current = true;
    const rangeChanged = rangeLabelRef.current !== range.label;
    rangeLabelRef.current = range.label;
    const silent = statusRef.current === "ok" && !rangeChanged;
    if (!silent) {
      setStatus("loading");
    }
    void (async () => {
      try {
        const halves = splitWindow(start, end);
        const none = { metrics: [] as ServiceMetrics[] };
        const [curBody, firstBody, secondBody, found] = await Promise.all([
          getMetrics({ start, end, limit: "100", step: range.step }, ac.signal),
          halves
            ? getMetrics({ start: halves.first.start, end: halves.first.end, limit: "100" }, ac.signal)
            : Promise.resolve(none),
          halves
            ? getMetrics({ start: halves.second.start, end: halves.second.end, limit: "100" }, ac.signal)
            : Promise.resolve(none),
          getErrorCauses({ start, end, limit: "8" }, ac.signal),
        ]);
        if (n !== seq.current) {
          return;
        }
        setNow(curBody.metrics);
        setFirstHalf(firstBody.metrics);
        setSecondHalf(secondBody.metrics);
        setCauses(found);
        setWindowS(curBody.window_s);
        setPoints(curBody.series[0]?.points ?? []);
        setStatus("ok");
        setError("");
      } catch (e) {
        if (n !== seq.current) {
          return;
        }
        if (e instanceof DOMException && e.name === "AbortError") {
          return;
        }
        if (silent) {
          return;
        }
        const msg = e instanceof SearchError || e instanceof Error ? e.message : "load failed";
        setNow([]);
        setFirstHalf([]);
        setSecondHalf([]);
        setCauses([]);
        setPoints([]);
        setWindowS(0);
        setError(msg);
        setStatus("error");
      } finally {
        if (n === seq.current) {
          fetching.current = false;
        }
      }
    })();
    return () => ac.abort();
  }, [start, end, range.step, range.label]);

  useEffect(() => {
    if (!active) {
      wasActive.current = false;
      return;
    }
    const entering = !wasActive.current;
    wasActive.current = true;
    const slide = () => {
      if (document.visibilityState === "hidden" || fetching.current) {
        return;
      }
      const win = rangeWindow(rangeRef.current.ms);
      setStart(win.start);
      setEnd(win.end);
    };
    if (entering) {
      slide();
    }
    const id = window.setInterval(slide, METRICS_POLL_MS);
    const onVis = () => {
      if (document.visibilityState === "visible") {
        slide();
      }
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [active]);

  const kpis = useMemo(() => fleetKpis(now, windowS), [now, windowS]);
  const issues = useMemo(
    () =>
      [...now]
        .filter((r) => r.error_rate > 0)
        .sort((a, b) => b.error_rate - a.error_rate || a.service.localeCompare(b.service)),
    [now],
  );
  const prevByName = useMemo(() => {
    const m = new Map<string, ServiceMetrics>();
    for (const r of firstHalf) {
      m.set(r.service, r);
    }
    return m;
  }, [firstHalf]);
  const regressions = useMemo(() => {
    const rows: Regression[] = [];
    for (const cur of secondHalf) {
      const before = prevByName.get(cur.service);
      if (!before || !(before.p95_ns > 0) || cur.p95_ns <= before.p95_ns * minRise) {
        continue;
      }
      rows.push({ service: cur.service, prev_ns: before.p95_ns, curr_ns: cur.p95_ns });
    }
    rows.sort((a, b) => b.curr_ns / b.prev_ns - a.curr_ns / a.prev_ns || a.service.localeCompare(b.service));
    return rows;
  }, [secondHalf, prevByName]);
  const slowest = useMemo(
    () =>
      [...now]
        .filter((r) => r.p95_ns > 0)
        .sort((a, b) => b.p95_ns - a.p95_ns || a.service.localeCompare(b.service))
        .slice(0, 8),
    [now],
  );
  const busiest = useMemo(
    () => [...now].sort((a, b) => b.spans - a.spans || a.service.localeCompare(b.service)).slice(0, 8),
    [now],
  );

  const times = useMemo(
    () =>
      points.map((p) => {
        const ms = Date.parse(p.t);
        return Number.isFinite(ms) ? ms / 1000 : 0;
      }),
    [points],
  );
  const stepS = rangeStepSeconds(range.step);
  const traffic = useMemo<ChartSeries[]>(
    () => [{ label: "RATE", tone: "svc-0", fill: true, values: points.map((p) => p.rate) }],
    [points],
  );
  const errors = useMemo<ChartSeries[]>(
    () => [{ label: "ERR/s", tone: "err", fill: true, values: points.map((p) => (stepS > 0 ? p.errors / stepS : 0)) }],
    [points, stepS],
  );
  const latency = useMemo<ChartSeries[]>(
    () => [
      { label: "p50", tone: "svc-0", values: points.map((p) => (p.spans > 0 ? p.p50_ns : null)) },
      { label: "p95", tone: "svc-1", values: points.map((p) => (p.spans > 0 ? p.p95_ns : null)) },
      { label: "p99", tone: "svc-2", values: points.map((p) => (p.spans > 0 ? p.p99_ns : null)) },
    ],
    [points],
  );

  function applyRange(next: RangePreset): void {
    const nextWin = rangeWindow(next.ms);
    setRange(next);
    setStart(nextWin.start);
    setEnd(nextWin.end);
    const form = formFromSearchParams(new URLSearchParams(window.location.search));
    writePageURL({ ...form, start: nextWin.start, end: nextWin.end }, "");
  }

  function tracesFor(service: string, extra: Partial<SearchForm> = {}): SearchForm {
    return {
      ...defaultForm(),
      start,
      end,
      service,
      op: "",
      min: "",
      status: "",
      limit: "50",
      ...extra,
    };
  }

  return (
    <div className="page">
      <div className="page-head">
        <div className="dash-range">
          {RANGE_PRESETS.map((p) => (
            <GhostButton key={p.label} label={p.label} active={range.label === p.label} onClick={() => applyRange(p)} />
          ))}
        </div>
      </div>
      {status === "error" ? (
        <p className="surface-error">
          <span className="surface-error-word">ERROR</span> {error}
        </p>
      ) : (
        <>
          <div className="card-grid card-kpis">
            <Kpi label="RATE" value={formatRate(kpis.rate)} />
            <Kpi label="ERR/s" value={formatRate(kpis.errS)} err={kpis.errS > 0} />
            <Kpi label="ERR" value={formatPct(kpis.errorRate)} err={kpis.errorRate > 0} />
            <Kpi label="P95" value={formatDuration(kpis.p95)} />
            <Kpi label="P99" value={formatDuration(kpis.p99)} />
            <Kpi label="SPANS" value={String(kpis.spans)} />
          </div>
          <div className="card-grid card-charts">
            <div className="card card-chart">
              <DashChart title="traffic" times={times} series={traffic} formatY={formatRate} />
            </div>
            <div className="card card-chart">
              <DashChart title="errors" times={times} series={errors} formatY={formatRate} />
            </div>
            <div className="card card-chart card-chart-wide">
              <DashChart title="latency" times={times} series={latency} formatY={formatDuration} />
            </div>
          </div>
          <ActivityFeed
            active={active}
            rangeKey={range.label}
            start={start}
            end={end}
            slowNs={kpis.p95}
            onOpen={onOpenTrace}
          />
          <div className="card-grid card-split">
            <TabCard
              tabs={[
                {
                  id: "issues",
                  label: "issues",
                  empty: status === "loading" ? "loading" : "no error rates",
                  rows: issues.map((row) => ({
                    key: row.service,
                    name: row.service,
                    value: (
                      <span className="tok-err">
                        ERR {formatPct(row.error_rate)}
                      </span>
                    ),
                    onClick: () => onOpenService(row.service),
                  })),
                },
                {
                  id: "incidents",
                  label: "incidents",
                  empty: status === "loading" ? "loading" : "no incidents",
                  swatch: false,
                  rows: causes.map((row) => ({
                    key: row.cause,
                    name: row.cause || "error",
                    value: `${row.count}${row.first_seen ? ` · ${formatClock(row.first_seen)}` : ""}`,
                    onClick: () => onOpenTraces(tracesFor("", { status: "error" })),
                  })),
                },
              ]}
            />
            <TabCard
              tabs={[
                {
                  id: "regressions",
                  label: "regressions",
                  empty: status === "loading" ? "loading" : "no p95 rises",
                  rows: regressions.map((row) => {
                    const delta = formatDeltaPct(row.prev_ns, row.curr_ns);
                    return {
                      key: row.service,
                      name: row.service,
                      value: (
                        <>
                          {formatDuration(row.prev_ns)} → {formatDuration(row.curr_ns)}
                          {delta ? (
                            <>
                              {" "}
                              <span className="tok-warn">{delta}</span>
                            </>
                          ) : null}
                        </>
                      ),
                      onClick: () => onOpenService(row.service),
                    };
                  }),
                },
                {
                  id: "slowest",
                  label: "slowest",
                  empty: status === "loading" ? "loading" : "no latency",
                  rows: slowest.map((row) => ({
                    key: row.service,
                    name: row.service,
                    value: formatDuration(row.p95_ns),
                    onClick: () => onOpenService(row.service),
                  })),
                },
              ]}
            />
            <OverviewList
              title="busiest"
              empty={status === "loading" ? "loading" : "no services"}
              rows={busiest.map((row) => ({
                key: row.service,
                name: row.service,
                value: `${row.spans} · ${formatDuration(row.p95_ns)}`,
                onClick: () => onOpenService(row.service),
              }))}
            />
          </div>
        </>
      )}
    </div>
  );
}

type Regression = {
  service: string;
  prev_ns: number;
  curr_ns: number;
};

function Kpi({ label, value, err = false }: { label: string; value: string; err?: boolean }) {
  return (
    <div className="card card-kpi">
      <span className="dash-kpi-label">{label}</span>
      <span className={err ? "dash-kpi-val is-err" : "dash-kpi-val"}>{value}</span>
    </div>
  );
}

function OverviewList({
  title,
  empty,
  rows,
}: {
  title: string;
  empty: string;
  rows: { key: string; name: string; value: ReactNode; onClick: () => void }[];
}) {
  return (
    <section className="card">
      <p className="card-label">{title}</p>
      {rows.length === 0 ? (
        <p className="empty">{empty}</p>
      ) : (
        rows.map((row) => (
          <button key={row.key} type="button" className="insight-row" onClick={row.onClick}>
            <span className={`svc-swatch svc-${serviceRampIndex(row.name)}`} aria-hidden="true" />
            <span className="insight-op">
              <Trunc text={row.name} />
            </span>
            <span className="insight-dur">{row.value}</span>
          </button>
        ))
      )}
    </section>
  );
}

function fleetKpis(rows: ServiceMetrics[], windowS: number) {
  let spans = 0;
  let errors = 0;
  let rate = 0;
  let p95w = 0;
  let p99w = 0;
  for (const r of rows) {
    spans += r.spans;
    errors += r.errors;
    rate += r.rate;
    p95w += r.p95_ns * r.spans;
    p99w += r.p99_ns * r.spans;
  }
  return {
    rate,
    errS: windowS > 0 ? errors / windowS : 0,
    errorRate: spans > 0 ? errors / spans : 0,
    p95: spans > 0 ? p95w / spans : 0,
    p99: spans > 0 ? p99w / spans : 0,
    spans,
  };
}

function rangeStepSeconds(step: string): number {
  switch (step) {
    case "1m":
      return 60;
    case "5m":
      return 300;
    case "10m":
      return 600;
    case "1h":
      return 3600;
    default:
      return 60;
  }
}

function splitWindow(start: string, end: string): { first: { start: string; end: string }; second: { start: string; end: string } } | null {
  const a = Date.parse(start);
  const b = Date.parse(end);
  if (!Number.isFinite(a) || !Number.isFinite(b) || b <= a) {
    return null;
  }
  const mid = toRFC3339(new Date(a + (b - a) / 2));
  return { first: { start, end: mid }, second: { start: mid, end } };
}

function rangeWindow(ms: number, now = new Date()): { start: string; end: string } {
  return slidingWindow(ms, now);
}

function windowFromSearch(): { preset: RangePreset; start: string; end: string } {
  const form = formFromSearchParams(new URLSearchParams(window.location.search));
  const startMs = Date.parse(form.start);
  const endMs = Date.parse(form.end);
  if (Number.isFinite(startMs) && Number.isFinite(endMs) && endMs > startMs) {
    const span = endMs - startMs;
    const preset = RANGE_PRESETS.find((p) => Math.abs(p.ms - span) < 2 * 60 * 1000) ?? RANGE_PRESETS[2];
    return { preset: preset ?? RANGE_PRESETS[2], start: form.start, end: form.end };
  }
  const preset = RANGE_PRESETS[2];
  return { preset, ...rangeWindow(preset.ms) };
}
