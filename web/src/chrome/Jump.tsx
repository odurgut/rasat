import type { MouseEvent, ReactNode } from "react";

type JumpProps = {
  children: ReactNode;
  onClick: () => void;
  title?: string;
};

/** In-page jump. Looks like a prose link (underline), not a ghost/primary. */
export function Jump({ children, onClick, title }: JumpProps) {
  return (
    <button
      type="button"
      className="jump"
      title={title}
      onClick={(e: MouseEvent<HTMLButtonElement>) => {
        e.stopPropagation();
        onClick();
      }}
    >
      {children}
    </button>
  );
}
