const STORAGE_KEY = "promptsheon.settings.v1";

const defaults = {
  apiBase: "",
  apiKey: ""
};

const subscribers = new Set();

export function loadSettings() {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return { ...defaults };
    const parsed = JSON.parse(raw);
    return { ...defaults, ...parsed };
  } catch {
    return { ...defaults };
  }
}

export function saveSettings(partial) {
  const next = { ...loadSettings(), ...partial };
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
  } catch {
    return next;
  }
  for (const fn of subscribers) {
    try { fn(next); } catch { /* ignore */ }
  }
  return next;
}

export function clearSettings() {
  try {
    window.localStorage.removeItem(STORAGE_KEY);
  } catch {
    return;
  }
  for (const fn of subscribers) {
    try { fn({ ...defaults }); } catch { /* ignore */ }
  }
}

export function watchSettings(fn) {
  subscribers.add(fn);
  return () => subscribers.delete(fn);
}
