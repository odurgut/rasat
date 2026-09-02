import { useEffect, useRef, useState, type CSSProperties, type ReactNode, type TransitionEvent } from "react";
import { getBuild, type BuildInfo } from "../search/api";
import { FloatTip } from "../search/trunc";
import type { Theme } from "../theme";
import { views, type View } from "../views";

const BRAND = "rasat";
const BRAND_REELS = [
  "kwnxqhdbyz",
  "mvyuzpgenx",
  "txkbnwqhdm",
  "dhyvmuxpqk",
  "qwknxbdzvn",
];
const REEL_FILL = BRAND_REELS[0]?.length ?? 10;
const REEL_LETTER = REEL_FILL + 1;
const REEL_LOOP = 2 * (REEL_FILL + 1);

type ChromeProps = {
  view: View;
  theme: Theme;
  onView: (view: View) => void;
  onTheme: (theme: Theme) => void;
};

export function Chrome({ view, theme, onView, onTheme }: ChromeProps): ReactNode {
  const [build, setBuild] = useState<BuildInfo | null>(null);

  useEffect(() => {
    const ac = new AbortController();
    void getBuild(ac.signal).then((info) => {
      if (info) {
        setBuild(info);
      }
    });
    return () => ac.abort();
  }, []);

  return (
    <header className="shell-rail">
      <h1 className="brand">
        <BrandMark onHome={() => onView("overview")} />
      </h1>
      <nav className="nav" aria-label="primary">
        {views.map((v) => (
          <button
            key={v}
            type="button"
            className={view === v ? "rail-item rail-tab is-active" : "rail-item rail-tab"}
            onClick={() => onView(v)}
          >
            {v}
          </button>
        ))}
      </nav>
      <div className="shell-rail-foot">
        {build ? <VersionMark info={build} /> : null}
        <button
          type="button"
          className="theme-pair"
          aria-label={theme === "dark" ? "switch to light" : "switch to dark"}
          onClick={() => onTheme(theme === "dark" ? "light" : "dark")}
        >
          <span className={theme === "dark" ? "is-active" : ""}>dark</span>
          <span className="theme-sep" aria-hidden="true">
            /
          </span>
          <span className={theme === "light" ? "is-active" : ""}>light</span>
        </button>
      </div>
    </header>
  );
}

function buildLabel(info: BuildInfo): string {
  if (info.commit && !info.version.includes(info.commit)) {
    return `${info.version} ${info.commit}`;
  }
  return info.version;
}

function VersionMark({ info }: { info: BuildInfo }): ReactNode {
  const [copied, setCopied] = useState(false);
  const [tip, setTip] = useState<{ x: number; y: number } | null>(null);
  const full = buildLabel(info);

  useEffect(() => {
    if (!copied) {
      return;
    }
    const t = window.setTimeout(() => setCopied(false), 1200);
    return () => window.clearTimeout(t);
  }, [copied]);

  return (
    <>
      <button
        type="button"
        className="rail-version"
        aria-label={`copy ${full}`}
        onMouseEnter={(e) => {
          const r = e.currentTarget.getBoundingClientRect();
          setTip({ x: Math.min(Math.max(8, r.left), window.innerWidth - 280), y: r.bottom + 6 });
        }}
        onMouseLeave={() => setTip(null)}
        onClick={() => {
          void navigator.clipboard.writeText(full).then(
            () => setCopied(true),
            () => undefined,
          );
        }}
      >
        {copied ? "copied" : info.version}
      </button>
      {tip ? <FloatTip x={tip.x} y={tip.y}>{full}</FloatTip> : null}
    </>
  );
}

function BrandBox({ i }: { i: number }): ReactNode {
  return (
    <span className="brand-slot">
      <span className={`svc-swatch svc-${i} brand-box`} />
    </span>
  );
}

function BrandGlyphs({ strip, prefix }: { strip: string; prefix: string }): ReactNode {
  return Array.from(strip).map((g, k) => (
    <span key={`${prefix}-${k}`} className="brand-slot">
      {g}
    </span>
  ));
}

function BrandMark({ onHome }: { onHome: () => void }): ReactNode {
  const ref = useRef<HTMLAnchorElement>(null);

  function spinOn(): void {
    const a = ref.current;
    if (!a) {
      return;
    }
    a.classList.add("is-on");
    a.classList.remove("is-away");
    a.querySelectorAll(".brand-reel").forEach((reel) => reel.classList.remove("is-home"));
  }

  function spinOff(): void {
    const a = ref.current;
    if (!a) {
      return;
    }
    a.classList.remove("is-on");
    a.classList.add("is-away");
  }

  function onTransitionEnd(e: TransitionEvent<HTMLAnchorElement>): void {
    if (e.propertyName !== "transform") {
      return;
    }
    const reel = e.target;
    if (!(reel instanceof HTMLElement) || !reel.classList.contains("brand-reel")) {
      return;
    }
    const a = ref.current;
    if (!a || a.classList.contains("is-on")) {
      return;
    }
    reel.classList.add("is-home");
  }

  return (
    <a
      ref={ref}
      href="/"
      aria-label="rasat"
      style={{ "--reel-letter": REEL_LETTER, "--reel-loop": REEL_LOOP } as CSSProperties}
      onClick={(e) => {
        e.preventDefault();
        onHome();
      }}
      onMouseEnter={spinOn}
      onMouseLeave={spinOff}
      onFocus={spinOn}
      onBlur={spinOff}
      onTransitionEnd={onTransitionEnd}
    >
      {BRAND_REELS.map((strip, i) => (
        <span key={`${BRAND[i]}-${i}`} className={`brand-cell svc-${i}`}>
          <span className="brand-reel" aria-hidden="true">
            <BrandBox i={i} /> <BrandGlyphs strip={strip} prefix="a" />
            <span className="brand-slot">{BRAND[i]}</span>
            <BrandGlyphs strip={strip} prefix="b" />
            <BrandBox i={i} />
          </span>
        </span>
      ))}
    </a>
  );
}
