import puppeteer from "puppeteer-core";
const browser = await puppeteer.launch({
  executablePath: "/Users/sachin/Library/Caches/ms-playwright/chromium_headless_shell-1217/chrome-headless-shell-mac-arm64/chrome-headless-shell",
  args: ["--no-sandbox"],
  headless: true
});
const page = await browser.newPage();
const consoleErrs = [];
const apiErrs = [];
page.on("console", (m) => { if (m.type() === "error") consoleErrs.push(m.text()); });
page.on("response", (r) => {
  if (r.status() === 401 && r.url().includes("/api/v1/")) apiErrs.push(r.url().split("/api/v1/")[1].split("?")[0]);
});

// Scenario: user opens the dashboard WITHOUT an API key, then clicks the bell.
await page.goto("http://localhost:4173/", { waitUntil: "networkidle0" });
await new Promise((r) => setTimeout(r, 4000));
console.log("=== after first load (no key) ===");
console.log("401s:", apiErrs.length);
apiErrs.forEach((u) => console.log("  ", u));
apiErrs.length = 0;

// Click the bell
await page.click("button[data-open-notifications]");
await new Promise((r) => setTimeout(r, 3000));
console.log("\n=== after bell click ===");
console.log("401s:", apiErrs.length);
apiErrs.forEach((u) => console.log("  ", u));
apiErrs.length = 0;

// Navigate to /audit
await page.evaluate(() => document.getElementById("modal-root")?.replaceChildren());
await page.evaluate(() => { location.hash = "/audit"; });
await new Promise((r) => setTimeout(r, 4000));
console.log("\n=== after /audit navigation ===");
console.log("401s:", apiErrs.length);
apiErrs.forEach((u) => console.log("  ", u));
apiErrs.length = 0;

// Navigate to /logs
await page.evaluate(() => { location.hash = "/logs"; });
await new Promise((r) => setTimeout(r, 4000));
console.log("\n=== after /logs navigation ===");
console.log("401s:", apiErrs.length);
apiErrs.forEach((u) => console.log("  ", u));
apiErrs.length = 0;

console.log("\nconsole errors:", consoleErrs.slice(0, 5));
await browser.close();
