import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { SearchError, getErrorCauses, getMetrics, getServiceMap, listOperations, listServices, type ErrorCause, type MetricPoint, type OperationRow, type ServiceMapEdge, type ServiceMetrics, type ServiceRow } from "./api";
import { ServiceList } from "./ServiceList";
import { DashChart } from "./DashChart";
import type { ChartSeries } from "./chart";
import { formatDuration, formatPct, formatRate, formatTimestamp } from "./format";
import { PrimaryButton } from "../chrome/PrimaryButton";
import { GhostButton } from "../chrome/GhostButton";
import { formFromSearchParams, slidingWindow, writePageURL, defaultForm, type SearchForm } from "./query";
import { serviceRampIndex } from "./color";
import { Trunc } from "./trunc";
import { Workspace } from "../chrome/Workspace";
import { TabCard } from "./TabCard";

type ServicesViewProps = {
  active?: boolean;
  focus?: string;
  focusTick?: number;
  onOpen: (service: string) => void;
  onOpenTraces: (form: SearchForm) => void;
};

const RANGE_PRESETS = [
  { label: "1h", ms: 60 * 60 * 1000, step: "1m" },
  { label: "6h", ms: 6 * 60 * 60 * 1000, step: "5m" },
  { label: "24h", ms: 24 * 60 * 60 * 1000, step: "10m" },
  { label: "7d", ms: 7 * 24 * 60 * 60 * 1000, step: "1h" },
] as const;

const maxDashOps = 8;
const LIST_POLL_MS = 15_000;

type RangePreset = (typeof RANGE_PRESETS)[number];

