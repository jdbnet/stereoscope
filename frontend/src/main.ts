import { Speaker, Volume2, VolumeX, createElement } from "lucide";
import { Events } from "@wailsio/runtime";
import {
  BeginCapture,
  CaptureState,
  Devices,
  Platform,
  PlayTest,
  Settings,
  Select,
  Version,
} from "../bindings/stereoscope/internal/service/service";
import type { Device, PairLevels, Source } from "../bindings/stereoscope/internal/audio/models";

const THRESHOLD = 0.0031622776;
const HOLD_MS = 150;

type Side = { active: boolean; pending: boolean | null; since: number };

const app = document.querySelector<HTMLDivElement>("#app")!;
const holds = new Map<string, { left: Side; right: Side }>();
const peaks = new Map<string, { left: number; right: number }>();

declare global {
  interface Window {
    _wails?: { environment?: { OS?: string }; invoke?: unknown };
    wails?: { invoke?: unknown };
    chrome?: { webview?: unknown };
    webkit?: { messageHandlers?: { external?: unknown } };
  }
}

// environment.OS is filled in after the runtime is ready. The native bridge
// is already present when this script runs.
const preview = !(
  window.chrome?.webview ||
  window.wails?.invoke ||
  window.webkit?.messageHandlers?.external
);

let platform = "windows";
let capture = "unsupported";
let devices: Device[] = [];
let levels: PairLevels[] = [];
let sources: Source[] = [];
let deviceID = "";
let showAbout = false;
let version = "dev";

function side(): Side {
  return { active: false, pending: null, since: 0 };
}

function updateSide(key: string, which: "left" | "right", level: number): boolean {
  let pair = holds.get(key);
  if (!pair) {
    pair = { left: side(), right: side() };
    holds.set(key, pair);
  }
  const s = pair[which];
  const above = level >= THRESHOLD;
  const now = performance.now();
  if (above === s.active) {
    s.pending = null;
    return s.active;
  }
  if (s.pending === null || s.pending !== above) {
    s.pending = above;
    s.since = now;
    return s.active;
  }
  if (now - s.since >= HOLD_MS) {
    s.active = above;
    s.pending = null;
  }
  return s.active;
}

function label(left: boolean, right: boolean): string {
  if (left && right) return "Both";
  if (left) return "Left only";
  if (right) return "Right only";
  return "Silent";
}

function holdPeak(key: string, sideName: "left" | "right", level: number): number {
  let pair = peaks.get(key);
  if (!pair) {
    pair = { left: 0, right: 0 };
    peaks.set(key, pair);
  }
  const decay = 0.02;
  pair[sideName] = Math.max(level, pair[sideName] - decay);
  return pair[sideName];
}

function icon(node: typeof Speaker, width: number, height: number): string {
  const svg = createElement(node, { width, height });
  return `<span class="icon" aria-hidden="true">${svg.outerHTML}</span>`;
}

function meter(name: string, level: number, held: number): string {
  const pct = Math.min(100, Math.round(level * 100));
  const hold = Math.min(100, Math.round(held * 100));
  return `<div class="meter">
    <div class="meter-label"><span>${name}</span><span class="pct">${pct}%</span></div>
    <div class="track"><div class="fill" style="width:${pct}%"></div><div class="hold" style="left:${hold}%"></div></div>
  </div>`;
}

const shownLevel = new Map<string, { left: number; right: number }>();

function smoothLevel(id: string, sideName: "left" | "right", level: number): number {
  let pair = shownLevel.get(id);
  if (!pair) {
    pair = { left: level, right: level };
    shownLevel.set(id, pair);
  }
  const target = Math.min(1, Math.max(0, level));
  const next = pair[sideName] + (target - pair[sideName]) * 0.22;
  pair[sideName] = Math.abs(next - target) < 0.004 ? target : next;
  return pair[sideName];
}

