import type { ReactNode } from "react";
import "./TabKeepAlive.css";

type Props = {
  active: boolean;
  /** Скрыть вкладку (оверлей тренировки / поддержки поверх). */
  hidden?: boolean;
  className?: string;
  children: ReactNode;
};

/** Держит дочерний экран смонтированным; показывает только при active. */
export function TabKeepAlive({ active, hidden = false, className, children }: Props) {
  const show = active && !hidden;
  return (
    <div
      className={[
        "tab-pane",
        show ? "tab-pane--active" : "",
        hidden ? "tab-pane--overlay-hidden" : "",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      aria-hidden={!show}
      {...(!show ? { inert: true as const } : {})}
    >
      {children}
    </div>
  );
}
