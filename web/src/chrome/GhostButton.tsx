type GhostButtonProps = {
  label: string;
  active?: boolean;
  disabled?: boolean;
  className?: string;
  onClick: () => void;
};

export function GhostButton({
  label,
  active = false,
  disabled = false,
  className = "",
  onClick,
}: GhostButtonProps) {
  const cls = ["ghost"];
  if (active) {
    cls.push("is-active");
  }
  if (className) {
    cls.push(className);
  }
  return (
    <button type="button" className={cls.join(" ")} disabled={disabled} onClick={onClick}>
      {label}
    </button>
  );
}
