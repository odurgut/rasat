const SIZE = 32;
const BOX = 16;
const N = 5;
const PERIOD_MS = 1400;

export function startFavicon(): void {
  const canvas = document.createElement("canvas");
  canvas.width = SIZE;
  canvas.height = SIZE;
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    return;
  }

  let tick = 0;
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!link) {
    link = document.createElement("link");
    link.rel = "icon";
    document.head.appendChild(link);
  }

  const paint = (): void => {
    const colors = svcColors();
    const c = colors[tick % N] ?? colors[0];
    ctx.clearRect(0, 0, SIZE, SIZE);
    if (c) {
      ctx.fillStyle = c;
      const o = Math.round((SIZE - BOX) / 2);
      ctx.fillRect(o, o, BOX, BOX);
    }
    link.href = canvas.toDataURL("image/png");
  };

  paint();

  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)");
  let timer = 0;
  const arm = (): void => {
    if (timer) {
      window.clearInterval(timer);
      timer = 0;
    }
    if (reduced.matches) {
      return;
    }
    timer = window.setInterval(() => {
      tick = (tick + 1) % N;
      paint();
    }, PERIOD_MS);
  };
  arm();
  reduced.addEventListener("change", arm);

  const obs = new MutationObserver(paint);
  obs.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
}

function svcColors(): string[] {
  const s = getComputedStyle(document.documentElement);
  const out: string[] = [];
  for (let i = 0; i < N; i++) {
    const raw = s.getPropertyValue(`--color-svc-${i}`).trim();
    out.push(raw ? `rgb(${raw})` : "rgb(128 128 128)");
  }
  return out;
}
