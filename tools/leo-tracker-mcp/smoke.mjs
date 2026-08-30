#!/usr/bin/env node
// Дымовой прогон MCP-сервера трекера против живого ms_tracker.
//
//   node Fat-Leopard/tools/leo-tracker-mcp/smoke.mjs
//   node Fat-Leopard/tools/leo-tracker-mcp/smoke.mjs --from-railway
//
// Поднимает сервер как настоящий MCP-клиент (stdio, JSON-RPC), зовёт только
// читающие инструменты: health → очередь → упавшие карточки → ветка задачи.
// Ничего не пишет, ничего не мержит.

import { spawn, execFileSync } from "node:child_process";
import readline from "node:readline";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SERVER = path.join(HERE, "server.mjs");
const fromRailway = process.argv.includes("--from-railway");

const REPO = path.resolve(HERE, "..", "..");
const ENV_FILE = path.join(REPO, ".env.tracker-mcp");

// Railway ищет привязку проекта по текущему каталогу, а слинкованы у нас
// Fat-Leopard и Fat-Leopard/ms_tracker — из корня рабочей папки CLI говорит
// «No linked project found». Поэтому зовём его из нужного каталога.
function railwayVars(cwd, service) {
  const args = ["variables", "--json"];
  if (service) args.push("--service", service);
  try {
    const raw = execFileSync("railway", args, { cwd, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
    return { vars: JSON.parse(raw), where: `${path.basename(cwd)}${service ? " → " + service : ""}` };
  } catch (e) {
    const why = [e.stderr, e.stdout, e.message].map((x) => String(x || "").trim()).find(Boolean) || "неизвестно";
    return { error: why.split("\n").filter((l) => !/failed to refresh OAuth/.test(l)).join(" ").trim(), where: `${path.basename(cwd)}${service ? " → " + service : ""}` };
  }
}

function adopt(vars) {
  const took = [];
  const put = (key, val, label) => {
    if (!val || process.env[key]) return;
    process.env[key] = String(val);
    took.push(label || key);
  };
  put("TRACKER_SECRET", vars.TRACKER_SECRET || vars.BOARD_SSO_SECRET, "TRACKER_SECRET");
  put("NOTIFY_SECRET", vars.NOTIFY_SECRET || vars.TRACKER_SECRET || vars.BOARD_SSO_SECRET, "NOTIFY_SECRET");
  put("LEO_NOTIFY_URL", vars.LEO_NOTIFY_URL || vars.BOARD_NOTIFY_URL, "LEO_NOTIFY_URL");
  put("BOARD_REPO", vars.BOARD_REPO);
  put("BOARD_BRANCH", vars.BOARD_BRANCH);
  // У ms_tracker собственный адрес — в RAILWAY_PUBLIC_DOMAIN; у ms_leo он же
  // лежит готовым в TRACKER_URL, которым бот зовёт доску.
  put("TRACKER_URL", vars.TRACKER_URL, "TRACKER_URL");
  if (!process.env.TRACKER_URL && vars.RAILWAY_PUBLIC_DOMAIN && /tracker/i.test(String(vars.RAILWAY_SERVICE_NAME || ""))) {
    put("TRACKER_URL", "https://" + vars.RAILWAY_PUBLIC_DOMAIN, "TRACKER_URL (из RAILWAY_PUBLIC_DOMAIN)");
  }
  return took;
}

if (fromRailway) {
  const attempts = [
    [path.join(REPO, "ms_tracker"), null],
    [REPO, "ms_tracker"],
    [REPO, null],
    [REPO, "ms_leo"],
  ];
  const failures = [];
  for (const [cwd, service] of attempts) {
    const res = railwayVars(cwd, service);
    if (res.error) {
      failures.push(`  ${res.where}: ${res.error}`);
      continue;
    }
    const took = adopt(res.vars);
    console.log(`Railway ${res.where}: ${took.length ? "взято " + took.join(", ") : "нужных ключей нет"}`);
    if (process.env.TRACKER_URL && process.env.TRACKER_SECRET) break;
  }
  if (process.env.TRACKER_URL && process.env.TRACKER_SECRET) {
    // Кладём в .env.tracker-mcp, чтобы MCP-сервер в Claude Code работал сам,
    // без railway и без этого скрипта.
    const lines = ["# Заполнено smoke.mjs --from-railway"];
    for (const k of ["TRACKER_URL", "TRACKER_SECRET", "LEO_NOTIFY_URL", "NOTIFY_SECRET", "BOARD_REPO", "BOARD_BRANCH"]) {
      if (process.env[k]) lines.push(`${k}=${process.env[k]}`);
    }
    // Заготовка с пустыми значениями — не повод не писать: перетираем всё,
    // кроме файла с уже настоящим секретом.
    let filled = false;
    try {
      filled = /^TRACKER_SECRET=.+$/m.test(fs.readFileSync(ENV_FILE, "utf8"));
    } catch {}
    if (!filled || process.argv.includes("--write-env")) {
      fs.writeFileSync(ENV_FILE, lines.join("\n") + "\n", { mode: 0o600 });
      console.log(`Доступы записаны в ${ENV_FILE}\n`);
    } else {
      console.log(`${ENV_FILE} уже есть — не трогаю (перезаписать: --write-env)\n`);
    }
  } else {
    console.log("\nRailway не отдал переменные. Что пробовал:");
    console.log(failures.join("\n") || "  (сервисы ответили, но нужных ключей в них нет)");
    console.log("\nПроверь:  cd Fat-Leopard/ms_tracker && railway status && railway variables -k\n");
  }
}

const server = spawn("node", [SERVER], { stdio: ["pipe", "pipe", "inherit"], env: process.env });
const pending = new Map();
readline.createInterface({ input: server.stdout }).on("line", (line) => {
  let msg;
  try {
    msg = JSON.parse(line);
  } catch {
    return;
  }
  const done = pending.get(msg.id);
  if (done) {
    pending.delete(msg.id);
    done(msg);
  }
});
let seq = 0;
const rpc = (method, params) =>
  new Promise((resolve) => {
    const id = ++seq;
    pending.set(id, resolve);
    server.stdin.write(JSON.stringify({ jsonrpc: "2.0", id, method, params }) + "\n");
  });
const callTool = async (name, args = {}) => {
  const res = await rpc("tools/call", { name, arguments: args });
  const r = res.result ?? res.error;
  return { text: r?.content ? r.content.map((c) => c.text).join("\n") : JSON.stringify(r), isError: !!r?.isError };
};
const step = (title, { text, isError }) => {
  console.log(`\n${isError ? "✗" : "✓"} ${title}\n${text}`);
  return !isError;
};

const init = await rpc("initialize", {
  protocolVersion: "2025-06-18",
  capabilities: {},
  clientInfo: { name: "leo-tracker-smoke", version: "1.0.0" },
});
if (init.error) {
  console.log("Сервер не поднялся: " + JSON.stringify(init.error));
  process.exit(1);
}
console.log(`✓ handshake: протокол ${init.result.protocolVersion}, сервер ${init.result.serverInfo.name} ${init.result.serverInfo.version}`);
server.stdin.write(JSON.stringify({ jsonrpc: "2.0", method: "notifications/initialized" }) + "\n");

const tools = await rpc("tools/list", {});
console.log(`✓ инструментов: ${tools.result.tools.length} (${tools.result.tools.map((t) => t.name).join(", ")})`);

const healthy = step("tracker_health", await callTool("tracker_health"));
if (!healthy || /не отвечает|не задан/.test((await callTool("tracker_health")).text)) {
  // health сам печатает, чего не хватает — дальше идти смысла нет.
}

const jobs = await callTool("tracker_jobs", { limit: 10 });
step("tracker_jobs (последние 10)", jobs);

const failed = await callTool("tracker_jobs", { only_failed: true, limit: 10 });
step("tracker_jobs only_failed", failed);

// Ветку берём у первой упавшей задачи из вывода списка.
const branch = (failed.text.match(/tracker\/\d+-\d+/) || jobs.text.match(/tracker\/\d+-\d+/) || [])[0];
if (branch) {
  step(`tracker_inspect_branch ${branch}`, await callTool("tracker_inspect_branch", { branch }));
} else {
  console.log("\n· веток задач в выводе не нашлось — tracker_inspect_branch пропущен");
}

console.log("\nПисать в доску (tracker_report) и мержить (tracker_ship) прогон не пробует — это по решению человека.");
server.stdin.end();
