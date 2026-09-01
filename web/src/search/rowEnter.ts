import { useLayoutEffect, useRef, useState } from "react";

/** IDs that arrived after the current list epoch (live prepend). Initial search does not enter. */
export function useRowEnter(ids: readonly string[], epoch: number): ReadonlySet<string> {
  const seenRef = useRef(new Set<string>());
  const epochRef = useRef(epoch);
  const idsRef = useRef(ids);
  idsRef.current = ids;
  const [enter, setEnter] = useState<ReadonlySet<string>>(() => new Set());
  const key = ids.join("\n");

  useLayoutEffect(() => {
    const current = idsRef.current;
    if (epochRef.current !== epoch) {
      epochRef.current = epoch;
      seenRef.current = new Set(current);
      setEnter((prev) => (prev.size === 0 ? prev : new Set()));
      return;
    }
    const seen = seenRef.current;
    const fresh: string[] = [];
    for (const id of current) {
      if (!seen.has(id)) {
        seen.add(id);
        fresh.push(id);
      }
    }
    if (fresh.length === 0) {
      return;
    }
    setEnter(new Set(fresh));
  }, [key, epoch]);

  return enter;
}
