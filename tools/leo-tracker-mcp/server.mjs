#!/usr/bin/env node
// MCP-сервер доски Fat Leopard.
//
// Транспорт — stdio: JSON-RPC 2.0, по одному сообщению в строке (без
// Content-Length, как требует спека MCP). Зависимостей нет: только стандартная
// библиотека Node, чтобы сервер поднимался на любой машине без npm install.
//
// Даёт агенту (Claude Code) прямой доступ к ms_tracker и к доске ms_leo:
// посмотреть очередь, ветку задачи, поставить/снять задачу, поставить коммит
// фазы, отчитаться в карточку, влить ветку в main.

import readline from "node:readline";
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const SERVER_NAME = "leo-tracker";
const SERVER_VERSION = "1.0.0";
const SUPPORTED_PROTOCOLS = ["2025-06-18", "2025-03-26", "2024-11-05"];
const DEFAULT_PROTOCOL = "2025-06-18";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(HERE, "..", "..");

// ── конфиг ────────────────────────────────────────────────────────────────────

// Секреты трекера живут на Railway, а не в репозитории. Локально их кладут
// в .env.tracker-mcp рядом с .env — файл гитигнорится.
function loadEnvFiles() {
  const files = [];
  if (process.env.LEO_TRACKER_ENV) files.push(process.env.LEO_TRACKER_ENV);
  files.push(path.join(REPO_ROOT, ".env.tracker-mcp"));
  files.push(path.join(REPO_ROOT, ".env"));
  const loaded = [];
  for (const file of files) {
    let raw;
    try {
      raw = fs.readFileSync(file, "utf8");
    } catch {
      continue;
    }
    loaded.push(file);
    for (const line of raw.split("\n")) {
      const s = line.trim();
      if (!s || s.startsWith("#")) continue;
      const eq = s.indexOf("=");
      if (eq <= 0) continue;
      const key = s.slice(0, eq).trim();
      let val = s.slice(eq + 1).trim();
      if (
        (val.startsWith('"') && val.endsWith('"')) ||
        (val.startsWith("'") && val.endsWith("'"))
      ) {
        val = val.slice(1, -1);
      }
      // Реальное окружение сильнее файла: так .mcp.json может переопределить.
      if (process.env[key] === undefined && val !== "") process.env[key] = val;
    }
  }
  return loaded;
}

const ENV_FILES = loadEnvFiles();

function firstEnv(...keys) {
  for (const k of keys) {
    const v = (process.env[k] || "").trim();
    if (v) return v;
  }
  return "";
}

function config() {
  return {
    trackerURL: firstEnv("TRACKER_URL", "MYVIBELAB_URL", "BOARD_URL").replace(/\/+$/, ""),
    trackerSecret: firstEnv("TRACKER_SECRET", "BOARD_SSO_SECRET"),
    notifyURL: firstEnv("LEO_NOTIFY_URL", "BOARD_NOTIFY_URL"),
    notifySecret: firstEnv("NOTIFY_SECRET", "TRACKER_SECRET", "BOARD_SSO_SECRET"),
    repo: firstEnv("BOARD_REPO") || "fuserwyn/Fat-Leopard",
    branch: firstEnv("BOARD_BRANCH") || "main",
  };
}

function mask(v) {
  if (!v) return "— не задано";
  if (v.length <= 8) return "•".repeat(v.length);
  return v.slice(0, 4) + "…" + v.slice(-2) + ` (${v.length} симв.)`;
}

// ── HTTP к ms_tracker ─────────────────────────────────────────────────────────

// Railway за VPN (TUN / fake-ip) часто рвёт TLS на рукопожатии.
// Повторяем только сетевые обрывы, не HTTP-ошибки доски.
function isTransientNet(err) {
  const msg = String(err?.cause?.message || err?.message || err);
  return /fetch failed|network|ECONNRESET|ECONNREFUSED|ETIMEDOUT|socket|TLS|SSL|handshake|eof|disconnected|aborted|timeout|UND_ERR/i.test(
    msg,
  );
}

async function fetchWithRetry(url, { method, headers, body, timeoutMs = 60000, tries = 4 }) {
  let last;
  for (let i = 1; i <= tries; i++) {
    try {
      return await fetch(url, {
        method,
        headers,
        body,
        signal: AbortSignal.timeout(timeoutMs),
      });
    } catch (e) {
      last = e;
      if (i === tries || !isTransientNet(e)) throw e;
      await new Promise((r) => setTimeout(r, 300 * i));
    }
  }
  throw last;
}

