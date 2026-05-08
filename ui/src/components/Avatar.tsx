export function Avatar({ letter = 'O', size = 28 }: { letter?: string; size?: number }) {
  return (
    <div
      className="avatar"
      style={{ width: size, height: size, fontSize: size * 0.43 }}
      aria-label={`Account: ${letter}`}
    >
      {letter.charAt(0).toUpperCase()}
    </div>
  );
}