export function ServicesView({ active = true, focus = "", focusTick = 0, onOpen, onOpenTraces }: ServicesViewProps) {
  const initial = useMemo(() => windowFromSearch(), []);
  const [range, setRange] = useState<RangePreset>(initial.preset);
  const [start, setStart] = useState(initial.start);
  const [end, setEnd] = useState(initial.end);
  const [rows, setRows] = useState<ServiceRow[]>([]);
  const [status, setStatus] = useState<"loading" | "ok" | "error">("loading");
  const [error, setError] = useState("");
  const [selected, setSelected] = useState(() => formFromSearchParams(new URLSearchParams(window.location.search)).service);
  const [kpis, setKpis] = useState<ServiceMetrics | null>(null);
  const [causes, setCauses] = useState<ErrorCause[]>([]);
  const [ops, setOps] = useState<OperationRow[]>([]);
  const [edges, setEdges] = useState<ServiceMapEdge[]>([]);
  const [points, setPoints] = useState<MetricPoint[]>([]);
  const [windowS, setWindowS] = useState(0);
  const [dashStatus, setDashStatus] = useState<"idle" | "loading" | "ok" | "error">("idle");
  const [dashError, setDashError] = useState("");
  const seq = useRef(0);
  const dashSeq = useRef(0);
  const fetching = useRef(false);
  const wasActive = useRef(active);
  const rangeRef = useRef(range);
  rangeRef.current = range;

  useEffect(() => {
    const ac = new AbortController();
    const n = ++seq.current;
    fetching.current = true;
    void (async () => {
      try {
        const form = {
          service: "",
          op: "",
          min: "",
          status: "",
          start,
          end,
          limit: "100",
        };
        const [found, graph] = await Promise.all([
          listServices(form, ac.signal),
          getServiceMap(form, ac.signal).then(
            (g) => g.edges,
            () => [] as ServiceMapEdge[],
          ),
        ]);
        if (n !== seq.current) {
          return;
        }
        setRows(found);
        setEdges(graph);
        setStatus("ok");
        setSelected((cur) => {
          if (cur && found.some((r) => r.service === cur)) {
            return cur;
          }
          const fromURL = formFromSearchParams(new URLSearchParams(window.location.search)).service.trim();
          if (fromURL && found.some((r) => r.service === fromURL)) {
            return fromURL;
          }
          return "";
        });
      } catch (e) {
        if (n !== seq.current) {
          return;
        }
        if (e instanceof DOMException && e.name === "AbortError") {
          return;
        }
        const msg = e instanceof SearchError || e instanceof Error ? e.message : "load failed";
        setRows([]);
        setEdges([]);
        setError(msg);
        setStatus("error");
      } finally {
        if (n === seq.current) {
          fetching.current = false;
        }
      }
    })();
    return () => ac.abort();
  }, [start, end]);

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
    const id = window.setInterval(slide, LIST_POLL_MS);
    return () => window.clearInterval(id);
  }, [active]);

  useEffect(() => {
    if (focus) {
      setSelected(focus);
    }
  }, [focus, focusTick]);

  useEffect(() => {
    if (!selected) {
      setKpis(null);
      setCauses([]);
      setOps([]);
      setPoints([]);
      setDashStatus("idle");
      return;
    }
    const ac = new AbortController();
    const n = ++dashSeq.current;
    setDashStatus("loading");
    void (async () => {
      try {
        const body = await getMetrics(
          { start, end, limit: "1", service: selected, step: range.step },
          ac.signal,
        );
        if (n !== dashSeq.current) {
          return;
        }
        setKpis(body.metrics[0] ?? null);
        setPoints(body.series[0]?.points ?? []);
        setWindowS(body.window_s);
        setDashStatus("ok");
        setDashError("");
        try {
          const found = await getErrorCauses({ start, end, limit: "5", service: selected }, ac.signal);
          if (n !== dashSeq.current) {
            return;
          }
          setCauses(found);
        } catch (e) {
          if (n !== dashSeq.current) {
            return;
          }
          if (e instanceof DOMException && e.name === "AbortError") {
            return;
          }
          setCauses([]);
        }
        try {
          const found = await listOperations(selected, start, end, ac.signal, "20");
          if (n !== dashSeq.current) {
            return;
          }
          setOps(found);
        } catch (e) {
          if (n !== dashSeq.current) {
            return;
          }
          if (e instanceof DOMException && e.name === "AbortError") {
            return;
          }
          setOps([]);
        }
      } catch (e) {
        if (n !== dashSeq.current) {
          return;
        }
        if (e instanceof DOMException && e.name === "AbortError") {
          return;
        }
        const msg = e instanceof SearchError || e instanceof Error ? e.message : "load failed";
        setKpis(null);
        setCauses([]);
        setOps([]);
        setPoints([]);
        setDashError(msg);
        setDashStatus("error");
      }
    })();
    return () => ac.abort();
  }, [selected, start, end, range.step]);

  const row = rows.find((r) => r.service === selected);
  const outgoing = edges.filter((e) => e.from === selected).sort(byCallsDesc);
  const incoming = edges.filter((e) => e.to === selected).sort(byCallsDesc);
  const svcTone = rampTone(selected || "x");
  const errS = windowS > 0 && kpis ? kpis.errors / windowS : 0;
  const stepS = rangeStepSeconds(range.step);

  const times = useMemo(
    () =>
      points.map((p) => {
        const ms = Date.parse(p.t);
        return Number.isFinite(ms) ? ms / 1000 : 0;
      }),
    [points],
  );

  const traffic = useMemo<ChartSeries[]>(
    () => [
      {
        label: "RATE",
        tone: svcTone,
        fill: true,
        values: points.map((p) => p.rate),
      },
    ],
    [points, svcTone],
  );
  const errors = useMemo<ChartSeries[]>(
    () => [
      {
        label: "ERR/s",
        tone: "err",
        fill: true,
        values: points.map((p) => (stepS > 0 ? p.errors / stepS : 0)),
      },
    ],
    [points, stepS],
  );
  const latency = useMemo<ChartSeries[]>(
    () => [
      {
        label: "p50",
        tone: "svc-0",
        values: points.map((p) => (p.spans > 0 ? p.p50_ns : null)),
      },
      {
        label: "p95",
        tone: "svc-1",
        values: points.map((p) => (p.spans > 0 ? p.p95_ns : null)),
      },
      {
        label: "p99",
        tone: "svc-2",
        values: points.map((p) => (p.spans > 0 ? p.p99_ns : null)),
      },
    ],
    [points],
  );

  function applyRange(next: RangePreset): void {
    const window = rangeWindow(next.ms);
    setRange(next);
    setStart(window.start);
    setEnd(window.end);
  }

  function openTraces(): void {
    if (!selected) {
      return;
    }
    const form = formFromSearchParams(new URLSearchParams());
    writePageURL(
      {
        ...form,
        start,
        end,
        service: selected,
        op: "",
        min: "",
        status: "",
        limit: form.limit || "50",
      },
      "",
    );
    onOpen(selected);
  }

  function openCause(): void {
    if (!selected) {
      return;
    }
    onOpenTraces({
      ...defaultForm(),
      start,
      end,
      service: selected,
      status: "error",
      op: "",
      min: "",
      limit: "50",
    });
  }

  function openNeighbor(name: string): void {
    const form = formFromSearchParams(new URLSearchParams(window.location.search));
    writePageURL({ ...form, start, end, service: name }, "");
    setSelected(name);
  }

  function openOp(op: string): void {
    if (!selected) {
      return;
    }
    onOpenTraces({
      ...defaultForm(),
      start,
      end,
      service: selected,
      op,
      status: "",
      min: "",
      limit: "50",
    });
  }

  return (
    <Workspace
      list={
        <>
          <div className="pane-filter">
            <p className="field-label">range</p>
            <div className="dash-range">
              {RANGE_PRESETS.map((p) => (
                <GhostButton key={p.label} label={p.label} active={range.label === p.label} onClick={() => applyRange(p)} />
              ))}
            </div>
          </div>
          <div className="pane-scroll">
            {status === "loading" ? (
              <p className="empty">loading</p>
            ) : status === "error" ? (
              <p className="surface-error">
                <span className="surface-error-word">ERROR</span> {error}
              </p>
            ) : (
              <ServiceList rows={rows} selected={selected} onSelect={setSelected} />
            )}
          </div>
        </>
      }
      work={
        <div className="page">
          {dashStatus === "error" ? (
            <p className="surface-error">
              <span className="surface-error-word">ERROR</span> {dashError}
            </p>
          ) : !selected ? (
            <p className="empty">select a service</p>
          ) : (
            <>
              <div className="page-head">
                <p className="page-head-title">
                  <span className={`svc-swatch svc-${serviceRampIndex(selected)}`} aria-hidden="true" />
                  <Trunc text={selected} />
                </p>
                <span className="page-head-meta">
                  {kpis?.spans ?? row?.spans ?? 0} spans
                  {(kpis?.errors ?? row?.errors ?? 0) > 0 ? (
                    <>
                      {" · "}
                      <span className="tok-err">ERR</span> {formatPct(kpis?.error_rate ?? 0)}
                    </>
                  ) : null}
                  {row ? ` · ${formatTimestamp(row.last_seen)}` : ""}
                </span>
                <div className="page-head-tools">
                  <PrimaryButton label="open traces" onClick={openTraces} />
                </div>
              </div>
              <div className="card-grid card-kpis">
                <Kpi label="RATE" value={formatRate(kpis?.rate ?? 0)} />
                <Kpi label="ERR/s" value={formatRate(errS)} err={errS > 0} />
                <Kpi label="ERR" value={formatPct(kpis?.error_rate ?? 0)} err={(kpis?.error_rate ?? 0) > 0} />
                <Kpi label="P50" value={formatDuration(kpis?.p50_ns ?? 0)} />
                <Kpi label="P95" value={formatDuration(kpis?.p95_ns ?? 0)} />
                <Kpi label="P99" value={formatDuration(kpis?.p99_ns ?? 0)} />
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
              <div className="card-grid card-split">
                <section className="card">
                  <p className="card-label">summary</p>
                  <div className="kv-table">
                    {row ? (
                      <div className="kv-row">
                        <span className="kv-key">last seen</span>
                        <span className="kv-val">
                          <Trunc text={formatTimestamp(row.last_seen)} />
                        </span>
                      </div>
                    ) : null}
                    <div className="kv-row">
                      <span className="kv-key">spans</span>
                      <span className="kv-val">{String(kpis?.spans ?? row?.spans ?? 0)}</span>
                    </div>
                    <div className="kv-row">
                      <span className="kv-key">errors</span>
                      <span className={(kpis?.errors ?? row?.errors ?? 0) > 0 ? "kv-val is-err" : "kv-val"}>
                        {String(kpis?.errors ?? row?.errors ?? 0)}
                      </span>
                    </div>
                    <div className="kv-row">
                      <span className="kv-key">error rate</span>
                      <span className={(kpis?.error_rate ?? 0) > 0 ? "kv-val is-err" : "kv-val"}>
                        {formatPct(kpis?.error_rate ?? 0)}
                      </span>
                    </div>
                  </div>
                </section>
                <TabCard
                  key={`${selected}-graph`}
                  tabs={[
                    {
                      id: "dependencies",
                      label: "dependencies",
                      empty: dashStatus === "loading" ? "loading" : "no dependencies",
                      rows: outgoing.map((e) => ({
                        key: e.to,
                        name: e.to,
                        value: edgeValue(e),
                        onClick: () => openNeighbor(e.to),
                      })),
                    },
                    {
                      id: "dependents",
                      label: "dependents",
                      empty: dashStatus === "loading" ? "loading" : "no dependents",
                      rows: incoming.map((e) => ({
                        key: e.from,
                        name: e.from,
                        value: edgeValue(e),
                        onClick: () => openNeighbor(e.from),
                      })),
                    },
                  ]}
                />
                <TabCard
                  key={`${selected}-ops`}
                  tabs={[
                    {
                      id: "top",
                      label: "top operations",
                      empty: dashStatus === "loading" ? "loading" : "no operations",
                      swatch: false,
                      rows: topOperations(ops).map((o) => ({
                        key: `top-${o.operation}`,
                        name: o.operation || "(no op)",
                        value: String(o.spans),
                        onClick: () => openOp(o.operation),
                      })),
                    },
                    {
                      id: "slowest",
                      label: "slowest",
                      empty: dashStatus === "loading" ? "loading" : "no operations",
                      swatch: false,
                      rows: slowestOperations(ops).map((o) => ({
                        key: `slow-${o.operation}`,
                        name: o.operation || "(no op)",
                        value: formatDuration(o.p95_ns),
                        onClick: () => openOp(o.operation),
                      })),
                    },
                  ]}
                />
                <CardList
                  title="causes"
                  swatch={false}
                  empty={dashStatus === "loading" ? "loading" : "no causes"}
                  rows={causes.map((c) => ({
                    key: c.cause,
                    name: c.cause || "error",
                    value: String(c.count),
                    onClick: openCause,
                  }))}
                />
              </div>
            </>
          )}
        </div>
      }
    />
  );
}

