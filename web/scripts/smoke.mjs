#!/usr/bin/env node
import puppeteer from "puppeteer-core";
import { spawn } from "node:child_process";
import { setTimeout as wait } from "node:timers/promises";
import { mkdir, rm } from "node:fs/promises";
import { existsSync } from "node:fs";
import path from "node:path";
import process from "node:process";

import { fileURLToPath } from "node:url";
const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
process.chdir(REPO);

const HEADLESS = process.env.SMOKE_BROWSER || "/Users/sachin/Library/Caches/ms-playwright/chromium_headless_shell-1217/chrome-headless-shell-mac-arm64/chrome-headless-shell";
const BASE = process.env.SMOKE_URL || "http://127.0.0.1:8080";
const FRONTEND = process.env.SMOKE_FRONTEND || "http://localhost:4173";
const SMOKE_BOOT_DAEMON = process.env.SMOKE_BOOT_DAEMON === "1";
const SMOKE_BOOT_VITE = process.env.SMOKE_BOOT_VITE === "1";

async function killPid(pid) {
  if (!pid) return;
  try { process.kill(pid, "SIGTERM"); } catch {}
  await wait(200);
  try { process.kill(pid, "SIGKILL"); } catch {}
}

async function waitFor(url, attempts = 60) {
  for (let i = 0; i < attempts; i++) {
    try {
      const r = await fetch(url, { signal: AbortSignal.timeout(800) });
      if (r.status < 500) return r;
    } catch {}
    await wait(500);
  }
  throw new Error(`timeout waiting for ${url}`);
}

function spawnChild(cmd, args, env, cwd = REPO) {
  const child = spawn(cmd, args, { cwd, env, stdio: ["ignore", "pipe", "pipe"], shell: false });
  child.stdout.on("data", () => {});
  child.stderr.on("data", (chunk) => process.stderr.write(`[child ${cmd}] ${chunk}`));
  child.on("error", (err) => process.stderr.write(`[smoke] child ${cmd} error: ${err.message}\n`));
  return child;
}

async function bootstrapDaemon() {
  const dbPath = path.resolve(REPO, "promptsheon.db");
  for (const f of [dbPath, dbPath + "-shm", dbPath + "-wal"]) { try { await rm(f, { force: true }); } catch {} }
  const env = {
    ...process.env,
    PROMPTSHEON_AUTH: "false",
    PROMPTSHEON_ADDR: "127.0.0.1:8080",
    PROMPTSHEON_LOG_LEVEL: "warn",
    PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS: "true",
    PROMPTSHEON_RATE_LIMIT: "5000",
    PROMPTSHEON_RATE_BURST: "2000",
    PROMPTSHEON_DB_PATH: "promptsheon.db"
  };
  const binary = path.resolve(REPO, "promptsheond");
  console.log("[smoke] booting daemon:", binary, "exists=", existsSync(binary));
  const child = spawnChild(binary, [], env);
  await waitFor(`${BASE}/health`, 60);
  return child;
}

async function bootstrapVite() {
  const child = spawnChild("node", ["./node_modules/vite/bin/vite.js"], { ...process.env, FORCE_COLOR: "0" }, path.resolve(REPO, "web"));
  await waitFor(`${FRONTEND}`, 60);
  return child;
}

async function bootstrapKey() {
  const H = { "Content-Type": "application/json" };
  const r = await fetch(`${BASE}/api/v1/setup`, { method: "POST", headers: H, body: "{}" });
  const body = await r.json();
  return body.key;
}

async function seed(key) {
  const A = { Authorization: `Bearer ${key}`, "Content-Type": "application/json" };
  const ws = await (await fetch(`${BASE}/api/v1/workspaces`, { method: "POST", headers: A, body: JSON.stringify({ name: "Smoke", organization: "Smoke Org" }) })).json();
  const proj = await (await fetch(`${BASE}/api/v1/workspaces/${ws.id}/projects`, { method: "POST", headers: A, body: JSON.stringify({ name: "Smoke project" }) })).json();
  for (let i = 0; i < 3; i++) {
    await fetch(`${BASE}/api/v1/projects/${proj.id}/capabilities`, { method: "POST", headers: A, body: JSON.stringify({ name: `Smoke cap ${i + 1}`, description: `Smoke ${i + 1}` }) });
  }
  return key;
}

