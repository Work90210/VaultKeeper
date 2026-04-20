import React from 'react';

function stripMotionProps(props: Record<string, unknown>) {
  const {
    initial,
    animate,
    exit,
    whileInView,
    whileHover,
    viewport,
    transition,
    variants,
    layout,
    layoutId,
    ...rest
  } = props;
  return rest;
}

// Cache component references so React doesn't unmount/remount on re-render
const componentCache = new Map<string, React.ForwardRefExoticComponent<Record<string, unknown>>>();

export const motion = new Proxy(
  {},
  {
    get(_target, prop: string) {
      if (!componentCache.has(prop)) {
        const Component = React.forwardRef(function MotionComponent(
          props: Record<string, unknown>,
          ref: React.Ref<unknown>,
        ) {
          return React.createElement(prop, { ...stripMotionProps(props), ref });
        });
        Component.displayName = `motion.${prop}`;
        componentCache.set(prop, Component);
      }
      return componentCache.get(prop)!;
    },
  },
);

export function AnimatePresence({
  children,
}: {
  children?: React.ReactNode;
  initial?: boolean;
}) {
  return <>{children}</>;
}

export function useScroll() {
  return { scrollYProgress: { get: () => 0 } };
}

export function useTransform() {
  return 1;
}

export function useInView() {
  return true;
}
