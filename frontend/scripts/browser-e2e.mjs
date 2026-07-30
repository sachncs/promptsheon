// Browser-level e2e for the dashboard. Boots the daemon, opens
// it in headless Chrome, and walks through every newly wired
// flow. Captures a screenshot at each step + asserts that the
// expected DOM hook (heading, table cell, modal title) actually
// rendered. Run with: node frontend/scripts/browser-e2e.mjs
//
// Pre-conditions:
//   - the daemon binary at bin/promptsheond is built
//   - the dashboard bundle is at cmd/promptsheond/frontend/dist
//   - the chrome binary is at the path below (or override via
//     SMOKE_BROWSER=...)
import { spawn } from "node:child_process";
import { setTimeout as wait } from "node:timers/promises";
import { mkdir, rm, readFile, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import path from "node:path";
import process from "node:process";
import puppeteer from "puppeteer-core";

const REPO = path.resolve(path.dirname(new URL(import.meta.url).pathname), "..", "..");
process.chdir(REPO);

const CHROME = process.env.SMOKE_BROWSER || "/Users/sachin/Library/Caches/ms-playwright/chromium-1217/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing";
const PORT = 18099;
const BASE = `http://localhost:${PORT}`;
const SHOTS = "/tmp/promptsheon-shots";

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

async function spawnDaemon() {
  const dbPath = path.resolve(REPO, "/tmp/promptsheon-be.db");
  for (const f of [dbPath, dbPath + "-shm", dbPath + "-wal"]) { try { await rm(f, { force: true }); } catch {} }
  const env = {
    ...process.env,
    PROMPTSHEON_AUTH: "true",
    PROMPTSHEON_ADDR: `127.0.0.1:${PORT}`,
    PROMPTSHEON_INSECURE_LOOPBACK: "true",
    PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS: "true",
    PROMPTSHEON_DB_PATH: dbPath,
    PROMPTSHEON_RATE_LIMIT: "0",
    PROMPTSHEON_BOOTSTRAP_TOKEN: "e2e-bootstrap-secret"
  };
  const child = spawn("./bin/promptsheond", [], { env, stdio: ["ignore", "pipe", "pipe"], shell: false });
  child.stdout.on("data", () => {});
  child.stderr.on("data", () => {});
  return child;
}

async function seedData(key) {
  const H = { Authorization: `Bearer ${key}`, "Content-Type": "application/json" };
  const ws = await (await fetch(`${BASE}/api/v1/workspaces`, { method: "POST", headers: H, body: JSON.stringify({ name: "e2e-ws" }) })).json();
  const wsId = ws.id;
  const proj = await (await fetch(`${BASE}/api/v1/workspaces/${wsId}/projects`, { method: "POST", headers: H, body: JSON.stringify({ name: "e2e-proj", description: "test" }) })).json();
  const projId = proj.id;
  const cap = await (await fetch(`${BASE}/api/v1/projects/${projId}/capabilities`, { method: "POST", headers: H, body: JSON.stringify({ name: "e2e-cap", description: "test" }) })).json();
  const capId = cap.id;
  // Seed a dataset and precondition so the harness-modals tests
  // have something to interact with.
  const ds = await (await fetch(`${BASE}/api/v1/capabilities/${capId}/datasets`, { method: "POST", headers: H, body: JSON.stringify({ name: "seed-dataset" }) })).json();
  await fetch(`${BASE}/api/v1/datasets/${ds.id}/cases`, { method: "PUT", headers: H, body: JSON.stringify({ cases: [{ inputs: { q: "hi" }, expected: "hello" }] }) });
  await fetch(`${BASE}/api/v1/capabilities/${capId}/preconditions`, { method: "POST", headers: H, body: JSON.stringify({ name: "seed-pre", command: "true", timeout_seconds: 5 }) });
  // Seed a user (so user-edit modal has something to edit)
  await fetch(`${BASE}/api/v1/users`, { method: "POST", headers: H, body: JSON.stringify({ email: "alice@e2e.local", name: "Alice", role: "writer" }) });
  // Seed an alert rule + group (so M2M test has targets)
  await fetch(`${BASE}/api/v1/alerts/notifications`, { method: "POST", headers: H, body: JSON.stringify({ name: "e2e-group", webhook_url: "https://example.com" }) });
  await fetch(`${BASE}/api/v1/alerts/rules`, { method: "POST", headers: H, body: JSON.stringify({ name: "e2e-rule", type: "error_rate", severity: "medium", threshold: 0.9, duration_minutes: 5, window_minutes: 1 }) });
  // Seed a version so the latest-version badge has something
  // to surface. Otherwise the badge shows nothing because
  // getLatestVersion returns 404.
  await fetch(`${BASE}/api/v1/capabilities/${capId}/versions`, {
    method: "POST", headers: H,
    body: JSON.stringify({
      version: 1,
      manifest: {
        prompt: { kind: "prompt", hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
        model_policy: { kind: "model_policy", hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
        runtime_policy: { kind: "runtime_policy", hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
        context_contract: { kind: "context_contract", hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
        memory: { kind: "memory", hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" }
      }
    })
  });
  return { wsId, projId, capId, dsId: ds.id };
}

let pass = 0;
let fail = 0;
const results = [];

async function step(name, fn) {
  try {
    await fn();
    pass++;
    results.push({ name, status: "PASS" });
    console.log(`  PASS  ${name}`);
  } catch (e) {
    fail++;
    results.push({ name, status: "FAIL", error: String(e?.message || e) });
    console.log(`  FAIL  ${name}: ${e?.message || e}`);
  }
}

async function main() {
  await mkdir(SHOTS, { recursive: true });
  await rm("/tmp/promptsheon-be.db", { force: true });
  await rm("/tmp/promptsheon-be.db-shm", { force: true });
  await rm("/tmp/promptsheon-be.db-wal", { force: true });

  console.log("Booting daemon...");
  const daemon = await spawnDaemon();
  await waitFor(`${BASE}/health`, 60);

  // Bootstrap admin key
  const boot = await (await fetch(`${BASE}/api/v1/setup`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Bootstrap-Token": "e2e-bootstrap-secret" },
    body: JSON.stringify({ email: "admin@e2e.local", name: "admin" })
  })).json();
  const key = boot.key;
  console.log(`  admin key: ${key.slice(0, 20)}...`);

  const ids = await seedData(key);
  console.log(`  seeded: workspace=${ids.wsId.slice(-8)} project=${ids.projId.slice(-8)} cap=${ids.capId.slice(-8)} dataset=${ids.dsId.slice(-8)}`);

  // Make the e2e seed the api key in localStorage before
  // navigation, so the dashboard doesn't show the first-run modal.
  const launcher = `
    <!doctype html>
    <html><body><script>
      localStorage.setItem("promptsheon:settings", JSON.stringify({ apiBase: "${BASE}", apiKey: "${key}" }));
      window.location.replace("${BASE}/");
    </script></body></html>
  `;
  // (We set localStorage after the browser launches instead.)

  console.log("Launching browser...");
  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: "new",
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"]
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1400, height: 900 });

  // Seed localStorage on every new document load, before any
  // SPA script runs. This guarantees the dashboard has a key
  // to authenticate with from the very first navigation.
  await page.evaluateOnNewDocument(([base, k]) => {
    try { localStorage.setItem("promptsheon.settings.v1", JSON.stringify({ apiBase: base, apiKey: k })); } catch {}
  }, [BASE, key]);

  await page.goto(`${BASE}/`, { waitUntil: "networkidle0" });
  const settings = await page.evaluate(() => {
    try {
      const raw = localStorage.getItem("promptsheon.settings.v1");
      return raw ? JSON.parse(raw) : null;
    } catch { return null; }
  });
  console.log(`  SPA settings: ${JSON.stringify(settings)}`);
  await page.screenshot({ path: `${SHOTS}/01-overview.png`, fullPage: true });

  // Step: overview loads
  await step("overview loads", async () => {
    const title = await page.$eval("h1, h2", (el) => el.textContent);
    if (!title) throw new Error("no h1/h2 rendered on overview");
  });

  // Step: capabilities list shows project section
  await step("capabilities list shows seeded project", async () => {
    await page.goto(`${BASE}/#/capabilities`, { waitUntil: "networkidle0" });
    await wait(1500); // give the SPA's renderCapabilitiesList time to fetch
    await page.screenshot({ path: `${SHOTS}/02-capabilities.png`, fullPage: true });
    const html = await page.content();
    if (!html.includes("e2e-proj")) throw new Error("seeded project not visible");
  });

  // Step: project detail page renders + delete button present
  await step("project detail page", async () => {
    await page.goto(`${BASE}/#/projects/${ids.projId}`, { waitUntil: "networkidle0" });
    await page.screenshot({ path: `${SHOTS}/03-project-detail.png`, fullPage: true });
    const heading = await page.$eval("h1", (el) => el.textContent);
    if (!heading?.includes("e2e-proj")) throw new Error(`h1 not project name, got: ${heading}`);
    if (!(await page.$("[data-delete-proj]"))) throw new Error("delete project button missing");
    if (!(await page.$("[data-edit-proj]"))) throw new Error("edit project button missing");
  });

  // Step: workspace detail page renders
  await step("workspace detail page", async () => {
    await page.goto(`${BASE}/#/workspaces/${ids.wsId}`, { waitUntil: "networkidle0" });
    await page.screenshot({ path: `${SHOTS}/04-workspace-detail.png`, fullPage: true });
    const heading = await page.$eval("h1", (el) => el.textContent);
    if (!heading?.includes("e2e-ws")) throw new Error(`h1 not workspace name, got: ${heading}`);
    if (!(await page.$("[data-delete-ws]"))) throw new Error("delete workspace button missing");
  });

  // Step: capability detail page shows datasets + preconditions
  await step("capability detail page (datasets + preconditions)", async () => {
    await page.goto(`${BASE}/#/capabilities/${ids.capId}`, { waitUntil: "networkidle0" });
    await wait(2000); // capability-detail fetches 5 endpoints in parallel
    await page.screenshot({ path: `${SHOTS}/05-capability-detail.png`, fullPage: true });
    const html = await page.content();
    if (!html.includes("seed-dataset")) throw new Error("seed-dataset not visible in capability detail");
    if (!html.includes("seed-pre")) throw new Error("seed-pre not visible in capability detail");
    if (!(await page.$("[data-action=new-dataset]"))) throw new Error("new dataset button missing");
    if (!(await page.$("[data-action=new-precondition]"))) throw new Error("new precondition button missing");
  });

  // Step: settings tab
  await step("operations → settings tab", async () => {
    await page.goto(`${BASE}/#/operations/settings`, { waitUntil: "networkidle0" });
    await wait(1500);
    await page.screenshot({ path: `${SHOTS}/06-settings.png`, fullPage: true });
    const html = await page.content();
    if (!html.includes("llm.openai.api_key_ref")) throw new Error("settings list missing default keys");
  });

  // Step: API keys tab
  await step("operations → API keys tab", async () => {
    await page.goto(`${BASE}/#/operations/apikeys`, { waitUntil: "networkidle0" });
    await page.screenshot({ path: `${SHOTS}/07-apikeys.png`, fullPage: true });
    if (!(await page.$("#apikey-new"))) throw new Error("apikeys new button missing");
  });

  // Step: providers list links to detail
  await step("providers list links to detail", async () => {
    await page.goto(`${BASE}/#/operations/providers`, { waitUntil: "networkidle0" });
    await page.screenshot({ path: `${SHOTS}/08-providers.png`, fullPage: true });
    const links = await page.$$eval("a[href*='#/providers/']", (els) => els.length);
    if (links === 0) throw new Error("no provider detail links");
  });

  // Step: provider detail page
  await step("provider detail page", async () => {
    await page.goto(`${BASE}/#/providers/openai`, { waitUntil: "networkidle0" });
    await wait(1500);
    await page.screenshot({ path: `${SHOTS}/09-provider-detail.png`, fullPage: true });
    if (!(await page.$("#provider-test"))) throw new Error("test connection button missing");
  });

  // Step: evaluations page
  await step("evaluations page", async () => {
    await page.goto(`${BASE}/#/evaluations`, { waitUntil: "networkidle0" });
    await page.screenshot({ path: `${SHOTS}/10-evaluations.png`, fullPage: true });
  });

  // Step: users tab + edit button
  await step("users tab + edit button", async () => {
    await page.goto(`${BASE}/#/operations/users`, { waitUntil: "networkidle0" });
    await page.screenshot({ path: `${SHOTS}/11-users.png`, fullPage: true });
    if (!(await page.$("[data-user-edit]"))) throw new Error("user edit button missing");
  });

  // Step: alerts tab + edit button
  await step("alerts tab + edit button", async () => {
    await page.goto(`${BASE}/#/operations/alerts`, { waitUntil: "networkidle0" });
    await page.screenshot({ path: `${SHOTS}/12-alerts.png`, fullPage: true });
    if (!(await page.$("[data-rule-edit]"))) throw new Error("alert rule edit button missing");
  });

  // Step: not-found page on bogus route
  await step("not-found route renders fallback", async () => {
    await page.goto(`${BASE}/#/this-route-does-not-exist`, { waitUntil: "networkidle0" });
    await page.screenshot({ path: `${SHOTS}/13-not-found.png`, fullPage: true });
    const heading = await page.$eval("h1", (el) => el.textContent).catch(() => "");
    if (!heading || !heading.toLowerCase().includes("not found")) {
      // The not-found view uses an h1 with class containing
      // text — accept either an h1 or any text that says 404.
      const html = await page.content();
      if (!/not\s*found|404/i.test(html)) throw new Error(`not-found page did not render 404, heading was: ${heading}`);
    }
  });

  // Step: M2M alert rule ↔ group link button present
  await step("M2M alert rule link buttons present", async () => {
    await page.goto(`${BASE}/#/operations/alerts`, { waitUntil: "networkidle0" });
    await wait(1500);
    await page.screenshot({ path: `${SHOTS}/14-m2m-link-buttons.png`, fullPage: true });
    if (!(await page.$("[data-rule-link]"))) throw new Error("no data-rule-link buttons rendered");
  });

  // Step: catalog search renders
  await step("catalog search renders", async () => {
    await page.goto(`${BASE}/#/operations/catalog`, { waitUntil: "networkidle0" });
    await wait(1500);
    await page.screenshot({ path: `${SHOTS}/15-catalog.png`, fullPage: true });
    if (!(await page.$("#catalog-form"))) throw new Error("catalog form missing");
    if (!(await page.$("#catalog-ws"))) throw new Error("catalog workspace picker missing");
  });

  // Step: user detail page
  await step("user detail page", async () => {
    const H = { Authorization: `Bearer ${key}` };
    const users = await (await fetch(`${BASE}/api/v1/users?limit=5`, { headers: H })).json();
    const u = users[0];
    if (!u) throw new Error("no users to navigate to");
    await page.goto(`${BASE}/#/users/${u.id}`, { waitUntil: "networkidle0" });
    await wait(1000);
    await page.screenshot({ path: `${SHOTS}/16-user-detail.png`, fullPage: true });
    const h1 = await page.$eval("h1", (el) => el.textContent);
    if (!h1 || !h1.includes(u.name)) throw new Error(`user-detail h1 mismatch: got ${h1}`);
  });

  // Step: latest-version badge on capability detail
  await step("latest-version badge on capability detail", async () => {
    await page.goto(`${BASE}/#/capabilities/${ids.capId}`, { waitUntil: "networkidle0" });
    await wait(2000);
    await page.screenshot({ path: `${SHOTS}/17-latest-version.png`, fullPage: true });
    const html = await page.content();
    if (!/Latest v1/.test(html)) throw new Error("'Latest v1' badge not rendered");
  });

  await browser.close();
  await killPid(daemon.pid);
  await rm("/tmp/promptsheon-be.db", { force: true });
  await rm("/tmp/promptsheon-be.db-shm", { force: true });
  await rm("/tmp/promptsheon-be.db-wal", { force: true });

  console.log("");
  console.log(`Result: ${pass} pass, ${fail} fail`);
  if (fail > 0) {
    process.exit(1);
  }
}

main().catch((e) => {
  console.error("e2e runner crashed:", e);
  process.exit(2);
});