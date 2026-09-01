export type ChartTone = "svc-0" | "svc-1" | "svc-2" | "svc-3" | "svc-4" | "svc-5" | "err";

export type ChartSeries = {
  label: string;
  values: (number | null)[];
  tone: ChartTone;
  /** Area to y=0. Traffic and errors; not latency. */
  fill?: boolean;
};

export function toneVar(tone: ChartTone): string {
  return tone === "err" ? "--color-error" : `--color-${tone}`;
}

/** Read a ground/signal RGB triplet token as a CSS color. */
export function cssColor(token: string, alpha?: number): string {
  const raw = getComputedStyle(document.documentElement).getPropertyValue(token).trim();
  const parts = raw.split(/\s+/);
  const r = parts[0];
  const g = parts[1];
  const b = parts[2];
  if (!r || !g || !b) {
    return alpha == null ? "rgb(128 128 128)" : "rgb(128 128 128 / 0.12)";
  }
  if (alpha == null) {
    return `rgb(${r} ${g} ${b})`;
  }
  return `rgb(${r} ${g} ${b} / ${alpha})`;
}

export function hasChartData(series: ChartSeries[]): boolean {
  return series.some((s) => s.values.some((v) => v != null && Number.isFinite(v)));
}

/** Short UTC tick: HH:MM under 3 days, MM-DD otherwise. */
export function axisTimeLabel(ts: number, spanS: number): string {
  const iso = new Date(ts * 1000).toISOString();
  if (spanS >= 3 * 24 * 3600) {
    return iso.slice(5, 10);
  }
  return iso.slice(11, 16);
}

export function uniqueTickLabels(splits: number[], label: (ts: number) => string): (string | null)[] {
  let prev = "";
  return splits.map((s) => {
    if (s == null || !Number.isFinite(s)) {
      return null;
    }
    const lab = label(s);
    if (lab === prev) {
      return null;
    }
    prev = lab;
    return lab;
  });
}
