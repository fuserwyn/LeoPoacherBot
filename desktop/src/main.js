// Sign-in screen. Network and Keychain live in Rust (see src-tauri/src/auth.rs);
// this file handles UI state and auth_poll polling.

import { applyDom, pickLang, strings } from "./i18n.js";

const invoke = window.__TAURI__?.core?.invoke;
const lang = pickLang();
const S = applyDom(lang);

const POLL_INTERVAL_MS = 2000;
const LOGIN_TIMEOUT_MS = 5 * 60 * 1000;

const views = {
  checking: document.getElementById("viewChecking"),
  idle: document.getElementById("viewIdle"),
  waiting: document.getElementById("viewWaiting"),
  done: document.getElementById("viewDone"),
};

const el = {
  error: document.getElementById("error"),
  countdown: document.getElementById("countdown"),
  doneName: document.getElementById("doneName"),
  settings: document.getElementById("settings"),
  baseUrl: document.getElementById("baseUrl"),
  botName: document.getElementById("botName"),
  loginBtn: document.getElementById("loginBtn"),
  reopenBtn: document.getElementById("reopenBtn"),
  cancelBtn: document.getElementById("cancelBtn"),
  saveBtn: document.getElementById("saveSettingsBtn"),
  logoutBtn: document.getElementById("logoutBtn"),
};

let attempt = null; // { nonce, link, deadline, timer }

function show(name) {
  for (const [key, node] of Object.entries(views)) {
    node.classList.toggle("hidden", key !== name);
  }
}

function showError(message) {
  el.error.textContent = message || "";
  el.error.classList.toggle("hidden", !message);
}

function applyReady(ready) {
  el.loginBtn.disabled = !ready;
  if (!ready) {
    el.settings.open = true;
    showError(S.serverRequired);
  }
}

function tickCountdown() {
  if (!attempt) return;
  const left = Math.max(0, attempt.deadline - Date.now());
  const min = Math.floor(left / 60000);
  const sec = String(Math.floor((left % 60000) / 1000)).padStart(2, "0");
  el.countdown.textContent = `${min}:${sec}`;
}

function stopAttempt() {
  if (attempt?.timer) clearTimeout(attempt.timer);
  attempt = null;
  el.countdown.textContent = "";
}

async function pollOnce() {
  if (!attempt) return;
  const { nonce } = attempt;
  try {
    const res = await invoke("auth_poll", { nonce });
    if (!attempt || attempt.nonce !== nonce) return;

    if (res.status === "ok") {
      stopAttempt();
      const name = res.user?.first_name || res.user?.username || "";
      el.doneName.textContent = S.hello(name);
      show("done");
      await invoke("open_app");
      return;
    }
    if (res.status === "expired") {
      stopAttempt();
      show("idle");
      showError(S.expired);
      return;
    }
  } catch (e) {
    showError(String(e));
  }

  if (!attempt) return;
  if (Date.now() >= attempt.deadline) {
    stopAttempt();
    show("idle");
    showError(S.timeout);
    return;
  }
  tickCountdown();
  attempt.timer = setTimeout(pollOnce, POLL_INTERVAL_MS);
}

async function startLogin() {
  showError("");
  el.loginBtn.disabled = true;
  try {
    const { nonce, link } = await invoke("auth_begin");
    attempt = { nonce, link, deadline: Date.now() + LOGIN_TIMEOUT_MS, timer: null };
    show("waiting");
    tickCountdown();
    attempt.timer = setTimeout(pollOnce, POLL_INTERVAL_MS);
  } catch (e) {
    showError(String(e));
    el.settings.open = true;
  } finally {
    el.loginBtn.disabled = false;
  }
}

async function saveSettings() {
  showError("");
  el.saveBtn.disabled = true;
  try {
    const s = await invoke("settings_set", {
      url: el.baseUrl.value,
      bot: el.botName.value,
    });
    el.baseUrl.value = s.base_url;
    el.botName.value = s.bot_username;
    applyReady(s.ready);
  } catch (e) {
    showError(String(e));
  } finally {
    el.saveBtn.disabled = false;
  }
}

async function logout() {
  stopAttempt();
  showError("");
  try {
    await invoke("auth_logout");
  } catch (e) {
    showError(String(e));
  }
  show("idle");
}

async function boot() {
  if (!invoke) {
    show("idle");
    showError(S.tauriMissing);
    return;
  }
  try {
    const s = await invoke("settings_get");
    el.baseUrl.value = s.base_url;
    el.botName.value = s.bot_username;

    const user = await invoke("session_status");
    if (user) {
      el.doneName.textContent = S.hello(user.first_name);
      show("done");
      await invoke("open_app");
      return;
    }

    show("idle");
    applyReady(s.ready);
  } catch (e) {
    show("idle");
    showError(String(e));
  }
}

el.loginBtn.addEventListener("click", startLogin);
el.cancelBtn.addEventListener("click", () => {
  stopAttempt();
  show("idle");
});
el.reopenBtn.addEventListener("click", async () => {
  if (!attempt) return;
  try {
    await invoke("auth_open_link", { link: attempt.link });
  } catch (e) {
    showError(String(e));
  }
});
el.saveBtn.addEventListener("click", saveSettings);
el.logoutBtn.addEventListener("click", logout);

boot();
