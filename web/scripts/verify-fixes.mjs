import puppeteer from "puppeteer-core";
const ADMIN_KEY = process.env.ADMIN_KEY;
const browser = await puppeteer.launch({
  executablePath: "/Users/sachin/Library/Caches/ms-playwright/chromium_headless_shell-1217/chrome-headless-shell-mac-arm64/chrome-headless-shell",
  args: ["--no-sandbox"],
  headless: true
});
const page = await browser.newPage();
const consoleErrors = [];
page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text()); });
page.on("pageerror", (e) => consoleErrors.push(`pageerror: ${e.message}`));

await page.goto("http://localhost:4173/", { waitUntil: "networkidle0" });
await page.evaluate((k) => localStorage.setItem("promptsheon.settings.v1", JSON.stringify({ apiBase: "", apiKey: k })), ADMIN_KEY);
await page.reload({ waitUntil: "networkidle0" });
await new Promise((r) => setTimeout(r, 5000));

console.log("\n=== BUG 1: notification icon ===");
const bell = await page.$("button[data-open-notifications]");
await bell.click();
await new Promise((r) => setTimeout(r, 2500));
const notifState = await page.evaluate(() => {
  const root = document.getElementById("modal-root");
  return {
    title: root.querySelector("h2")?.textContent,
    hasAlerts: !!root.querySelector("#notif-alerts"),
    hasAudit: !!root.querySelector("#notif-audit"),
    auditRows: root.querySelectorAll("#notif-audit a").length,
    alertsRows: root.querySelectorAll("#notif-alerts a").length,
    length: root.innerHTML.length
  };
});
console.log(JSON.stringify(notifState, null, 2));

console.log("\n=== BUG 2: New capability (from capability catalog) ===");
await page.evaluate(() => document.getElementById("modal-root")?.replaceChildren());
await page.evaluate(() => { location.hash = "/capabilities"; });
await new Promise((r) => setTimeout(r, 4000));
const catState = await page.evaluate(() => ({
  h1: document.querySelector("h1")?.textContent,
  hasNewButton: !!document.querySelector("button[data-new-capability]"),
  hasAddCapabilityButtons: document.querySelectorAll("[data-new-capability-for-project]").length,
  hasProjectSections: document.querySelectorAll("section.panel").length
}));
console.log(JSON.stringify(catState, null, 2));

// Click "New capability" in catalog
await page.click("button[data-new-capability]");
await new Promise((r) => setTimeout(r, 1500));
const newCapModal = await page.evaluate(() => {
  const root = document.getElementById("modal-root");
  return {
    title: root.querySelector("h2")?.textContent,
    hasForm: !!root.querySelector("#new-capability-form"),
    projectOptions: [...root.querySelectorAll("#nc-project option")].map((o) => o.textContent)
  };
});
console.log("modal:", JSON.stringify(newCapModal, null, 2));

// Submit a new cap
if (newCapModal.hasForm && newCapModal.projectOptions.length > 1) {
  const projectId = await page.evaluate(() => document.getElementById("nc-project").options[1].value);
  await page.type("#nc-name", "Catalog test cap");
  await page.type("#nc-description", "Created from catalog");
  await page.select("#nc-project", projectId);
  await page.click("#new-capability-form button[type=submit]");
  await new Promise((r) => setTimeout(r, 4000));
  const after = await page.evaluate(() => ({
    capRows: document.querySelectorAll("#cap-list [data-searchable]").length,
    modalOpen: !!document.querySelector("#modal-root .modal-card")
  }));
  console.log("after submit:", JSON.stringify(after, null, 2));
}

console.log("\n=== BUG 4: Live log ===");
await page.evaluate(() => { location.hash = "/logs"; });
await new Promise((r) => setTimeout(r, 6000));
const logState = await page.evaluate(() => ({
  status: document.getElementById("logs-status")?.textContent,
  hasStream: !!document.querySelector("#logs-stream")
}));
console.log("log state:", JSON.stringify(logState, null, 2));

console.log("\nconsole errors:", consoleErrors.slice(0, 10));
await browser.close();
