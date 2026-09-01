import { useEffect, useLayoutEffect, useRef, useState, type MouseEvent as MouseEvt, type PointerEvent as PtrEvent } from "react";
import { formatClock, formatDuration, formatTimestamp, kindLabel } from "./format";
import { SearchError, searchLogs, type Bottleneck, type CriticalPathStep, type LogRow, type SpanDetail } from "./api";
import { coverTraceWindow, type SearchForm } from "./query";
import { Jump } from "../chrome/Jump";
import { Trunc } from "./trunc";
import { serviceRampIndex } from "./color";
import { levelClass } from "./LogList";

type SpanInspectorProps = {
  span: SpanDetail;
  traceID: string;
  form: SearchForm;
  onOpenLogs?: (traceID: string) => void;
  onOpenService?: (service: string) => void;
  onOpenTrace?: (traceID: string) => void;
};

type Tab = "details" | "events" | "logs";

type KvRow = {
  label: string;
  value: string;
  err?: boolean;
  onLabel?: () => void;
  onValue?: () => void;
};

export function SpanInspector({
  span,
  traceID,
  form,
  onOpenLogs,
  onOpenService,
  onOpenTrace,
}: SpanInspectorProps) {
  const [tab, setTab] = useState<Tab>("details");
  const [logs, setLogs] = useState<LogRow[]>([]);
  const [logsErr, setLogsErr] = useState("");
  const [logsLoading, setLogsLoading] = useState(false);
  const err = span.status_code === 2;
  const attrs = span.span_attributes ?? {};
  const resource = span.resource_attributes ?? {};
  const events = span.events ?? [];
  const links = span.links ?? [];
  const op = span.operation || "(no op)";
  const service = span.service || "(unknown)";

  useEffect(() => {
    if (!traceID) {
      setLogs([]);
      return;
    }
    const ac = new AbortController();
    const w = coverTraceWindow(form, { timestamp: span.timestamp, duration_ns: span.duration_ns });
    setLogsLoading(true);
    setLogsErr("");
    void (async () => {
      try {
        const found = await searchLogs(
          {
            service: "",
            level: "",
            trace_id: traceID,
            start: w.start,
            end: w.end,
            limit: "50",
          },
          ac.signal,
        );
        if (ac.signal.aborted) {
          return;
        }
        found.sort((a, b) => Date.parse(a.timestamp) - Date.parse(b.timestamp));
        setLogs(found);
        setLogsLoading(false);
      } catch (e) {
        if (ac.signal.aborted) {
          return;
        }
        if (e instanceof DOMException && e.name === "AbortError") {
          return;
        }
        const msg = e instanceof SearchError || e instanceof Error ? e.message : "load failed";
        setLogs([]);
        setLogsErr(msg);
        setLogsLoading(false);
      }
    })();
    return () => ac.abort();
  }, [traceID, span.timestamp, span.duration_ns, form.start, form.end]);

  return (
    <>
      <header className="insp-head">
        <p className={err ? "insp-title is-err" : "insp-title"}>
          {err ? <span className="tok-err">ERR </span> : null}
          {op}
        </p>
        <p className="insp-meta">
          <span className={`svc-swatch svc-${serviceRampIndex(service)}`} aria-hidden="true" />
          {onOpenService && span.service ? (
            <Jump onClick={() => onOpenService(span.service)}>{service}</Jump>
          ) : (
            service
          )}
          {" · "}
          {kindLabel(span.kind)}
          {" · "}
          {formatDuration(span.duration_ns)}
        </p>
      </header>
      <div className="tabs">
        <button type="button" className={tab === "details" ? "ghost is-active" : "ghost"} onClick={() => setTab("details")}>
          details
        </button>
        <button type="button" className={tab === "events" ? "ghost is-active" : "ghost"} onClick={() => setTab("events")}>
          events{events.length > 0 ? ` ${events.length}` : ""}
        </button>
        <button type="button" className={tab === "logs" ? "ghost is-active" : "ghost"} onClick={() => setTab("logs")}>
          logs{logs.length > 0 ? ` ${logs.length}` : ""}
        </button>
        <CopyWord text={formatAttrs(span)} />
      </div>
      {tab === "details" ? (
        <>
          <section className="kv-section">
            <p className="kv-section-label">ids</p>
            <KvTable
              rows={[
                { label: "span", value: span.span_id },
                { label: "parent", value: span.parent_span_id || "—" },
                { label: "start", value: formatTimestamp(span.timestamp) },
                { label: "status", value: statusLabel(span.status_code), err },
                ...(span.status_message
                  ? [{ label: "message", value: span.status_message, err }]
                  : []),
              ]}
            />
          </section>
          {Object.keys(attrs).length > 0 ? (
            <section className="kv-section">
              <p className="kv-section-label">attributes</p>
              <KvTable rows={Object.entries(attrs).map(([label, value]) => ({ label, value }))} />
            </section>
          ) : null}
          {links.length > 0 ? (
            <section className="kv-section">
              <p className="kv-section-label">links</p>
              <KvTable
                rows={links.map((l) => ({
                  label: l.trace_id,
                  value: l.span_id,
                  onLabel:
                    onOpenTrace && l.trace_id && l.trace_id !== traceID
                      ? () => onOpenTrace(l.trace_id)
                      : undefined,
                }))}
              />
            </section>
          ) : null}
          {Object.keys(resource).length > 0 ? (
            <section className="kv-section">
              <p className="kv-section-label">resource</p>
              <KvTable rows={Object.entries(resource).map(([label, value]) => ({ label, value }))} />
            </section>
          ) : null}
          {span.scope_name ? (
            <section className="kv-section">
              <p className="kv-section-label">scope</p>
              <KvTable
                rows={[
                  { label: "name", value: span.scope_name },
                  ...(span.scope_version ? [{ label: "version", value: span.scope_version }] : []),
                ]}
              />
            </section>
          ) : null}
        </>
      ) : tab === "events" ? (
        events.length === 0 ? (
          <p className="empty">no events</p>
        ) : (
          events.map((ev, i) => (
            <section key={`${ev.time}-${ev.name}-${i}`} className="kv-section">
              <p className="kv-section-label">{ev.name || "event"}</p>
              <KvTable
                rows={[
                  { label: "time", value: formatTimestamp(ev.time) },
                  ...Object.entries(ev.attributes ?? {}).map(([label, value]) => ({ label, value })),
                ]}
              />
            </section>
          ))
        )
      ) : logsLoading ? (
        <p className="empty">loading</p>
      ) : logsErr ? (
        <p className="surface-error">
          <span className="surface-error-word">ERROR</span> {logsErr}
        </p>
      ) : logs.length === 0 ? (
        <p className="empty">no related logs</p>
      ) : (
        <>
          {logs.map((row, i) => (
            <section key={`${row.timestamp}-${i}`} className="kv-section">
              <p className="kv-section-label">
                <span className={levelClass(row.level)}>{(row.level || "INFO").toUpperCase()}</span>
                {" · "}
                {formatClock(row.timestamp)}
              </p>
              <p className="log-related-msg">{row.message}</p>
            </section>
          ))}
          {onOpenLogs && traceID ? (
            <p className="search-actions log-jump">
              <button type="button" className="ghost" onClick={() => onOpenLogs(traceID)}>
                open logs
              </button>
            </p>
          ) : null}
        </>
      )}
    </>
  );
}

