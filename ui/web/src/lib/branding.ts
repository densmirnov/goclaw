const defaultAppName = "GoClaw";

export const APP_NAME =
  (import.meta.env.VITE_APP_NAME as string | undefined)?.trim() ||
  defaultAppName;

export const APP_SHORT_NAME = APP_NAME
  .split(/\s+/)
  .filter(Boolean)
  .map((part) => part[0]?.toUpperCase() || "")
  .join("")
  .slice(0, 2) || "GC";
