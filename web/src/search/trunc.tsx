import { useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";

type TruncProps = {
  text: string;
  className?: string;
};

export function Trunc({ text, className }: TruncProps) {
  const ref = useRef<HTMLSpanElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null);

  function onEnter(): void {
    const el = ref.current;
    if (!el) {
      return;
    }
    if (el.scrollWidth <= el.clientWidth + 1) {
      return;
    }
    const r = el.getBoundingClientRect();
    const x = Math.min(Math.max(8, r.left), window.innerWidth - 280);
    const y = r.bottom + 6;
    setPos({ x, y });
  }

  const cls = className ? `trunc ${className}` : "trunc";
  return (
    <>
      <span ref={ref} className={cls} onMouseEnter={onEnter} onMouseLeave={() => setPos(null)}>
        {text}
      </span>
      {pos ? <FloatTip x={pos.x} y={pos.y}>{text}</FloatTip> : null}
    </>
  );
}

export function FloatTip({ x, y, children }: { x: number; y: number; children: ReactNode }) {
  return createPortal(
    <div className="float-tip" style={{ left: x, top: y }}>
      {children}
    </div>,
    document.body,
  );
}