type TraceInsightProps = {
  path: CriticalPathStep[];
  pathNs: number;
  totalNs: number;
  bottlenecks: Bottleneck[];
  selectedID: string;
  onSelect: (spanID: string) => void;
};

const DOCK_MIN = 88;
const DOCK_SNAP = 108;
const DOCK_DRAG = 4;
const DOCK_PAD = 8;
const DOCK_WORK = 96;

function dockStd(): number {
  const root = document.documentElement;
  const raw = getComputedStyle(root).getPropertyValue("--dock-std").trim();
  const fs = parseFloat(getComputedStyle(root).fontSize) || 12;
  if (raw.endsWith("rem")) {
    return parseFloat(raw) * fs;
  }
  if (raw.endsWith("px")) {
    return parseFloat(raw);
  }
  return 216;
}

export function InsightDock({
  path,
  pathNs,
  totalNs,
  bottlenecks,
  selectedID,
  onSelect,
}: TraceInsightProps) {
  const std = dockStd();
  const [open, setOpen] = useState(true);
  const [h, setH] = useState(std);
  const dockRef = useRef<HTMLDivElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const lastH = useRef(std);
  const fitRef = useRef(std);
  const drag = useRef<{ y: number; h: number; moved: boolean; raw: number } | null>(null);
  const didDrag = useRef(false);
  const split = path.length > 0 && bottlenecks.length > 0;

  function headH(): number {
    const head = dockRef.current?.querySelector(".wf-dock-head");
    return head ? Math.round(head.getBoundingClientRect().height) : 32;
  }

  function contentFit(): number {
    const body = bodyRef.current;
    if (!body) {
      return fitRef.current;
    }
    let inner = 0;
    for (const section of body.querySelectorAll(".kv-section")) {
      let col = 0;
      for (const child of section.children) {
        col += (child as HTMLElement).offsetHeight;
      }
      inner = Math.max(inner, col);
    }
    return headH() + inner + DOCK_PAD;
  }

  function paneCap(): number {
    const pane = dockRef.current?.parentElement;
    if (!pane) {
      return 480;
    }
    return Math.max(std, pane.clientHeight - DOCK_WORK);
  }

  function maxH(): number {
    const fit = contentFit();
    fitRef.current = fit;
    const cap = paneCap();
    if (fit <= std) {
      return Math.min(std, cap);
    }
    return Math.min(fit, cap);
  }

  function clampOpen(px: number): number {
    return Math.min(maxH(), Math.max(DOCK_MIN, px));
  }

  function isMax(): boolean {
    return open && h >= maxH() - 6;
  }

  useLayoutEffect(() => {
    if (!open) {
      return;
    }
    const max = maxH();
    setH((prev) => (prev > max ? max : prev));
  }, [open, path, bottlenecks]);

  if (path.length === 0 && bottlenecks.length === 0) {
    return null;
  }

  function toggle(): void {
    if (open) {
      lastH.current = h;
      setOpen(false);
      return;
    }
    setH(clampOpen(lastH.current));
    setOpen(true);
  }

  function onHeadDown(e: PtrEvent<HTMLDivElement>): void {
    if ((e.target as HTMLElement).closest("button")) {
      return;
    }
    e.currentTarget.setPointerCapture(e.pointerId);
    const el = dockRef.current;
    const vis = el ? el.getBoundingClientRect().height : h;
    didDrag.current = false;
    drag.current = { y: e.clientY, h: vis, moved: false, raw: vis };
  }

  function onHeadMove(e: PtrEvent<HTMLDivElement>): void {
    const d = drag.current;
    if (!d) {
      return;
    }
    const raw = d.h + (d.y - e.clientY);
    d.raw = raw;
    if (!d.moved && Math.abs(e.clientY - d.y) < DOCK_DRAG) {
      return;
    }
    d.moved = true;
    didDrag.current = true;
    const max = maxH();
    const minVis = headH();
    if (!open) {
      setOpen(true);
    }
    setH(Math.min(max, Math.max(minVis, raw)));
  }

  function onHeadUp(): void {
    const d = drag.current;
    drag.current = null;
    if (!d?.moved) {
      return;
    }
    if (d.raw <= DOCK_SNAP) {
      if (d.h > DOCK_SNAP) {
        lastH.current = d.h;
        setH(d.h);
      }
      setOpen(false);
      return;
    }
    const next = clampOpen(d.raw);
    lastH.current = next;
    setH(next);
    setOpen(true);
  }

  function onHeadDblClick(e: MouseEvt<HTMLDivElement>): void {
    if ((e.target as HTMLElement).closest("button")) {
      return;
    }
    if (didDrag.current) {
      return;
    }
    e.preventDefault();
    if (!open || !isMax()) {
      if (open) {
        lastH.current = h;
      }
      setH(maxH());
      setOpen(true);
      return;
    }
    lastH.current = h;
    setOpen(false);
  }

  return (
    <div
      ref={dockRef}
      className={split ? "wf-dock is-split" : "wf-dock"}
      style={open ? { height: h } : undefined}
    >
      <div
        className={split ? "wf-dock-head is-split" : "wf-dock-head"}
        onPointerDown={onHeadDown}
        onPointerMove={onHeadMove}
        onPointerUp={onHeadUp}
        onPointerCancel={onHeadUp}
        onLostPointerCapture={onHeadUp}
        onDoubleClick={onHeadDblClick}
      >
        {path.length > 0 ? (
          <div className={bottlenecks.length > 0 ? "kv-section-label" : "kv-section-label wf-dock-end"}>
            critical path
            {bottlenecks.length === 0 ? <Toggle open={open} onClick={toggle} /> : null}
          </div>
        ) : null}
        {bottlenecks.length > 0 ? (
          <div className="kv-section-label wf-dock-end">
            bottlenecks
            <Toggle open={open} onClick={toggle} />
          </div>
        ) : null}
      </div>
      {open ? (
        <div className="wf-dock-body" ref={bodyRef}>
          <TraceInsight
            path={path}
            pathNs={pathNs}
            totalNs={totalNs}
            bottlenecks={bottlenecks}
            selectedID={selectedID}
            onSelect={onSelect}
            labels={false}
          />
        </div>
      ) : null}
    </div>
  );
}

