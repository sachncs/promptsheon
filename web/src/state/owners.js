const owners = new Map();

async function fetchJSON(path, opts) {
  const r = await fetch(path, opts);
  return r.ok ? r.json() : null;
}

export async function ensureOwnerIndex() {
  if (owners.size) return owners;
  const data = await fetchJSON("/api/v1/users?limit=200");
  if (!Array.isArray(data)) return owners;
  for (const user of data) owners.set(user.id, user.name || user.email || user.id);
  return owners;
}

export function ownerName(id) {
  if (!id) return "—";
  return owners.get(id) || id;
}

export function setOwners(list) {
  owners.clear();
  for (const user of list || []) owners.set(user.id, user.name || user.email || user.id);
}

export function getOwners() {
  return owners;
}
