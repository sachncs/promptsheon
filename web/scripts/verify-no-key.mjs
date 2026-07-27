import puppeteer from "puppeteer-core";
const browser = await puppeteer.launch({
  executablePath: "/Users/sachin/Library/Caches/ms-playwright/chromium_headless_shell-1217/chrome-headless-shell-mac-arm64/chrome-headless-shell",
  args: ["--no-sandbox"],
  headless: true
});
const page = await browser.newPage();
const errs = [];
page.on("response", async (r) => {
  if (r.status() === 401) errs.push(r.url());
});

await page.goto("http://localhost:4173/", { waitUntil: "networkidle0" });
await new Promise((r) => setTimeout(r, 4000));
const noKeyState = await page.evaluate(() => ({
  h1: document.querySelector("h1")?.textContent,
  hasConnectButton: !!document.querySelector("button[data-open-settings]"),
  errors: []
}));
console.log("no-key state:", JSON.stringify(noKeyState));
console.log("401s with no key:", errs.length, errs);
await browser.close();
