interface SkeletonProps {
  width?: string | number;
  height?: string | number;
  className?: string;
  rounded?: boolean;
}

export function Skeleton({ width, height, className, rounded }: SkeletonProps) {
  return (
    <div
      className={`animate-pulse bg-[var(--bg-2)] rounded-[var(--radius-sm)]${rounded ? ' rounded-full' : ''}${className ? ` ${className}` : ''}`}
      style={{
        width: typeof width === 'number' ? `${width}px` : width,
        height: typeof height === 'number' ? `${height}px` : height,
      }}
    />
  );
}