function render() {
  const options = devices
    .map((d) => {
      const mark = d.active ? " (active)" : d.default ? " (default)" : "";
      const selected = d.id === deviceID ? " selected" : "";
      return `<option value="${escape(d.id)}"${selected}>${escape(d.name)}${mark}</option>`;
    })
    .join("");

  const cards = (levels.length ? levels : [{ pair: { id: "front", label: "Front" }, left: 0, right: 0 }])
    .map((lv) => {
      const leftOn = updateSide(lv.pair.id, "left", lv.left);
      const rightOn = updateSide(lv.pair.id, "right", lv.right);
      const text = label(leftOn, rightOn);
      const tests =
        platform === "android"
          ? ""
          : `<div class="tests">
              <button data-test="${lv.pair.id}:left">Left</button>
              <button data-test="${lv.pair.id}:right">Right</button>
              <button data-test="${lv.pair.id}:both">Both</button>
            </div>`;
      return `<article class="card" data-pair="${escape(lv.pair.id)}">
        <header><h2>${escape(lv.pair.label)}</h2><p class="status ${text === "Silent" ? "silent" : ""}">${text}</p></header>
        <div class="meters">
          ${meter("L", smoothLevel(lv.pair.id, "left", lv.left), holdPeak(lv.pair.id, "left", lv.left))}
          ${meter("R", smoothLevel(lv.pair.id, "right", lv.right), holdPeak(lv.pair.id, "right", lv.right))}
        </div>
        ${tests}
      </article>`;
    })
    .join("");

  const playing =
    sources.length === 0
      ? `<p class="empty">Nothing is playing to this device</p>`
      : sources
          .map((src) => playingRow(src))
          .join("");

  const android =
    platform === "android" && capture !== "capturing"
      ? `<section class="notice">
          <p>Android asks for the microphone permission before the screen-capture prompt. That permission is how Android allows an app to measure playback. StereoScope does not listen to the microphone.</p>
          <p>Apps that opt out of capture, including many streaming apps, will not move the meters. This app doesn't allow audio capture.</p>
          <p>Now playing shows the playback type. An app name appears only when a media notification is visible and notification access is enabled.</p>
          <button id="capture">${capture === "capturing" ? "Listening" : "Allow audio capture"}</button>
          <p class="hint">Capture status: ${escape(capture)}</p>
        </section>`
      : "";

  const about = showAbout
    ? `<section class="about">
        <h2>About StereoScope</h2>
        <p>Version ${escape(version)}</p>
        <p>StereoScope shows whether sound is coming from the left channel, the right channel, or both, and which apps are playing to the selected device.</p>
        <button id="close-about">Close</button>
      </section>`
    : "";

  const banner = preview
    ? `<p class="preview">Browser preview. Sample meters only; the Go backend is not connected. Use <code>wails3 dev</code> for a live desktop session.</p>`
    : "";

  app.innerHTML = `${banner}<header class="top">
      <div class="brand">${icon(Speaker, 22, 22)}<div><h1>StereoScope</h1><p>Live channel monitor</p></div></div>
      <label>Output device
        <select id="device">${options}</select>
      </label>
      <button id="about" class="ghost">About</button>
    </header>
    ${android}
    <section class="grid">${cards}</section>
    <section class="playing${platform === "android" ? " names" : ""}">
      <h2>${icon(sources.length ? Volume2 : VolumeX, 18, 18)} Now playing</h2>
      <ul>${playing}</ul>
    </section>
    ${about}`;

  document.querySelector<HTMLSelectElement>("#device")?.addEventListener("change", async (ev) => {
    const id = (ev.target as HTMLSelectElement).value;
    deviceID = id;
    await Select(id);
  });
  document.querySelectorAll<HTMLButtonElement>("[data-test]").forEach((button) => {
    button.addEventListener("click", () => {
      const [pair, sideName] = button.dataset.test!.split(":");
      if (preview) {
        previewTone(pair, sideName);
        return;
      }
      void PlayTest(pair, sideName);
    });
  });
  document.querySelector("#about")?.addEventListener("click", () => {
    showAbout = true;
    render();
  });
  document.querySelector("#close-about")?.addEventListener("click", () => {
    showAbout = false;
    render();
  });
  document.querySelector("#capture")?.addEventListener("click", async () => {
    capture = await BeginCapture();
    render();
  });
}

function playingRow(src: Source): string {
  const img = src.icon ? `<img alt="" src="${src.icon}" />` : icon(Volume2, 18, 18);
  const note = src.note ? `<small>${escape(src.note)}</small>` : "";
  const level = platform === "android" ? "" : `<div class="mini"><div style="width:${Math.min(100, Math.round(src.peak * 100))}%"></div></div>`;
  return `<li>${img}<div><strong>${escape(src.name)}</strong>${note}</div>${level}</li>`;
}

