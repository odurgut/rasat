type PrimaryButtonProps = {
  label: string;
  onClick?: () => void;
  type?: "button" | "submit";
  disabled?: boolean;
};

export function PrimaryButton({ label, onClick, type = "button", disabled = false }: PrimaryButtonProps) {
  return (
    <button type={type} className="primary" onClick={onClick} disabled={disabled}>
      {label}
    </button>
  );
}
