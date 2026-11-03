type Props = {
  value: number;
  label: string;
};

export function ProgressBar({ value, label }: Props) {
  const percentage = Math.round(Math.min(1, Math.max(0, value)) * 100);
  return (
    <div className="progress-cell" aria-label={`${label}: ${percentage}%`}>
      <div className="progress-track"><span style={{ width: `${percentage}%` }} /></div>
      <span>{label}</span>
    </div>
  );
}
