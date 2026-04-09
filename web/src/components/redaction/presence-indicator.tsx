'use client';

interface PresenceUser {
  id: string;
  name: string;
  color: string;
}

interface PresenceIndicatorProps {
  users: PresenceUser[];
}

const USER_COLORS = [
  '#4a7c59', // olive
  '#8b6f47', // sienna
  '#5b6abf', // slate blue
  '#9b5f5f', // dusty rose
  '#6a8fa7', // steel
  '#7a6b8f', // mauve
  '#8f7a3f', // gold
  '#5f8f7a', // teal
];

export function getUserColor(index: number): string {
  return USER_COLORS[index % USER_COLORS.length];
}

export function PresenceIndicator({ users }: PresenceIndicatorProps) {
  if (users.length === 0) return null;

  return (
    <div className="flex items-center gap-[var(--space-xs)]">
      <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
        {users.length} online
      </span>
      <div className="flex -space-x-1.5">
        {users.map((user) => (
          <div
            key={user.id}
            className="w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-medium border-2"
            style={{
              backgroundColor: user.color,
              borderColor: 'var(--bg-elevated)',
              color: '#fff',
            }}
            title={user.name}
          >
            {user.name.charAt(0).toUpperCase()}
          </div>
        ))}
      </div>
    </div>
  );
}
