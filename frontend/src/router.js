const listeners = new Set();
let current = parseHash();

function parseHash() {
  const raw = (typeof window !== "undefined" ? window.location.hash : "") || "";
  const stripped = raw.startsWith("#") ? raw.slice(1) : raw;
  const path = stripped || "/";
  const [pathname, query = ""] = path.split("?");
  const segments = pathname.split("/").filter(Boolean);
  const params = {};
  if (pathname.startsWith("/capabilities/")) params.id = segments[1];
  if (pathname.startsWith("/operations/")) params.tab = segments[1];
  if (pathname.startsWith("/releases/")) params.id = segments[1];
  if (pathname.startsWith("/versions/")) {
    params.id = segments[1];
    params.kind = segments[2] || "";
  }
  if (pathname.startsWith("/workspaces/")) params.id = segments[1];
  if (pathname.startsWith("/projects/")) params.id = segments[1];
  if (pathname.startsWith("/executions/")) params.id = segments[1];
  if (pathname.startsWith("/providers/")) params.id = segments[1];
  if (pathname.startsWith("/evals/")) params.id = segments[1];
  if (pathname.startsWith("/users/")) params.id = segments[1];
  const qs = Object.fromEntries(new URLSearchParams(query));
  return { path: pathname, segments, params, query: qs };
}

export function currentRoute() {
  return current;
}

export function go(path, options = {}) {
  let target = path.startsWith("#") ? path : `#${path}`;
  if (target === "#" || target === "") target = "#/";
  if (typeof window !== "undefined") {
    if (options.replace) {
      window.location.replace(target);
    } else {
      window.location.hash = target;
    }
  }
}

export function initRouter({ onChange }) {
  if (typeof window === "undefined") return;
  const onHashChange = () => {
    const next = parseHash();
    if (next.path === current.path && JSON.stringify(next.query) === JSON.stringify(current.query)) {
      if (next.params.id !== current.params.id) {
        current = next;
        onChange(current);
      }
      return;
    }
    current = next;
    listeners.forEach((fn) => { try { fn(current); } catch { /* ignore */ } });
    onChange(current);
  };
  window.addEventListener("hashchange", onHashChange);
  onHashChange();
  return () => window.removeEventListener("hashchange", onHashChange);
}

export function subscribeRoute(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function buildPath(pathname, query) {
  if (!query || !Object.keys(query).length) return pathname;
  const qs = new URLSearchParams(query);
  return `${pathname}?${qs.toString()}`;
}
