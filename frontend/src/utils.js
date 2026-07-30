export function escape(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;"
  })[ch]);
}

export const formatCompact = (value) => {
  if (typeof value !== "number" || Number.isNaN(value)) return "0";
  return new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value);
};

export const formatInteger = (value) => {
  if (typeof value !== "number" || Number.isNaN(value)) return "0";
  return new Intl.NumberFormat("en-US").format(value);
};

export const formatMoney = (value) => {
  if (typeof value !== "number" || Number.isNaN(value)) return "$0";
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 }).format(value);
};

export const formatPercent = (value) => {
  if (typeof value !== "number" || Number.isNaN(value)) return "—";
  return `${value.toFixed(2)}%`;
};

export const formatRelative = (iso) => {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  const diff = (Date.now() - date.getTime()) / 1000;
  if (Math.abs(diff) < 60) return `${Math.round(diff)}s ago`;
  if (Math.abs(diff) < 3600) return `${Math.round(diff / 60)}m ago`;
  if (Math.abs(diff) < 86400) return `${Math.round(diff / 3600)}h ago`;
  if (Math.abs(diff) < 604800) return `${Math.round(diff / 86400)}d ago`;
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric" }).format(date);
};

export function apiStatusLabel(result) {
  if (!result) return "Offline";
  if (result.ok) return "OK";
  if (result.status === 0) return result.error || "Network error";
  if (result.status === 401) return "Auth required";
  if (result.status === 403) return "Forbidden";
  if (result.status === 404) return "Not found";
  if (result.status === 409) return result.error || "Conflict";
  if (result.status === 429) return "Rate limited";
  if (result.status >= 500) return "API error";
  return `HTTP ${result.status}`;
}