async function trackerFetch(method, urlPath, { query, body, timeoutMs = 60000 } = {}) {
  const cfg = config();
  if (!cfg.trackerURL) throw new Error("TRACKER_URL не задан — некуда стучаться. Смотри tracker_health.");
  if (!cfg.trackerSecret) throw new Error("TRACKER_SECRET / BOARD_SSO_SECRET не задан. Смотри tracker_health.");
  let url = cfg.trackerURL + urlPath;
  if (query && Object.keys(query).length) {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined && v !== null && v !== "") qs.set(k, String(v));
    }
    const s = qs.toString();
    if (s) url += "?" + s;
  }
  const headers = {
    "X-Tracker-Secret": cfg.trackerSecret,
    Authorization: "Bearer " + cfg.trackerSecret,
    Accept: "application/json",
  };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  let resp;
  try {
    resp = await fetchWithRetry(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      timeoutMs,
    });
  } catch (e) {
    throw new Error(`${method} ${url}: сеть не ответила — ${e.message}`);
  }
  const text = await resp.text();
  // Ровно та дыра, на которой висит агент трекера: 200 с пустым телом.
  // Здесь пустое тело — явная ошибка с текстом, а не «unexpected end of JSON input».
  let json = null;
  let parseErr = "";
  if (text.trim() !== "") {
    try {
      json = JSON.parse(text);
    } catch (e) {
      parseErr = e.message;
    }
  }
  if (resp.status >= 300) {
    const why = json?.error || json?.message || (text.trim() ? clip(text, 400) : "тело пустое");
    throw new Error(`${method} ${urlPath}: HTTP ${resp.status} — ${why}`);
  }
  if (json === null) {
    throw new Error(
      parseErr
        ? `${method} ${urlPath}: HTTP ${resp.status}, тело не JSON (${parseErr}): ${clip(text, 400)}`
        : `${method} ${urlPath}: HTTP ${resp.status} и пустое тело ответа`,
    );
  }
  if (json.ok === false) {
    throw new Error(`${method} ${urlPath}: доска ответила «${json.error || json.message || "ошибка"}»`);
  }
  return json;
}

function clip(s, n) {
  const t = String(s).trim();
  return t.length <= n ? t : t.slice(0, n) + "…";
}

// Подпись уведомления для ms_leo: HMAC-SHA256 base64url(payload) секретом доски —
// формат один в один с ms_tracker/internal/notify.makeToken.
function notifyToken(secret, kind, repo, uid) {
  const payload = Buffer.from(
    JSON.stringify({ k: kind, r: repo, u: uid, n: "MCP", e: Math.floor(Date.now() / 1000) + 300 }),
  ).toString("base64url");
  const mac = crypto.createHmac("sha256", secret).update(payload).digest("base64url");
  return payload + "." + mac;
}

// ── инструменты ───────────────────────────────────────────────────────────────

function jobLine(j) {
  const bits = [`#${j.num || j.source_task_id || j.id}`, `job=${j.id}`, j.status || "—"];
  if (j.branch) bits.push(j.branch);
  if (j.when) bits.push(j.when);
  let line = bits.join(" · ");
  if (j.prompt) line += `\n   ${clip(j.prompt, 160)}`;
  if (j.error) line += `\n   ⚠️ ${clip(j.error, 200)}`;
  const steps = Array.isArray(j.steps) ? j.steps : [];
  if (steps.length) line += `\n   шаги: ${clip(steps.join(" → "), 220)}`;
  return line;
}

