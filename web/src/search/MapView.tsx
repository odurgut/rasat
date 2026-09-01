import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { SearchError, getMetrics, getServiceMap, type ServiceMapEdge, type ServiceMapNode } from "./api";
import { formatDuration, formatPct } from "./format";
import { PrimaryButton } from "../chrome/PrimaryButton";
import { GhostButton } from "../chrome/GhostButton";
import { defaultForm, formFromSearchParams } from "./query";
import { serviceRampIndex } from "./color";
import { layoutServiceMap, type LaidMapEdge, type LaidMapNode } from "./mapLayout";
import { Jump } from "../chrome/Jump";
import { Workspace } from "../chrome/Workspace";
import { FloatTip, Trunc } from "./trunc";

type MapViewProps = {
  onOpen: (service: string) => void;
  onOpenService: (service: string) => void;
};

export function MapView({ onOpen, onOpenService }: MapViewProps) {
  const [nodes, setNodes] = useState<ServiceMapNode[]>([]);
  const [edges, setEdges] = useState<ServiceMapEdge[]>([]);
  const [p95, setP95] = useState<Record<string, number>>({});
  const [status, setStatus] = useState<"loading" | "ok" | "error">("loading");
  const [error, setError] = useState("");
  const [selected, setSelected] = useState("");
  const [mode, setMode] = useState<"calls" | "errors">("calls");
  const seq = useRef(0);

  useEffect(() => {
    const ac = new AbortController();
    const n = ++seq.current;
    void (async () => {
      try {
        const form = catalogForm();
        const [graph, metrics] = await Promise.all([
          getServiceMap(form, ac.signal),
          getMetrics({ start: form.start, end: form.end, limit: "100" }, ac.signal).then(
            (body) => body.metrics,
            () => [],
          ),
        ]);
        if (n !== seq.current) {
          return;
        }
        const nextP95: Record<string, number> = {};
        for (const m of metrics) {
          if (m.p95_ns > 0) {
            nextP95[m.service] = m.p95_ns;
          }
        }
        setNodes(graph.nodes);
        setEdges(graph.edges);
        setP95(nextP95);
        setStatus("ok");
      } catch (e) {
        if (n !== seq.current) {
          return;
        }
        if (e instanceof DOMException && e.name === "AbortError") {
          return;
        }
        const msg = e instanceof SearchError || e instanceof Error ? e.message : "load failed";
        setNodes([]);
        setEdges([]);
        setP95({});
        setError(msg);
        setStatus("error");
      }
    })();
    return () => ac.abort();
  }, []);

  const node = nodes.find((r) => r.service === selected);
  const incoming = edges.filter((e) => e.to === selected);
  const outgoing = edges.filter((e) => e.from === selected);
  const related = relatedNames(selected, edges);
  const nodeErr = node ? ratio(node.errors, node.spans) : 0;
  const nodeP95 = node ? p95[node.service] ?? 0 : 0;

  return (
    <Workspace
      work={
        <>
          <div className="pane-work-scroll map-work" onClick={() => setSelected("")}>
            <div
              className="map-modes"
              onClick={(ev) => ev.stopPropagation()}
            >
              <GhostButton label="calls" active={mode === "calls"} onClick={() => setMode("calls")} />
              <GhostButton label="errors" active={mode === "errors"} onClick={() => setMode("errors")} />
            </div>
            {status === "loading" ? (
              <p className="empty">loading</p>
            ) : status === "error" ? (
              <p className="surface-error">
                <span className="surface-error-word">ERROR</span> {error}
              </p>
            ) : nodes.length === 0 ? (
              <p className="empty">no services in window</p>
            ) : mode === "errors" && !edges.some((e) => e.errors > 0) && !nodes.some((n) => n.errors > 0) ? (
              <p className="empty">no errors in window</p>
            ) : (
              <ServiceMapSvg
                nodes={nodes}
                edges={edges}
                p95={p95}
                selected={selected}
                related={related}
                mode={mode}
                onSelect={setSelected}
              />
            )}
          </div>
        </>
      }
      detail={
        node ? (
          <div className="pane-detail-scroll">
              <header className="insp-head">
                <p className={nodeErr > 0 ? "insp-title is-err" : "insp-title"}>
                  <span className={`svc-swatch svc-${serviceRampIndex(node.service)}`} aria-hidden="true" />
                  <Jump onClick={() => onOpenService(node.service)}>{node.service}</Jump>
                </p>
                <p className="insp-meta">
                  {node.spans} spans
                  {" · "}
                  {incoming.length} in
                  {" · "}
                  {outgoing.length} out
                  {nodeErr > 0 ? (
                    <>
                      {" · "}
                      <span className="tok-err">ERR</span> {formatPct(nodeErr)}
                    </>
                  ) : null}
                  {nodeP95 > 0 ? (
                    <>
                      {" · "}
                      p95 {formatDuration(nodeP95)}
                    </>
                  ) : null}
                </p>
              </header>
              {outgoing.length > 0 ? (
                <section className="kv-section">
                  <p className="kv-section-label">calls</p>
                  <div className="kv-table">
                    {outgoing.map((e) => (
                      <Kv
                        key={`out-${e.to}`}
                        label={e.to}
                        value={edgeKv(e)}
                        onLabel={() => onOpenService(e.to)}
                      />
                    ))}
                  </div>
                </section>
              ) : null}
              {incoming.length > 0 ? (
                <section className="kv-section">
                  <p className="kv-section-label">called by</p>
                  <div className="kv-table">
                    {incoming.map((e) => (
                      <Kv
                        key={`in-${e.from}`}
                        label={e.from}
                        value={edgeKv(e)}
                        onLabel={() => onOpenService(e.from)}
                      />
                    ))}
                  </div>
                </section>
              ) : null}
              {outgoing.length === 0 && incoming.length === 0 ? (
                <p className="empty">no edges in window</p>
              ) : null}
              <p className="map-open">
                <PrimaryButton label="open traces" onClick={() => onOpen(node.service)} />
              </p>
          </div>
        ) : undefined
      }
    />
  );
}

