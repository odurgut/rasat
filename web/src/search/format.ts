export function formatDuration(ns: number): string {
  if (!Number.isFinite(ns) || ns <= 0) {
    return "0ms";
  }
  if (ns < 1_000_000) {
    const us = Math.round(ns / 1_000);
    if (us < 1) {
      return "<1us";
    }
    return `${us}us`;
  }
  if (ns < 1_000_000_000) {
    const ms = ns / 1_000_000;
    if (ms < 10) {
      return `${ms.toFixed(1)}ms`;
    }
    return `${Math.round(ms)}ms`;
  }
  const s = ns / 1_000_000_000;
  if (s < 10) {
    return `${s.toFixed(2)}s`;
  }
  return `${Math.round(s)}s`;
}

export function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }
  const t = d.toISOString();
  return `${t.slice(0, 10)} ${t.slice(11, 19)}`;
}

export function formatClock(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }
  return d.toISOString().slice(11, 19);
}

export function formatRate(n: number): string {
  if (!Number.isFinite(n) || n <= 0) {
    return "0";
  }
  if (n < 0.001) {
    return "<0.001";
  }
  if (n < 1) {
    return n.toFixed(3).replace(/0+$/, "").replace(/\.$/, "");
  }
  if (n < 100) {
    return n.toFixed(2).replace(/\.00$/, "");
  }
  return String(Math.round(n));
}

/** Fraction 0–1 → `6.3%`. One decimal; not a rounded integer. */
export function formatPct(rate: number): string {
  if (!Number.isFinite(rate) || rate <= 0) {
    return "0%";
  }
  const pct = rate * 100;
  if (pct < 0.1) {
    return "<0.1%";
  }
  if (pct >= 99.95) {
    return "100%";
  }
  return `${pct.toFixed(1)}%`;
}

/** Relative change when `prev` > 0. `496/47` → `+955%`. */
export function formatDeltaPct(prev: number, curr: number): string {
  if (!(prev > 0) || !Number.isFinite(curr)) {
    return "";
  }
  const pct = ((curr - prev) / prev) * 100;
  const rounded = Math.round(pct);
  if (rounded === 0) {
    return "";
  }
  return `${rounded > 0 ? "+" : ""}${rounded}%`;
}

export function formatShortID(id: string): string {
  const s = id.trim().toLowerCase();
  if (s.length <= 8) {
    return s || "—";
  }
  return s.slice(0, 8);
}

export function hourLabel(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return "--";
  }
  return d.toISOString().slice(11, 16);
}

export function kindLabel(kind: number): string {
  switch (kind) {
    case 1:
      return "internal";
    case 2:
      return "server";
    case 3:
      return "client";
    case 4:
      return "producer";
    case 5:
      return "consumer";
    default:
      return "unspecified";
  }
}

export type TimeTick = {
  label: string;
  pct: number;
};

const TICK_STEPS_NS = [
  1_000, 2_000, 5_000, 10_000, 20_000, 50_000, 100_000, 200_000, 500_000, 1_000_000, 2_000_000, 5_000_000, 10_000_000,
  20_000_000, 50_000_000, 100_000_000, 200_000_000, 500_000_000, 1_000_000_000, 2_000_000_000, 5_000_000_000,
  10_000_000_000, 30_000_000_000, 60_000_000_000,
];

export function timeTicks(durationNs: number): TimeTick[] {
  if (!Number.isFinite(durationNs) || durationNs <= 0) {
    return [{ label: "0", pct: 0 }];
  }
  const target = durationNs / 3;
  const last = TICK_STEPS_NS[TICK_STEPS_NS.length - 1] ?? durationNs;
  let step = last;
  for (const n of TICK_STEPS_NS) {
    if (n >= target) {
      step = n;
      break;
    }
  }
  const out: TimeTick[] = [];
  for (let t = 0; t < durationNs; t += step) {
    const pct = (t / durationNs) * 100;
    if (pct > 72) {
      break;
    }
    out.push({ label: t === 0 ? "0" : formatDuration(t), pct });
  }
  if (out.length === 0) {
    out.push({ label: "0", pct: 0 });
  }
  return out;
}
