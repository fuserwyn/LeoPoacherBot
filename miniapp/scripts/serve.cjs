/* Статика на $PORT (Railway / production). */
const { spawn } = require("child_process");
const path = require("path");

const port = String(process.env.PORT || 3000);
const serveMain = path.join(__dirname, "..", "node_modules", "serve", "build", "main.js");
// -c serve.json: длинный immutable-кэш на хэшированные ассеты Vite, no-cache на index.html —
// иначе serve отдаёт JS/CSS без кэша и они едут заново на каждый запуск мини-аппа (эгресс).
const serveConfig = path.join(__dirname, "..", "serve.json");
const child = spawn(process.execPath, [serveMain, "-s", "-c", serveConfig, "dist", "-l", `tcp://0.0.0.0:${port}`], {
  stdio: "inherit",
  cwd: path.join(__dirname, ".."),
  env: process.env,
});
child.on("exit", (code) => process.exit(code ?? 1));