function Toggle({ open, onClick }: { open: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      className="wf-dock-toggle"
      aria-expanded={open}
      aria-label={open ? "collapse" : "expand"}
      onPointerDown={(e) => e.stopPropagation()}
      onDoubleClick={(e) => e.stopPropagation()}
      onClick={onClick}
    >
      {open ? "▼" : "▲"}
    </button>
  );
}

export function TraceInsight({
  path,
  pathNs,
  totalNs,
  bottlenecks,
  selectedID,
  onSelect,
  labels = true,
}: TraceInsightProps & { labels?: boolean }) {
  if (path.length === 0 && bottlenecks.length === 0) {
    return null;
  }
  return (
    <div className={path.length > 0 && bottlenecks.length > 0 ? "wf-insight is-split" : "wf-insight"}>
      {path.length > 0 ? (
        <section className="kv-section">
          {labels ? <p className="kv-section-label">critical path</p> : null}
          <p className="insight-meta">
            {formatDuration(totalNs)} total
            {pathNs > 0 ? ` · ${formatDuration(pathNs)} path` : ""}
          </p>
          <PathChain path={path} selectedID={selectedID} onSelect={onSelect} />
        </section>
      ) : null}
      {bottlenecks.length > 0 ? (
        <section className="kv-section">
          {labels ? <p className="kv-section-label">bottlenecks</p> : null}
          <BottleneckList rows={bottlenecks} selectedID={selectedID} onSelect={onSelect} />
        </section>
      ) : null}
    </div>
  );
}

