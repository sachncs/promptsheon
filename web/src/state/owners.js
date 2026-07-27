import * as api from "../api.js";

const owners = new Map();

export async function ensureOwnerIndex() {
  if (owners.size) return owners;
  // /api/v1/users requires the UserManage permission (admin only). A reader
  // or writer key gets 403 here. In that case we fall back to the empty
  // index and the dashboard renders user ids in place of display names.
  const result = await api.listUsers(200);
  if (!result || !result.ok || !Array.isArray(result.data)) return owners;
  for (const user of result.data) owners.set(user.id, user.name || user.email || user.id);
  return owners;
}

export function ownerName(id) {
  if (!id) return "—";
  const cached = owners.get(id);
  if (cached) return cached;
  // Owner id we haven't loaded yet (admin only). Show a short
  // readable form rather than the full opaque id.
  if (typeof id === "string" && id.length > 12) return `…${id.slice(-8)}`;
  return id || "—";
}

export function setOwners(list) {
  owners.clear();
  for (const user of list || []) owners.set(user.id, user.name || user.email || user.id);
}

export function getOwners() {
  return owners;
}
