import { useLayoutEffect, useRef, useState, type Ref } from "react";
import { formatDuration, formatTimestamp, kindLabel, timeTicks } from "./format";
import { serviceRampIndex } from "./color";
import { layoutWaterfall, type LaidSpan } from "./layout";
import type { TraceDetail } from "./api";
import { Jump } from "../chrome/Jump";
import { GhostButton } from "../chrome/GhostButton";
import { FloatTip, Trunc } from "./trunc";

type WaterfallProps = {
  detail: TraceDetail;
  selectedID: string;
  onSelect: (spanID: string) => void;
  onOpenService?: (service: string) => void;
  onBack?: () => void;
};

export function Waterfall({ detail, selectedID, onSelect, onOpenService, onBack }: WaterfallProps) {
  const rows = layoutWaterfall(detail);
  const selectedRef = useRef<HTMLButtonElement>(null);
  useLayoutEffect(() => {
    selectedRef.current?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [selectedID, detail.trace_id]);
  if (rows.length === 0) {
    return <p className="empty">no spans</p>;
  }
  const root = rows[0];
  if (!root) {
    return <p className="empty">no spans</p>;
  }
  const title = root.span.operation || root.span.service || detail.trace_id;
  const ticks = timeTicks(detail.duration_ns);
  const services = uniqueServices(rows);
  const path = detail.critical_path ?? [];
  const pathSet = new Set(path.map((s) => s.span_id));
  return (
    <>
      <header className="wf-head">
        <p className="wf-head-title">
          {onBack ? <TraceBack onClick={onBack} /> : null}
          <Trunc text={title} />
        </p>
        <p className="wf-head-meta">
          {formatTimestamp(detail.timestamp)}
          {" · "}
          {formatDuration(detail.duration_ns)}
          {path.length > 0 && (detail.critical_path_ns ?? 0) > 0 ? ` · path ${formatDuration(detail.critical_path_ns ?? 0)}` : ""}
          {" · "}
          {detail.span_count} spans
        </p>
        <p className="wf-legend">
          {services.map((name) => (
            <span key={name} className="wf-legend-item">
              <span className={`svc-swatch svc-${serviceRampIndex(name)}`} aria-hidden="true" />
              {onOpenService && name !== "(unknown)" ? (
                <Jump onClick={() => onOpenService(name)}>
                  <Trunc text={name} />
                </Jump>
              ) : (
                <Trunc text={name} />
              )}
            </span>
          ))}
        </p>
      </header>
      <div className="wf">
        <div className="wf-axis">
          <span className="wf-axis-cell">span</span>
          <span className="wf-axis-cell">service</span>
          <span className="wf-axis-cell wf-axis-track">
            {ticks.map((t, i) => (
              <span
                key={`${t.pct}-${t.label}`}
                className={tickClass(i)}
                style={{ left: `${t.pct}%` }}
              >
                {t.label}
              </span>
            ))}
          </span>
          <span className="wf-axis-cell wf-axis-dur">{formatDuration(detail.duration_ns)}</span>
        </div>
        {rows.map((row) => (
          <WaterfallRow
            key={row.span.span_id}
            row={row}
            ticks={ticks}
            selected={row.span.span_id === selectedID}
            rowRef={row.span.span_id === selectedID ? selectedRef : undefined}
            onPath={pathSet.has(row.span.span_id)}
            hasPath={pathSet.size > 0}
            onSelect={() => onSelect(row.span.span_id)}
          />
        ))}
      </div>
    </>
  );
}

function uniqueServices(rows: LaidSpan[]): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const row of rows) {
    const name = row.span.service || "(unknown)";
    if (seen.has(name)) {
      continue;
    }
    seen.add(name);
    out.push(name);
  }
  return out;
}

function tickClass(i: number): string {
  if (i === 0) {
    return "wf-tick is-start";
  }
  return "wf-tick";
}

function WaterfallRow({
  row,
  ticks,
  selected,
  rowRef,
  onPath,
  hasPath,
  onSelect,
}: {
  row: LaidSpan;
  ticks: { pct: number }[];
  selected: boolean;
  rowRef?: Ref<HTMLButtonElement>;
  onPath: boolean;
  hasPath: boolean;
  onSelect: () => void;
}) {
  const [tip, setTip] = useState<{ x: number; y: number } | null>(null);
  const err = row.span.status_code === 2;
  const ramp = serviceRampIndex(row.span.service || "");
  const op = row.span.operation || "(no op)";
  const service = row.span.service || "(unknown)";
  const dur = formatDuration(row.span.duration_ns);
  const cls = ["wf-row-hit"];
  if (selected) {
    cls.push("is-active");
  }
  if (err) {
    cls.push("is-err");
  }
  if (hasPath && onPath) {
    cls.push("is-path");
  }
  if (hasPath && !onPath) {
    cls.push("is-off-path");
  }
  const inBar = row.widthPct >= 22;
  const barCls = ["wf-bar"];
  if (err) {
    barCls.push("is-err");
  } else {
    barCls.push(`svc-${ramp}`);
  }
  if (onPath) {
    barCls.push("is-path");
  }
  return (
    <button
      ref={rowRef}
      type="button"
      className={cls.join(" ")}
      onClick={onSelect}
      onMouseMove={(e) => setTip({ x: e.clientX + 14, y: e.clientY + 16 })}
      onMouseLeave={() => setTip(null)}
    >
      <span className="wf-name">
        <TreeGutter row={row} />
        <span className="wf-op">
          <span className="trunc">{op}</span>
        </span>
      </span>
      <span className="wf-svc">
        <span className={`svc-swatch svc-${ramp}`} aria-hidden="true" />
        <span className="trunc">{service}</span>
      </span>
      <span className="wf-track">
        {ticks.map((t, i) => (
          <span key={`${t.pct}-${i}`} className="wf-gridline" style={{ left: `${t.pct}%` }} />
        ))}
        <span className={barCls.join(" ")} style={{ left: `${row.offsetPct}%`, width: `${row.widthPct}%` }}>
          {inBar ? <span className="wf-bar-ms">{dur}</span> : null}
        </span>
      </span>
      <span className="wf-dur">{dur}</span>
      {tip ? (
        <FloatTip x={tip.x} y={tip.y}>
          <p className="float-tip-title">{op}</p>
          <p>
            {service}
            {" · "}
            {kindLabel(row.span.kind)}
            {" · "}
            {dur}
            {err ? " · ERR" : ""}
          </p>
        </FloatTip>
      ) : null}
    </button>
  );
}

function TreeGutter({ row }: { row: LaidSpan }) {
  if (row.depth === 0) {
    return <span className="wf-tree" aria-hidden="true" />;
  }
  return (
    <span className="wf-tree" aria-hidden="true">
      {row.ancestorLast.map((closed, i) => (
        <span key={i} className={closed ? "wf-guide is-gap" : "wf-guide"} />
      ))}
      <span className={row.isLast ? "wf-guide is-end" : "wf-guide is-mid"} />
    </span>
  );
}

export function TraceBack({ onClick }: { onClick: () => void }) {
  return <GhostButton label="←" onClick={onClick} />;
}
