import { useEffect, useRef, useState } from "react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import { axisTimeLabel, cssColor, hasChartData, toneVar, uniqueTickLabels, type ChartSeries } from "./chart";
import { FloatTip } from "./trunc";

type DashChartProps = {
  title: string;
  times: number[];
  series: ChartSeries[];
  formatY: (n: number) => string;
};

type Hover = { i: number; sidx: number; x: number; y: number };

const FONT = '12px "Ioskeley Mono", ui-monospace, "SF Mono", SFMono-Regular, Menlo, Monaco, monospace';

export function DashChart({ title, times, series, formatY }: DashChartProps) {
  const host = useRef<HTMLDivElement>(null);
  const plotRef = useRef<uPlot | null>(null);
  const payload = useRef({ times, series, formatY });
  const [theme, setTheme] = useState(() => document.documentElement.className);
  const [hover, setHover] = useState<Hover | null>(null);
  const empty = times.length === 0 || !hasChartData(series);
  payload.current = { times, series, formatY };
  const seriesKey = series.map((s) => `${s.label}:${s.tone}:${s.fill ? 1 : 0}`).join("|");

  useEffect(() => {
    const obs = new MutationObserver(() => setTheme(document.documentElement.className));
    obs.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    return () => obs.disconnect();
  }, []);

  useEffect(() => {
    const el = host.current;
    if (!el || empty) {
      return;
    }

    let alive = true;
    let plot: uPlot | null = null;
    let building = false;
    let over = false;
    const { times: t0, series: s0, formatY: fmt } = payload.current;
    const data: uPlot.AlignedData = [t0, ...s0.map((s) => s.values)];
    const first = t0[0] ?? 0;
    const last = t0[t0.length - 1] ?? first;
    const spanS = Math.max(last - first, 1);

    const tearDown = () => {
      plot?.destroy();
      plot = null;
      plotRef.current = null;
      el.replaceChildren();
    };

    const build = (width: number, height: number) => {
      const muted = cssColor("--color-text-muted");
      const grid = cssColor("--color-border-subtle");
      const spline = uPlot.paths.spline?.();
      const latencyBand =
        s0.length === 3
          ? [
              {
                series: [3, 1] as [number, number],
                fill: cssColor(toneVar(s0[2]?.tone ?? "svc-2"), 0.12),
              },
            ]
          : undefined;
      const opts: uPlot.Options = {
        width,
        height,
        class: "dash-uplot",
        pxAlign: false,
        padding: [8, 10, 0, 0],
        legend: { show: false },
        cursor: {
          x: true,
          y: false,
          drag: { setScale: false, x: false, y: false },
          focus: { prox: 1e6 },
          points: {
            one: true,
            size: 8,
            width: 1,
            fill: (u, seriesIdx) => seriesStroke(u, seriesIdx),
            stroke: () => cssColor("--color-bg-body"),
          },
        },
        tzDate: (ts) => uPlot.tzDate(new Date(ts * 1000), "Etc/UTC"),
        scales: {
          x: { time: true },
          y: {
            range: (_u, _min, max) => [0, max <= 0 ? 1 : max * 1.08],
          },
        },
        axes: [
          {
            stroke: muted,
            font: FONT,
            size: 28,
            gap: 6,
            space: 88,
            grid: { stroke: grid, width: 1 },
            ticks: { stroke: grid, width: 1, size: 3 },
            values: (_u, splits) => uniqueTickLabels(splits, (ts) => axisTimeLabel(ts, spanS)),
          },
          {
            stroke: muted,
            font: FONT,
            size: 56,
            gap: 8,
            space: 40,
            grid: { stroke: grid, width: 1 },
            ticks: { show: false },
            values: (_u, splits) => uniqueTickLabels(splits, (n) => fmt(n)),
          },
        ],
        bands: latencyBand,
        series: [
          {},
          ...s0.map((s) => {
            const stroke = cssColor(toneVar(s.tone));
            return {
              label: s.label,
              stroke,
              width: 1,
              fill: s.fill ? cssColor(toneVar(s.tone), 0.12) : undefined,
              spanGaps: false,
              pxAlign: false,
              paths: s.fill ? undefined : spline,
              points: s.fill ? { show: false } : squareMarks(stroke),
            };
          }),
        ],
        hooks: {
          ready: [
            (u) => {
              u.over.addEventListener("mouseenter", () => {
                over = true;
              });
              u.over.addEventListener("mouseleave", () => {
                over = false;
                if (alive) {
                  setHover(null);
                }
              });
            },
          ],
          setCursor: [
            (u) => {
              if (!alive || !over) {
                return;
              }
              const i = u.cursor.idx;
              if (i == null) {
                setHover(null);
                return;
              }
              const locked = lockToSeries(u, i, u.cursor.top);
              if (!locked) {
                setHover(null);
                return;
              }
              setHover({ i, sidx: locked.sidx, ...tipPos(u.over, locked.left, locked.top) });
            },
          ],
        },
      };
      plot = new uPlot(opts, data, el);
      plotRef.current = plot;
    };

    const layout = () => {
      if (!alive || building) {
        return;
      }
      const width = Math.floor(el.clientWidth);
      const height = Math.floor(el.clientHeight);
      if (width < 64 || height < 48) {
        return;
      }
      if (plot) {
        if (plot.width !== width || plot.height !== height) {
          plot.setSize({ width, height });
        }
        return;
      }
      building = true;
      try {
        build(width, height);
      } finally {
        building = false;
      }
    };

    layout();
    const ro = new ResizeObserver(layout);
    ro.observe(el);

    return () => {
      alive = false;
      ro.disconnect();
      tearDown();
      setHover(null);
    };
  }, [empty, theme, seriesKey]);

  useEffect(() => {
    const u = plotRef.current;
    if (!u || empty) {
      return;
    }
    u.setData([times, ...series.map((s) => s.values)]);
  }, [empty, times, series]);

  const tip = hover && times[hover.i] != null ? hover : null;

  return (
    <section className="dash-chart">
      <div className="dash-chart-head">
        <span className="dash-chart-title">{title}</span>
        <span className="wf-legend">
          {series.map((s) => (
            <span key={s.label} className="wf-legend-item">
              <span className={s.tone === "err" ? "svc-swatch is-err" : `svc-swatch ${s.tone}`} aria-hidden="true" />
              {s.label}
            </span>
          ))}
        </span>
      </div>
      <div className="dash-plot-wrap">
        <div ref={host} className="dash-plot" />
        {empty ? <p className="empty">no data</p> : null}
      </div>
      {tip ? (
        <FloatTip x={tip.x} y={tip.y}>
          <div className="chart-tip">
            <span className="chart-tip-time">{hoverClock(times[tip.i] ?? 0)}</span>
            {series.map((s, si) => {
              const v = s.values[tip.i];
              if (v == null || !Number.isFinite(v)) {
                return null;
              }
              return (
                <span
                  key={s.label}
                  className={si + 1 === tip.sidx ? "chart-tip-row is-active" : "chart-tip-row"}
                >
                  <span className="wf-legend-item">
                    <span className={s.tone === "err" ? "svc-swatch is-err" : `svc-swatch ${s.tone}`} aria-hidden="true" />
                    {s.label}
                  </span>
                  <span className="insight-dur">{formatY(v)}</span>
                </span>
              );
            })}
          </div>
        </FloatTip>
      ) : null}
    </section>
  );
}