function BottleneckList({
  rows,
  selectedID,
  onSelect,
}: {
  rows: Bottleneck[];
  selectedID: string;
  onSelect?: (spanID: string) => void;
}) {
  return (
    <div className="insight-list">
      {rows.map((b) => {
        const cls = ["insight-row"];
        if (b.span_id === selectedID) {
          cls.push("is-active");
        }
        return (
          <button key={b.span_id} type="button" className={cls.join(" ")} onClick={() => onSelect?.(b.span_id)}>
            <span className={`svc-swatch svc-${serviceRampIndex(b.service || "")}`} aria-hidden="true" />
            <span className="insight-svc">
              <Trunc text={b.service || "(unknown)"} />
            </span>
            <span className="insight-op">
              <Trunc text={b.operation || "(no op)"} />
            </span>
            <span className="insight-dur">{formatDuration(b.exclusive_ns)}</span>
          </button>
        );
      })}
    </div>
  );
}

function PathChain({
  path,
  selectedID,
  onSelect,
}: {
  path: CriticalPathStep[];
  selectedID: string;
  onSelect?: (spanID: string) => void;
}) {
  const last = path.length - 1;
  return (
    <div className="path-chain">
      {path.map((step, i) => {
        const cls = ["path-step"];
        if (step.span_id === selectedID) {
          cls.push("is-active");
        }
        return (
          <button
            key={step.span_id}
            type="button"
            className={cls.join(" ")}
            onClick={() => onSelect?.(step.span_id)}
          >
            <span className="wf-tree" aria-hidden="true">
              {Array.from({ length: i }, (_, d) => (
                <span
                  key={d}
                  className={d === i - 1 ? (i === last ? "wf-guide is-end" : "wf-guide is-mid") : "wf-guide"}
                />
              ))}
            </span>
            <span className="path-body">
              <span className={`svc-swatch svc-${serviceRampIndex(step.service || "")}`} aria-hidden="true" />
              <span className="path-svc">
                <Trunc text={step.service || "(unknown)"} />
              </span>
              <span className="path-dur">{formatDuration(step.duration_ns)}</span>
            </span>
          </button>
        );
      })}
    </div>
  );
}

