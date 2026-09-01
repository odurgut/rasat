import { hourLabel } from "./format";
import type { TraceListRow } from "./api";

export type HourGroup = {
  hour: string;
  rows: TraceListRow[];
};

export function groupByHour(rows: TraceListRow[]): HourGroup[] {
  const groups: HourGroup[] = [];
  const index = new Map<string, HourGroup>();
  for (const row of rows) {
    const hour = hourLabel(row.timestamp);
    let g = index.get(hour);
    if (!g) {
      g = { hour, rows: [] };
      index.set(hour, g);
      groups.push(g);
    }
    g.rows.push(row);
  }
  return groups;
}