function Kv({
  label,
  value,
  onLabel,
}: {
  label: string;
  value: ReactNode;
  onLabel?: () => void;
}) {
  return (
    <div className="kv-row">
      <span className="kv-key">
        {onLabel ? (
          <Jump onClick={onLabel}>
            <Trunc text={label} />
          </Jump>
        ) : (
          <Trunc text={label} />
        )}
      </span>
      <span className="kv-val">{value}</span>
    </div>
  );
}

function ServiceMapSvg({
  nodes,
  edges,
  p95,
  selected,
  related,
  mode,
  onSelect,
}: {
  nodes: ServiceMapNode[];
  edges: ServiceMapEdge[];
  p95: Record<string, number>;
  selected: string;
  related: Set<string>;
  mode: "calls" | "errors";
  onSelect: (name: string) => void;
}) {
  const wrap = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState({ w: 0, h: 0 });
  const [tip, setTip] = useState<MapTip | null>(null);

  useLayoutEffect(() => {
    const el = wrap.current;
    if (!el) {
      return;
    }
    const read = () => setBox({ w: el.clientWidth, h: el.clientHeight });
    read();
    const ro = new ResizeObserver(read);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const laid = layoutServiceMap(nodes, edges, box.w >= 8 && box.h >= 8 ? { width: box.w, height: box.h } : undefined);
  const fills = box.w + 1 >= laid.width && box.h + 1 >= laid.height;
  const maxCalls = laid.edges.reduce((m, e) => Math.max(m, e.calls), 0);
  const visibleEdges = [...(mode === "errors" ? laid.edges.filter((e) => e.errors > 0) : laid.edges)].sort((a, b) => {
    const ai = selected && (a.from === selected || a.to === selected) ? 1 : 0;
    const bi = selected && (b.from === selected || b.to === selected) ? 1 : 0;
    return ai - bi;
  });
  const errorNames = errorGraphNames(nodes, edges);

  return (
    <div ref={wrap} className="map-stage">
      {box.w < 8 || box.h < 8 ? null : (
        <svg
          className="map-svg"
          width={fills ? "100%" : laid.width}
          height={fills ? "100%" : laid.height}
          viewBox={`0 0 ${laid.width} ${laid.height}`}
          preserveAspectRatio="xMidYMid meet"
          role="img"
          aria-label={mode === "errors" ? "error map" : "service map"}
          onClick={() => onSelect("")}
          onMouseLeave={() => setTip(null)}
        >
          <defs>
            <marker id="map-arrow-ok" markerUnits="userSpaceOnUse" viewBox="0 0 5 5" markerWidth="5" markerHeight="5" refX="4.5" refY="2.5" orient="auto">
              <path d="M 0.5 0.6 L 4.5 2.5 L 0.5 4.4" fill="none" stroke="rgb(var(--color-text-heading))" strokeOpacity="0.38" strokeWidth="1" strokeLinejoin="miter" />
            </marker>
            <marker id="map-arrow-warn" markerUnits="userSpaceOnUse" viewBox="0 0 5 5" markerWidth="5" markerHeight="5" refX="4.5" refY="2.5" orient="auto">
              <path d="M 0.5 0.6 L 4.5 2.5 L 0.5 4.4" fill="none" stroke="rgb(var(--color-warn))" strokeWidth="1" strokeLinejoin="miter" />
            </marker>
            <marker id="map-arrow-err" markerUnits="userSpaceOnUse" viewBox="0 0 5 5" markerWidth="5" markerHeight="5" refX="4.5" refY="2.5" orient="auto">
              <path d="M 0.5 0.6 L 4.5 2.5 L 0.5 4.4" fill="none" stroke="rgb(var(--color-error))" strokeWidth="1" strokeLinejoin="miter" />
            </marker>
          </defs>
          {visibleEdges.map((e) => (
            <path
              key={`hit-${e.from}->${e.to}`}
              className="map-edge-hit"
              d={e.path}
              onMouseMove={(ev) => {
                ev.stopPropagation();
                setTip({ kind: "edge", x: ev.clientX + 14, y: ev.clientY + 16, edge: e });
              }}
              onMouseLeave={() => setTip(null)}
            />
          ))}
          {visibleEdges.map((e) => {
            const incident = Boolean(selected) && (e.from === selected || e.to === selected);
            const ego = Boolean(selected) && related.has(e.from) && related.has(e.to);
            const cls = ["map-edge", `is-${e.tone}`];
            if (selected) {
              if (incident) {
                cls.push("is-incident");
              } else if (ego) {
                cls.push("is-focus");
              } else {
                cls.push("is-dim");
              }
            }
            const errRate = ratio(e.errors, e.calls);
            const arrow = incident || (mode === "errors" && !selected);
            return (
              <path
                key={`${e.from}->${e.to}`}
                className={cls.join(" ")}
                d={e.path}
                markerEnd={arrow ? `url(#map-arrow-${e.tone})` : undefined}
                style={{
                  strokeWidth: edgeWidth(e.calls, maxCalls, incident),
                  strokeOpacity: e.tone === "err" ? 0.42 + 0.58 * Math.min(1, errRate / 0.2) : undefined,
                }}
              />
            );
          })}
          {laid.nodes.map((n) => {
            const ramp = serviceRampIndex(n.service);
            const active = n.service === selected;
            const near = related.has(n.service);
            const cls = ["map-node", `svc-${ramp}`];
            if (mode === "errors" && !errorNames.has(n.service) && !active) {
              cls.push("is-dim");
            } else if (selected) {
              if (active) {
                cls.push("is-active");
              } else if (near) {
                cls.push("is-related");
              } else {
                cls.push("is-dim");
              }
            }
            const errRate = ratio(n.errors, n.spans);
            const p95ns = p95[n.service] ?? 0;
            const trackW = n.w - 2;
            const meterW = Math.max(errRate > 0 ? 2 : 0, trackW * Math.min(1, errRate));
            return (
              <g
                key={n.service}
                className={cls.join(" ")}
                role="button"
                tabIndex={0}
                onClick={(ev) => {
                  ev.stopPropagation();
                  onSelect(n.service);
                }}
                onMouseMove={(ev) => {
                  ev.stopPropagation();
                  setTip({ kind: "node", x: ev.clientX + 14, y: ev.clientY + 16, node: n, p95: p95ns });
                }}
                onMouseLeave={() => setTip(null)}
                onKeyDown={(ev) => {
                  if (ev.key === "Enter" || ev.key === " ") {
                    ev.preventDefault();
                    onSelect(n.service);
                  }
                }}
              >
                <rect className="map-node-box" x={n.x} y={n.y} width={n.w} height={n.h} rx={0} ry={0} />
                {errRate > 0 ? (
                  <>
                    <rect className="map-node-rail" x={n.x} y={n.y} width={2} height={n.h} />
                    <rect className="map-node-meter" x={n.x + 2} y={n.y + n.h - 2} width={trackW} height={2} />
                    <rect className="map-node-meter-fill" x={n.x + 2} y={n.y + n.h - 2} width={meterW} height={2} />
                  </>
                ) : null}
                <rect className={`map-node-swatch svc-${ramp}`} x={n.x + 12} y={n.y + 10} width={8} height={8} />
                <text className="map-node-label" x={n.x + 28} y={n.y + 19}>
                  {clipName(n.service)}
                </text>
                {errRate > 0 ? (
                  <>
                    <text className="map-node-err-word" x={n.x + 12} y={n.y + 36}>
                      ERR
                    </text>
                    <text className="map-node-err-pct" x={n.x + n.w - 10} y={n.y + 36} textAnchor="end">
                      {formatPct(errRate)}
                    </text>
                  </>
                ) : null}
                {p95ns > 0 ? (
                  <>
                    <text className="map-node-p95-word" x={n.x + 12} y={n.y + 50}>
                      P95
                    </text>
                    <text className="map-node-p95-val" x={n.x + n.w - 10} y={n.y + 50} textAnchor="end">
                      {formatDuration(p95ns)}
                    </text>
                  </>
                ) : null}
              </g>
            );
          })}
          {selected
            ? visibleEdges
                .filter((e) => e.from === selected || e.to === selected)
                .map((e) => (
                  <text
                    key={`ms-${e.from}->${e.to}`}
                    className="map-edge-ms"
                    x={e.labelX}
                    y={e.labelY}
                    textAnchor="middle"
                    dy="-0.35em"
                  >
                    {formatDuration(e.avg_duration_ns)}
                  </text>
                ))
            : null}
        </svg>
      )}
      {tip ? <MapFloatTip tip={tip} /> : null}
    </div>
  );
}

type MapTip =
  | { kind: "edge"; x: number; y: number; edge: LaidMapEdge }
  | { kind: "node"; x: number; y: number; node: LaidMapNode; p95: number };

function MapFloatTip({ tip }: { tip: MapTip }) {
  if (tip.kind === "node") {
    const rate = ratio(tip.node.errors, tip.node.spans);
    return (
      <FloatTip x={tip.x} y={tip.y}>
        <p className="float-tip-title">{tip.node.service}</p>
        <p className="float-tip-meta">{tip.node.spans} spans</p>
        {rate > 0 ? (
          <p>
            <span className="tok-err">ERR</span> {formatPct(rate)}
          </p>
        ) : null}
        {tip.p95 > 0 ? <p className="float-tip-meta">p95 {formatDuration(tip.p95)}</p> : null}
      </FloatTip>
    );
  }
  const rate = ratio(tip.edge.errors, tip.edge.calls);
  return (
    <FloatTip x={tip.x} y={tip.y}>
      <p className="float-tip-title">
        {tip.edge.from} → {tip.edge.to}
      </p>
      <p className="float-tip-meta">
        {tip.edge.calls} · {formatDuration(tip.edge.avg_duration_ns)}
      </p>
      {rate > 0 ? (
        <p>
          <span className="tok-err">ERR</span> {formatPct(rate)}
        </p>
      ) : null}
    </FloatTip>
  );
}

function ratio(part: number, whole: number): number {
  if (!(whole > 0) || !(part > 0)) {
    return 0;
  }
  return part / whole;
}

function edgeWidth(calls: number, maxCalls: number, incident: boolean): number {
  const t = maxCalls > 0 ? calls / maxCalls : 0;
  const base = 1 + t * 1.1;
  return incident ? base + 0.35 : base;
}

function errorGraphNames(nodes: ServiceMapNode[], edges: ServiceMapEdge[]): Set<string> {
  const out = new Set<string>();
  for (const n of nodes) {
    if (n.errors > 0) {
      out.add(n.service);
    }
  }
  for (const e of edges) {
    if (e.errors > 0) {
      out.add(e.from);
      out.add(e.to);
    }
  }
  return out;
}

function edgeKv(e: ServiceMapEdge): ReactNode {
  const rate = ratio(e.errors, e.calls);
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

function relatedNames(selected: string, edges: ServiceMapEdge[]): Set<string> {
  const out = new Set<string>();
  if (!selected) {
    return out;
  }
  out.add(selected);
  for (const e of edges) {
    if (e.from === selected) {
      out.add(e.to);
    }
    if (e.to === selected) {
      out.add(e.from);
    }
  }
  return out;
}

function clipName(name: string): string {
  if (name.length <= 18) {
    return name;
  }
  return `${name.slice(0, 17)}…`;
}

function catalogForm() {
  const form = formFromSearchParams(new URLSearchParams(window.location.search));
  if (!form.start || !form.end) {
    const d = defaultForm();
    form.start = d.start;
    form.end = d.end;
  }
  form.limit = "100";
  return form;
}