function KvTable({ rows }: { rows: KvRow[] }) {
  return (
    <div className="kv-table">
      {rows.map((r) => (
        <div key={`${r.label}:${r.value}`} className="kv-row">
          <span className="kv-key">
            {r.onLabel ? (
              <Jump onClick={r.onLabel}>
                <Trunc text={r.label} />
              </Jump>
            ) : (
              <Trunc text={r.label} />
            )}
          </span>
          <span className={r.err ? "kv-val is-err" : "kv-val"}>
            {r.onValue ? (
              <Jump onClick={r.onValue}>
                {r.value}
              </Jump>
            ) : (
              r.value
            )}
          </span>
        </div>
      ))}
    </div>
  );
}

function statusLabel(code: number): string {
  switch (code) {
    case 1:
      return "OK";
    case 2:
      return "ERR";
    default:
      return "UNSET";
  }
}

function formatAttrs(span: SpanDetail): string {
  return JSON.stringify(
    {
      span_id: span.span_id,
      parent_span_id: span.parent_span_id || undefined,
      service: span.service,
      operation: span.operation,
      kind: span.kind,
      duration_ns: span.duration_ns,
      status_code: span.status_code,
      status_message: span.status_message || undefined,
      span_attributes: span.span_attributes ?? {},
      resource_attributes: span.resource_attributes ?? {},
      events: span.events ?? [],
      links: span.links ?? [],
      scope: span.scope_name ? `${span.scope_name}@${span.scope_version}` : undefined,
    },
    null,
    2,
  );
}

function CopyWord({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) {
      return;
    }
    const t = window.setTimeout(() => setCopied(false), 1200);
    return () => window.clearTimeout(t);
  }, [copied]);

  return (
    <button
      type="button"
      className="ghost inspector-copy"
      onClick={() => {
        void navigator.clipboard.writeText(text).then(
          () => setCopied(true),
          () => undefined,
        );
      }}
    >
      {copied ? "done" : "copy"}
    </button>
  );
}