async function teardown(key) {
  const A = { Authorization: `Bearer ${key}` };
  const ws = await (await fetch(`${BASE}/api/v1/workspaces`, { headers: A })).json();
  for (const w of ws || []) {
    const ps = await fetch(`${BASE}/api/v1/workspaces/${w.id}/projects`, { headers: A }).then((r) => r.ok ? r.json() : []).catch(() => []);
    for (const p of ps || []) {
      const cs = await fetch(`${BASE}/api/v1/projects/${p.id}/capabilities`, { headers: A }).then((r) => r.ok ? r.json() : []).catch(() => []);
      for (const c of cs || []) await fetch(`${BASE}/api/v1/capabilities/${c.id}`, { method: "DELETE", headers: A });
      await fetch(`${BASE}/api/v1/projects/${p.id}`, { method: "DELETE", headers: A });
    }
    await fetch(`${BASE}/api/v1/workspaces/${w.id}`, { method: "DELETE", headers: A });
  }
}

async function pageAssert(page, expected, label) {
  const viewHtml = await page.evaluate(() => {
    const v = document.querySelector("#view");
    return { text: document.body.textContent || "", view: v?.textContent || "", length: v?.textContent?.length || 0, hash: location.hash };
  });
  if (!viewHtml.text.includes(expected)) {
    throw new Error(`${label}: expected "${expected}"; hash=${viewHtml.hash}; view len=${viewHtml.length}; excerpt=${viewHtml.view.replace(/\s+/g, " ").slice(0, 300)}`);
  }
}

async function go(page, hash, settleMs = 5000) {
  await page.evaluate((h) => { location.hash = h; }, hash);
  await wait(settleMs);
}

async function smoke(key) {
  const browser = await puppeteer.launch({
    executablePath: HEADLESS,
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"],
    headless: true,
    defaultViewport: { width: 1440, height: 1100 }
  });
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => { if (m.type() === "error" && !/Failed to load resource/.test(m.text())) errors.push(`console: ${m.text()}`); });

  await page.goto(FRONTEND, { waitUntil: "networkidle0", timeout: 30000 });
  await page.evaluate((k) => localStorage.setItem("promptsheon.settings.v1", JSON.stringify({ apiBase: "", apiKey: k })), key);
  await page.reload({ waitUntil: "networkidle0" });
  await wait(5000);

  await pageAssert(page, "Promptsheon", "Overview");
  await go(page, "/capabilities");
  await pageAssert(page, "Workspace catalog", "Capabilities nav");
  await pageAssert(page, "Smoke cap", "Capabilities seed");

  await go(page, "/releases");
  await pageAssert(page, "Release pipeline", "Releases nav");

  await go(page, "/audit");
  await pageAssert(page, "Audit trail", "Audit nav");
  await page.evaluate(() => document.querySelector("#audit-verify")?.click());
  await wait(2500);
  await pageAssert(page, "Chain verified", "Verify chain result");

  await go(page, "/observability");
  await pageAssert(page, "Top capabilities", "Observability");

  await go(page, "/guardrails");
  await pageAssert(page, "Guardrails", "Guardrails");

  await go(page, "/evaluations");
  await pageAssert(page, "Evaluations", "Evaluations");

  for (const tab of ["alerts", "webhooks", "vault", "providers", "users", "reasoning"]) {
    await go(page, `/operations/${tab}`);
    await pageAssert(page, "Operator surface", `Operations/${tab}`);
  }

  await page.screenshot({ path: "web/screenshots/smoke-overview.png", fullPage: false });

  if (errors.length) {
    throw new Error(`Console errors during smoke: ${errors.join(" | ")}`);
  }

  await browser.close();
  console.log("[smoke] OK — all routes rendered with live data");
}

async function main() {
  if (!existsSync(HEADLESS)) {
    console.error(`[smoke] browser not found at ${HEADLESS}`);
    process.exit(2);
  }
  if (process.env.SMOKE_SKIP === "1") {
    console.log("[smoke] skipped");
    return;
  }
  await mkdir("web/screenshots", { recursive: true });
  let daemon = null;
  let vite = null;
  if (SMOKE_BOOT_DAEMON) {
    daemon = await bootstrapDaemon();
    process.on("exit", () => killPid(daemon?.pid));
  } else {
    await waitFor(`${BASE}/health`, 5);
  }
  if (SMOKE_BOOT_VITE) {
    vite = await bootstrapVite();
    process.on("exit", () => killPid(vite?.pid));
  } else {
    await waitFor(`${FRONTEND}`, 5);
  }
  const key = await bootstrapKey();
  try {
    await seed(key);
    await smoke(key);
  } finally {
    await teardown(key).catch(() => {});
    if (vite) killPid(vite.pid);
    if (daemon) killPid(daemon.pid);
  }
}

main().catch((err) => {
  console.error("[smoke] FAILED:", err.message);
  process.exit(1);
});