const TOOLS = [
  {
    name: "tracker_health",
    title: "Состояние трекера",
    description:
      "Проверить, с чем сервер вообще может работать: какие переменные подхватились, откуда, и жив ли ms_tracker (GET /health). Первое, что стоит звать, если доска «не работает».",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
    async run() {
      const cfg = config();
      const lines = [
        "Конфиг MCP-сервера трекера:",
        `  env-файлы: ${ENV_FILES.length ? ENV_FILES.join(", ") : "не найдены"}`,
        `  TRACKER_URL: ${cfg.trackerURL || "— не задано"}`,
        `  TRACKER_SECRET: ${mask(cfg.trackerSecret)}`,
        `  LEO_NOTIFY_URL: ${cfg.notifyURL || "— не задано"}`,
        `  NOTIFY_SECRET: ${mask(cfg.notifySecret)}`,
        `  BOARD_REPO: ${cfg.repo}`,
        `  BOARD_BRANCH: ${cfg.branch}`,
        "",
      ];
      if (!cfg.trackerURL || !cfg.trackerSecret) {
        lines.push(
          "Не хватает TRACKER_URL и/или TRACKER_SECRET.",
          "Возьми их из Railway (сервис ms_tracker) и положи в Fat-Leopard/.env.tracker-mcp.",
        );
        return lines.join("\n");
      }
      try {
        const res = await trackerFetch("GET", "/health", { timeoutMs: 20000 });
        lines.push(`ms_tracker жив: ${JSON.stringify(res)}`);
      } catch (e) {
        lines.push(`ms_tracker не отвечает: ${e.message}`);
      }
      return lines.join("\n");
    },
  },
  {
    name: "tracker_jobs",
    title: "Очередь задач",
    description:
      "Список задач исполнителя ms_tracker (последние 50). Фильтры: status (pending/running/done/error/canceled) и only_failed — только упавшие карточки.",
    inputSchema: {
      type: "object",
      properties: {
        status: { type: "string", description: "Фильтр по статусу задачи." },
        only_failed: { type: "boolean", description: "Только задачи с ошибкой." },
        limit: { type: "integer", minimum: 1, maximum: 50, description: "Сколько показать (по умолчанию 20)." },
      },
      additionalProperties: false,
    },
    async run(args) {
      const res = await trackerFetch("GET", "/api/scheduled");
      let tasks = Array.isArray(res.tasks) ? res.tasks : [];
      if (args.status) {
        const want = String(args.status).toLowerCase();
        tasks = tasks.filter((t) => String(t.status || "").toLowerCase() === want);
      }
      if (args.only_failed) {
        tasks = tasks.filter((t) => String(t.status || "").toLowerCase() === "error" || String(t.error || "").trim() !== "");
      }
      const limit = args.limit || 20;
      const shown = tasks.slice(0, limit);
      if (!shown.length) return "Задач под фильтр не нашлось.";
      return `Задач: ${tasks.length}, показываю ${shown.length}\n\n` + shown.map(jobLine).join("\n\n");
    },
  },
  {
    name: "tracker_job",
    title: "Карточка задачи",
    description: "Одна задача исполнителя целиком: промпт, статус, шаги, ошибка, результат, ветка.",
    inputSchema: {
      type: "object",
      properties: { id: { type: "integer", description: "id задачи в ms_tracker (поле job=… в списке)." } },
      required: ["id"],
      additionalProperties: false,
    },
    async run(args) {
      const res = await trackerFetch("GET", `/api/scheduled/${encodeURIComponent(args.id)}`);
      return JSON.stringify(res.task ?? res, null, 2);
    },
  },
  {
    name: "tracker_schedule",
    title: "Поставить задачу",
    description:
      "Поставить задачу в очередь ms_tracker. Осторожно: очередь подхватит её встроенный LLM-агент (тот самый, что пишет файлы целиком и умеет пушить). Для работы своими руками задачу ставить не нужно — читай карточку и правь код инструментами.",
    inputSchema: {
      type: "object",
      properties: {
        prompt: { type: "string", description: "Текст задачи агенту." },
        when: { type: "string", description: "Когда выполнить: «сейчас», «через 10 минут», «в 21:30». По умолчанию «сейчас»." },
        phase: { type: "string", enum: ["doing", "review", "test"], description: "Фаза. По умолчанию doing." },
        branch: { type: "string", description: "Ветка задачи, например tracker/24-31." },
        source_task_id: { type: "integer", description: "id карточки на доске ms_leo." },
        source_num: { type: "integer", description: "Номер карточки (#24)." },
        auto_push: { type: "boolean", description: "Пушить ветку. По умолчанию false." },
        model_key: { type: "string", description: "Ключ модели, если нужен не дефолтный." },
      },
      required: ["prompt"],
      additionalProperties: false,
    },
    async run(args) {
      const body = {
        prompt: args.prompt,
        when: args.when || "сейчас",
        phase: args.phase || "doing",
        auto_review: false,
        auto_push: args.auto_push === true,
      };
      if (args.branch) body.branch = args.branch;
      if (args.source_task_id) body.source_task_id = args.source_task_id;
      if (args.source_num) body.source_num = args.source_num;
      if (args.model_key) body.model_key = args.model_key;
      const res = await trackerFetch("POST", "/api/scheduled", { body });
      return `Задача поставлена: job=${res.id}, срок «${res.when || body.when}».`;
    },
  },
  {
    name: "tracker_cancel",
    title: "Снять задачу",
    description: "Снять задачу исполнителя с очереди по её id.",
    inputSchema: {
      type: "object",
      properties: { id: { type: "integer", description: "id задачи в ms_tracker." } },
      required: ["id"],
      additionalProperties: false,
    },
    async run(args) {
      await trackerFetch("POST", "/api/scheduled/cancel", { body: { id: args.id } });
      return `Задача job=${args.id} снята.`;
    },
  },
  {
    name: "tracker_inspect_branch",
    title: "Что на ветке",
    description:
      "Сравнить ветку задачи с базовой (обычно main) через GitHub: существует ли, какой HEAD, какие файлы, есть ли правки приложения (а не только .tracker/*.md).",
    inputSchema: {
      type: "object",
      properties: { branch: { type: "string", description: "Например tracker/24-31." } },
      required: ["branch"],
      additionalProperties: false,
    },
    async run(args) {
      const res = await trackerFetch("GET", "/api/inspect", { query: { branch: args.branch } });
      const files = Array.isArray(res.files) ? res.files : [];
      return [
        `Ветка ${res.branch || args.branch}`,
        `  существует: ${res.exists ? "да" : "нет"}`,
        `  HEAD: ${res.head || "—"}`,
        `  правки приложения: ${res.has_impl ? "есть" : "нет (только заметки трекера)"}`,
        `  файлов: ${files.length}`,
        ...files.slice(0, 40).map((f) => `    ${f}`),
        files.length > 40 ? `    … ещё ${files.length - 40}` : "",
      ]
        .filter(Boolean)
        .join("\n");
    },
  },
  {
    name: "tracker_stamp",
    title: "Коммит фазы",
    description:
      "Положить на ветку задачи коммит фазы (ревью/тест) с текстом вердикта в .tracker/job-N.md. Пуш только в ветку задачи, не в main.",
    inputSchema: {
      type: "object",
      properties: {
        note: { type: "string", description: "Текст вердикта." },
        phase: { type: "string", enum: ["review", "test", "doing"], description: "Фаза, по умолчанию review." },
        branch: { type: "string", description: "Ветка задачи." },
        source_task_id: { type: "integer", description: "id карточки на доске." },
        source_num: { type: "integer", description: "Номер карточки." },
      },
      required: ["note"],
      additionalProperties: false,
    },
    async run(args) {
      const body = {
        note: args.note,
        phase: args.phase || "review",
        branch: args.branch || "",
        source_task_id: args.source_task_id || 0,
        source_num: args.source_num || 0,
      };
      const res = await trackerFetch("POST", "/api/stamp", { body });
      return `Ветка ${res.branch}, коммит ${res.commit || "—"}, зафиксирован: ${res.committed ? "да" : "нет"}.`;
    },
  },
  {
    name: "tracker_report",
    title: "Отчёт в карточку",
    description:
      "Написать в карточку доски ms_leo (POST /api/miniapp/board/notify, подпись HMAC). Двигает карточку по колонкам: текст со словами «выполнен/готово/можно на тест/тест пройден» уводит её вперёд и может запустить сборку, «ошибка/не удалось» — помечает ошибкой. Так снимаются зависшие карточки вроде «агент не стартовал».",
    inputSchema: {
      type: "object",
      properties: {
        task_id: { type: "integer", description: "id карточки на доске ms_leo (или её номер)." },
        text: { type: "string", description: "Текст для карточки. Начинай с «Задача #N: …»." },
        author_id: { type: "integer", description: "Кому написать в личку; 0 — никому конкретно." },
      },
      required: ["task_id", "text"],
      additionalProperties: false,
    },
    async run(args) {
      const cfg = config();
      if (!cfg.notifyURL) throw new Error("LEO_NOTIFY_URL не задан — доске некуда писать. Смотри tracker_health.");
      if (!cfg.notifySecret) throw new Error("NOTIFY_SECRET / BOARD_SSO_SECRET не задан.");
      const author = args.author_id ?? 0;
      const body = {
        repo: cfg.repo,
        author_id: author,
        task_id: args.task_id,
        text: args.text,
        token: notifyToken(cfg.notifySecret, "notify", cfg.repo, author),
      };
      let resp;
      try {
        resp = await fetchWithRetry(cfg.notifyURL, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
          timeoutMs: 30000,
        });
      } catch (e) {
        throw new Error(`доска не ответила: ${e.message}`);
      }
      const text = await resp.text();
      if (resp.status >= 400) throw new Error(`доска ответила HTTP ${resp.status}: ${clip(text, 300) || "пусто"}`);
      return `Карточка #${args.task_id} обновлена (HTTP ${resp.status}). Ответ: ${clip(text, 200) || "пусто"}`;
    },
  },
  {
    name: "tracker_ship",
    title: "Влить ветку в main",
    description:
      "ОПАСНО: мержит ветку задачи в main, откуда Railway автодеплоит прод. Зовётся только по прямой просьбе человека и требует confirm=\"ship\". Перед этим обязательно tracker_inspect_branch.",
    inputSchema: {
      type: "object",
      properties: {
        branch: { type: "string", description: "Ветка задачи." },
        source_task_id: { type: "integer", description: "id карточки на доске." },
        source_num: { type: "integer", description: "Номер карточки." },
        confirm: { type: "string", description: "Ровно \"ship\" — подтверждение выката в прод." },
      },
      required: ["branch", "confirm"],
      additionalProperties: false,
    },
    async run(args) {
      if (args.confirm !== "ship") {
        throw new Error('Выкат не подтверждён: нужен confirm="ship". Это мерж в main и автодеплой прода.');
      }
      const body = { branch: args.branch, source_task_id: args.source_task_id || 0, source_num: args.source_num || 0 };
      const res = await trackerFetch("POST", "/api/ship", { body, timeoutMs: 120000 });
      return `Влито: ${res.head} → ${res.base}.`;
    },
  },
];