function escape(value: string): string {
  return value.replace(/[&<>"']/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch]!));
}

function previewDevices(): Device[] {
  return [
    { id: "speakers", name: "Speakers", default: true, channels: 8, active: true },
    { id: "headphones", name: "Headphones", default: false, channels: 2, active: false },
  ];
}

function previewTone(pair: string, sideName: string) {
  levels = levels.map((lv) => {
    if (lv.pair.id !== pair) return lv;
    return { ...lv, left: sideName === "right" ? 0 : 0.8, right: sideName === "left" ? 0 : 0.8 };
  });
  updateMeters();
}

function shownLevels(): PairLevels[] {
  return levels.length ? levels : [{ pair: { id: "front", label: "Front" }, left: 0, right: 0 }];
}

function updateMeters() {
  const rows = shownLevels();
  const cards = [...document.querySelectorAll<HTMLElement>("#app article.card")];
  if (cards.length !== rows.length || rows.some((lv, i) => cards[i].dataset.pair !== lv.pair.id)) {
    render();
    return;
  }
  for (const lv of rows) {
    const card = cards.find((item) => item.dataset.pair === lv.pair.id)!;
    const leftOn = updateSide(lv.pair.id, "left", lv.left);
    const rightOn = updateSide(lv.pair.id, "right", lv.right);
    const text = label(leftOn, rightOn);
    const status = card.querySelector(".status");
    if (status) {
      status.textContent = text;
      status.classList.toggle("silent", text === "Silent");
    }
    const fills = card.querySelectorAll<HTMLElement>(".fill");
    const holds = card.querySelectorAll<HTMLElement>(".hold");
    const sides = [
      [smoothLevel(lv.pair.id, "left", lv.left), holdPeak(lv.pair.id, "left", lv.left)],
      [smoothLevel(lv.pair.id, "right", lv.right), holdPeak(lv.pair.id, "right", lv.right)],
    ] as const;
    const labels = card.querySelectorAll<HTMLElement>(".pct");
    sides.forEach(([level, held], index) => {
      const pct = Math.min(100, Math.round(level * 100));
      if (fills[index]) fills[index].style.width = `${pct}%`;
      if (holds[index]) holds[index].style.left = `${Math.min(100, Math.round(held * 100))}%`;
      if (labels[index]) labels[index].textContent = `${pct}%`;
    });
  }
}

function updatePlaying() {
  const section = document.querySelector(".playing");
  if (!section) {
    render();
    return;
  }
  const heading = section.querySelector("h2");
  if (heading) heading.innerHTML = `${icon(sources.length ? Volume2 : VolumeX, 18, 18)} Now playing`;
  const playing =
    sources.length === 0
      ? `<p class="empty">Nothing is playing to this device</p>`
      : `<ul>${sources
          .map((src) => playingRow(src))
          .join("")}</ul>`;
  const list = section.querySelector("ul, .empty");
  if (list) list.outerHTML = playing;
  else section.insertAdjacentHTML("beforeend", playing);
}

function previewFrame() {
  const t = performance.now() / 1000;
  const pulse = (offset: number) => Math.max(0, Math.sin(t * 2 + offset)) * 0.7;
  const specs = [
    ["front", "Front", pulse(0), 0],
    ["centre-lfe", "Centre / LFE", pulse(1), pulse(1.4)],
    ["rear", "Rear", 0, pulse(2)],
    ["side", "Side", 0, 0],
  ] as const;
  if (deviceID === "headphones") {
    levels = [{ pair: { id: "front", label: "Front" }, left: pulse(0), right: 0 }];
  } else {
    levels = specs.map(([id, label, left, right]) => ({ pair: { id, label }, left, right }));
  }
  updateMeters();
}

async function boot() {
  render();
  if (preview) {
    platform = "windows";
    version = "dev";
    devices = previewDevices();
    deviceID = devices[0].id;
    sources = [
      { pid: 1, name: "Browser", icon: "", peak: 0.4, note: "" },
      { pid: 0, name: "System sounds", icon: "", peak: 0.15, note: "" },
    ];
    previewFrame();
    window.setInterval(previewFrame, 30);
    return;
  }
  try {
    platform = await Platform();
    version = await Version();
    capture = await CaptureState();
    const settings = await Settings();
    deviceID = settings.deviceId;
    devices = (await Devices()) ?? [];
    if (!deviceID) {
      deviceID = devices.find((d) => d.default || d.active)?.id ?? devices[0]?.id ?? "";
    }
  } catch (err) {
    console.error(err);
  }
  render();
}

Events.On("levels", (event) => {
  levels = (event.data as PairLevels[]) ?? [];
  updateMeters();
});
Events.On("sources", (event) => {
  sources = (event.data as Source[]) ?? [];
  updatePlaying();
});
Events.On("capture", (event) => {
  const next = String(event.data ?? "");
  if (!next || next === capture) return;
  capture = next;
  render();
});
Events.On("devices", (event) => {
  devices = (event.data as Device[]) ?? [];
  if (deviceID && !devices.some((d) => d.id === deviceID)) {
    deviceID = devices.find((d) => d.default || d.active)?.id ?? "";
    if (deviceID) void Select(deviceID);
  }
  render();
});

void boot();
