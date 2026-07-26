import * as api from "../api.js";
import { loadSettings } from "../settings.js";
import { escape } from "../utils.js";

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone}"><span class="status-dot"></span>${escape(text)}</span>`;
}

function tone(level) {
  return ({ error: "danger", warn: "warn", info: "neutral", debug: "neutral" })[level] || "neutral";
}

function row(entry) {
  return `<div class="flex items-start gap-3 border-b border-line/40 px-3 py-2 font-mono text-[.7rem]">
    <span class="shrink-0 text-[.66rem] text-muted w-[88px]">${escape(entry.time || "")}</span>
    <span class="shrink-0 w-[60px] text-right">${pill(entry.level || "info", tone(entry.level))}</span>
    <span class="shrink-0 w-[80px] truncate text-[.66rem] text-muted">${escape(entry.source || "—")}</span>
    <span class="flex-1 break-words text-ink">${escape(entry.message || "")}</span>
  </div>`;
}

function levelChips(active) {
  return ["all", "info", "warn", "error", "debug"].map((level) => {
    const on = (active || "all") === level;
    return `<button type="button" data-log-level="${escape(level)}" class="rounded-md ${on ? "bg-ink text-paper" : "bg-paper text-muted hover:text-ink"} px-2.5 py-1 text-[.66rem] font-${on ? "bold" : "semibold"}">${escape(level)}</button>`;
  }).join("");
}

const buffer = [];
const MAX_BUFFER = 500;

export async function renderLogs(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  const levelFilter = route?.query?.level || "all";

  root.innerHTML = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;

  const shell = `
    <section class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <div class="eyebrow">Telemetry</div>
        <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">Live logs</h1>
        <p class="mt-1 text-[.78rem] text-muted">Streamed from <span class="mono">/api/v1/logs/stream</span>. EventSource can't carry the API key, so the dashboard uses fetch + ReadableStream to preserve auth and reconnect automatically.</p>
      </div>
      <div class="flex items-center gap-2">
        <div class="flex items-center gap-1 rounded-lg bg-paper p-1" data-log-level-chips>${levelChips(levelFilter)}</div>
        <button id="logs-pause" class="quiet-button">Pause</button>
        <button id="logs-clear" class="quiet-button">Clear</button>
      </div>
    </section>
    <section class="mt-5">
      <div class="panel p-0 overflow-hidden">
        <div class="flex items-center justify-between border-b border-line bg-paper px-3 py-2 text-[.62rem] uppercase tracking-wider text-muted">
          <span>Status <span id="logs-status" class="ml-1 normal-case font-bold text-ink">connecting…</span></span>
          <span>Buffered <span id="logs-count" class="ml-1 normal-case font-bold text-ink">0</span> lines</span>
        </div>
        <div id="logs-stream" class="max-h-[520px] overflow-y-auto"></div>
      </div>
    </section>
  `;
  root.innerHTML = shell;
  buffer.length = 0;
  const stream = root.querySelector("#logs-stream");
  const status = root.querySelector("#logs-status");
  const counter = root.querySelector("#logs-count");
  let paused = false;
  let controller = null;

  function record() {
    counter.textContent = String(buffer.length);
  }

  function appendLines(lines) {
    if (paused) return;
    const allowed = levelFilter === "all" || lines.every((l) => l.level === levelFilter);
    for (const line of lines) {
      if (levelFilter !== "all" && line.level !== levelFilter) continue;
      buffer.push(line);
    }
    if (buffer.length > MAX_BUFFER) buffer.splice(0, buffer.length - MAX_BUFFER);
    const visible = levelFilter === "all" ? buffer : buffer.filter((l) => l.level === levelFilter);
    stream.innerHTML = visible.map(row).join("");
    record();
  }

  async function openStream() {
    controller?.abort();
    status.textContent = "connecting…";
    status.parentElement.querySelector(".status-pill, [data-state]")?.remove?.();
    controller = new AbortController();
    const settings = loadSettings();
    const url = settings.apiBase
      ? `${settings.apiBase.replace(/\/$/, "")}/api/v1/logs/stream`
      : "/api/v1/logs/stream";
    const headers = { Accept: "text/event-stream" };
    if (settings.apiKey) headers.Authorization = `Bearer ${settings.apiKey}`;
    try {
      const response = await fetch(url, {
        headers,
        signal: controller.signal,
        credentials: "omit"
      });
      if (!response.ok || !response.body) {
        status.textContent = `error ${response.status}`;
        return;
      }
      status.textContent = "live";
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let bufferText = "";
      let pending = [];
      const flush = () => {
        if (pending.length) {
          appendLines(pending);
          pending = [];
        }
      };
      const timer = setInterval(flush, 100);
      try {
        while (true) {
          const { value, done } = await reader.read();
          if (done) break;
          bufferText += decoder.decode(value, { stream: true });
          let idx;
          while ((idx = bufferText.indexOf("\n\n")) !== -1) {
            const block = bufferText.slice(0, idx);
            bufferText = bufferText.slice(idx + 2);
            const entry = parseEventBlock(block);
            if (entry) pending.push(entry);
          }
        }
      } catch (e) {
        if (controller.signal.aborted) return;
        status.textContent = `read error`;
      } finally {
        clearInterval(timer);
        flush();
      }
    } catch (e) {
      status.textContent = "network error";
    } finally {
      if (!controller.signal.aborted) {
        setTimeout(() => openStream(), 2000);
      }
    }
  }

  function parseEventBlock(block) {
    let level = "info";
    let source = "system";
    let message = "";
    let time = new Date().toISOString().slice(11, 19);
    let dataLines = [];
    for (const line of block.split("\n")) {
      if (line.startsWith("event:")) continue;
      if (line.startsWith("data:")) {
        dataLines.push(line.slice(5).trim());
      } else if (line.startsWith(":")) {
        continue;
      } else if (line.startsWith("id:")) {
        time = line.slice(3).trim();
      }
    }
    if (!dataLines.length) return null;
    const payload = dataLines.join(" ");
    try {
      const obj = JSON.parse(payload);
      if (typeof obj === "object" && obj) {
        level = obj.level || obj.severity || level;
        message = obj.message || obj.msg || payload;
        source = obj.source || obj.component || source;
        time = obj.time || obj.ts || time;
      } else {
        message = String(obj);
      }
    } catch {
      message = payload;
    }
    return { level, message, source, time };
  }

  root.querySelectorAll("[data-log-level]").forEach((b) => {
    b.addEventListener("click", () => {
      const next = b.dataset.logLevel;
      window.location.hash = `#/logs?level=${encodeURIComponent(next)}`;
      window.location.reload();
    });
  });
  root.querySelector("#logs-pause")?.addEventListener("click", (event) => {
    paused = !paused;
    event.target.textContent = paused ? "Resume" : "Pause";
  });
  root.querySelector("#logs-clear")?.addEventListener("click", () => {
    buffer.length = 0;
    stream.innerHTML = "";
    record();
  });

  openStream();
  record();
  return shell;
}