function CardList({
  title,
  rows,
  empty = "no data",
  swatch = true,
}: {
  title: string;
  empty?: string;
  swatch?: boolean;
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
            {swatch ? <span className={`svc-swatch svc-${serviceRampIndex(row.name)}`} aria-hidden="true" /> : null}
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

function byCallsDesc(a: ServiceMapEdge, b: ServiceMapEdge): number {
  return b.calls - a.calls || a.from.localeCompare(b.from) || a.to.localeCompare(b.to);
}

function edgeValue(e: ServiceMapEdge): ReactNode {
  const rate = e.calls > 0 && e.errors > 0 ? e.errors / e.calls : 0;
  return (
    <>
      {e.calls} · {formatDuration(e.avg_duration_ns)}
      {rate > 0 ? (
        <>
          {" · "}
          <span className="tok-err">{formatPct(rate)}</span>
        </>
      ) : null}
    </>
  );
}

function Kpi({ label, value, err = false }: { label: string; value: string; err?: boolean }) {
  return (
    <div className="card card-kpi">
      <span className="dash-kpi-label">{label}</span>
      <span className={err ? "dash-kpi-val is-err" : "dash-kpi-val"}>{value}</span>
    </div>
  );
}

function topOperations(rows: OperationRow[]): OperationRow[] {
  return rows.slice(0, maxDashOps);
}

function slowestOperations(rows: OperationRow[]): OperationRow[] {
  return [...rows]
    .sort((a, b) => b.p95_ns - a.p95_ns || b.spans - a.spans || a.operation.localeCompare(b.operation))
    .slice(0, maxDashOps);
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
  const w = rangeWindow(preset.ms);
  return { preset, ...w };
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

function rampTone(name: string): ChartSeries["tone"] {
  const i = serviceRampIndex(name);
  return (`svc-${i}`) as ChartSeries["tone"];
}
