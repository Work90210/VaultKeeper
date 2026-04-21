interface AvatarUser {
  initial: string;
  color?: 'a' | 'b' | 'c' | 'd' | 'e';
}

interface AvatarStackProps {
  users: AvatarUser[];
  max?: number;
}

export function AvatarStack({ users, max }: AvatarStackProps) {
  const limit = max ?? users.length;
  const visible = users.slice(0, limit);
  const remaining = users.length - limit;

  return (
    <div className="avs">
      {visible.map((user, i) => (
        <div key={i} className={`av${user.color ? ` ${user.color}` : ''}`}>
          {user.initial}
        </div>
      ))}
      {remaining > 0 && (
        <div className="av more">+{remaining}</div>
      )}
    </div>
  );
}