function seriesStroke(u: uPlot, seriesIdx: number): string {
  const stroke = u.series[seriesIdx]?.stroke;
  return typeof stroke === "string" ? stroke : cssColor("--color-text-heading");
}

function lockToSeries(
  u: uPlot,
  i: number,
  mouseTop: number | null | undefined,
): { sidx: number; left: number; top: number } | null {
  const xv = u.data[0]?.[i];
  if (xv == null || !Number.isFinite(xv)) {
    return null;
  }
  const left = u.valToPos(xv, "x");
  let sidx = -1;
  let top = 0;
  let best = Infinity;
  for (let s = 1; s < u.series.length; s++) {
    const ys = u.data[s];
    const v = ys?.[i];
    if (v == null || !Number.isFinite(v)) {
      continue;
    }
    const y = u.valToPos(v, u.series[s]?.scale ?? "y");
    const dist = mouseTop == null ? 0 : Math.abs(y - mouseTop);
    if (sidx < 0 || dist < best) {
      sidx = s;
      top = y;
      best = dist;
    }
    if (mouseTop == null) {
      break;
    }
  }
  if (sidx < 0) {
    return null;
  }
  return { sidx, left, top };
}

function hoverClock(ts: number): string {
  const iso = new Date(ts * 1000).toISOString();
  if (!Number.isFinite(ts) || Number.isNaN(Date.parse(iso))) {
    return "";
  }
  return `${iso.slice(0, 10)} ${iso.slice(11, 19)}`;
}

function tipPos(el: HTMLElement, left: number, top: number): { x: number; y: number } {
  const r = el.getBoundingClientRect();
  const tipW = 196;
  const tipH = 112;
  let x = r.left + left + 14;
  let y = r.top + top + 14;
  if (x + tipW > window.innerWidth - 8) {
    x = r.left + left - tipW;
  }
  if (y + tipH > window.innerHeight - 8) {
    y = r.top + top - tipH;
  }
  return { x: Math.max(8, x), y: Math.max(8, y) };
}

/** 3px squares — radius 0, same as the rest of the UI. */
function squareMarks(stroke: string): uPlot.Series.Points {
  return {
    show: true,
    size: 3,
    width: 1,
    space: 0,
    stroke,
    fill: stroke,
    paths: (u, sidx, i0, i1, filtIdxs) => {
      const xs = u.data[0];
      const ys = u.data[sidx];
      if (!xs || !ys) {
        return { stroke: null, fill: null };
      }
      const scale = u.series[sidx]?.scale ?? "y";
      const p = new Path2D();
      const visit = (i: number) => {
        const xv = xs[i];
        const yv = ys[i];
        if (xv == null || yv == null || !Number.isFinite(xv) || !Number.isFinite(yv)) {
          return;
        }
        const x = Math.round(u.valToPos(xv, "x", true));
        const y = Math.round(u.valToPos(yv, scale, true));
        p.rect(x - 1, y - 1, 3, 3);
      };
      if (filtIdxs) {
        for (const i of filtIdxs) {
          visit(i);
        }
      } else {
        for (let i = i0; i <= i1; i++) {
          visit(i);
        }
      }
      return { stroke: p, fill: p };
    },
  };
}