const TOOL_BY_NAME = new Map(TOOLS.map((t) => [t.name, t]));

// ── JSON-RPC ──────────────────────────────────────────────────────────────────

function send(msg) {
  process.stdout.write(JSON.stringify(msg) + "\n");
}

function reply(id, result) {
  send({ jsonrpc: "2.0", id, result });
}

function replyError(id, code, message) {
  send({ jsonrpc: "2.0", id, error: { code, message } });
}

async function handle(msg) {
  const { id, method, params } = msg;
  const isRequest = id !== undefined && id !== null;
  switch (method) {
    case "initialize": {
      const want = params?.protocolVersion;
      const protocolVersion = SUPPORTED_PROTOCOLS.includes(want) ? want : DEFAULT_PROTOCOL;
      reply(id, {
        protocolVersion,
        capabilities: { tools: { listChanged: false } },
        serverInfo: { name: SERVER_NAME, title: "Трекер Fat Leopard", version: SERVER_VERSION },
        instructions:
          "Доска задач Fat Leopard. Читай очередь и ветки этими инструментами, а код правь обычными файловыми инструментами в репозитории. tracker_ship мержит в main — только по прямой просьбе человека.",
      });
      return;
    }
    case "notifications/initialized":
    case "notifications/cancelled":
      return;
    case "ping":
      if (isRequest) reply(id, {});
      return;
    case "tools/list":
      reply(id, {
        tools: TOOLS.map((t) => ({
          name: t.name,
          title: t.title,
          description: t.description,
          inputSchema: t.inputSchema,
        })),
      });
      return;
    case "resources/list":
      reply(id, { resources: [] });
      return;
    case "resources/templates/list":
      reply(id, { resourceTemplates: [] });
      return;
    case "prompts/list":
      reply(id, { prompts: [] });
      return;
    case "tools/call": {
      const name = params?.name;
      const tool = TOOL_BY_NAME.get(name);
      if (!tool) {
        replyError(id, -32602, `Неизвестный инструмент: ${name}`);
        return;
      }
      const args = params?.arguments ?? {};
      for (const req of tool.inputSchema.required ?? []) {
        if (args[req] === undefined || args[req] === null || args[req] === "") {
          reply(id, { content: [{ type: "text", text: `Не хватает обязательного поля «${req}».` }], isError: true });
          return;
        }
      }
      try {
        const text = await tool.run(args);
        reply(id, { content: [{ type: "text", text: String(text) }] });
      } catch (e) {
        reply(id, { content: [{ type: "text", text: `Ошибка: ${e.message}` }], isError: true });
      }
      return;
    }
    default:
      if (isRequest) replyError(id, -32601, `Метод не поддерживается: ${method}`);
  }
}

const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
rl.on("line", (line) => {
  const s = line.trim();
  if (!s) return;
  let msg;
  try {
    msg = JSON.parse(s);
  } catch {
    send({ jsonrpc: "2.0", id: null, error: { code: -32700, message: "Тело не JSON" } });
    return;
  }
  // Пачка запросов приходит массивом — обрабатываем каждый отдельно.
  const items = Array.isArray(msg) ? msg : [msg];
  for (const item of items) {
    handle(item).catch((e) => {
      if (item?.id !== undefined && item?.id !== null) replyError(item.id, -32603, e.message);
    });
  }
});
rl.on("close", () => process.exit(0));
process.on("uncaughtException", (e) => {
  process.stderr.write(`leo-tracker-mcp: ${e?.stack || e}\n`);
});
