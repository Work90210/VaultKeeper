type PillStatus =
  | 'sealed'
  | 'hold'
  | 'draft'
  | 'live'
  | 'disc'
  | 'broken'
  | 'pseud'
  | 'active'
  | 'locked'
  | 'complete';

interface StatusPillProps {
  status: PillStatus;
  children?: React.ReactNode;
}

const STATUS_LABELS: Record<PillStatus, string> = {
  sealed: 'Sealed',
  hold: 'Hold',
  draft: 'Draft',
  live: 'Live',
  disc: 'Disc',
  broken: 'Broken',
  pseud: 'Pseud',
  active: 'Active',
  locked: 'Locked',
  complete: 'Complete',
};

export function StatusPill({ status, children }: StatusPillProps) {
  return (
    <span className={`pl ${status}`}>
      {children ?? STATUS_LABELS[status]}
    </span>
  );
}
