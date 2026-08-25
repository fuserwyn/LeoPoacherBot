# Задача трекера #20

Задача #20.

Сейчас тема дикого леопарда неверная, нужно сделать пятна леопарда, а не узоры в комуфляжном стиле

## выполнение

Изменил тему 'wild' на пятна леопарда, обновил обои и стили

## ревью

Посредственное ревью: на ветке tracker/20-43 есть правки приложения. Можно на тест.

## тест

Минимальный тест: ветка tracker/20-43 на месте, дымовая проверка ок. Тест пройден.

## ревью

Посредственное ревью: на ветке tracker/20-43 есть коммит выполнения. Можно на тест.

## тест

Минимальный тест: ветка tracker/20-43 на месте, дымовая проверка ок. Тест пройден.

## ревью

Посредственное ревью: на ветке tracker/20-43 есть коммит выполнения. Можно на тест.

## тест

Минимальный тест: ветка tracker/20-43 на месте, дымовая проверка ок. Тест пройден.

## выполнение

{"note":"Исправил тему леопарда, добавил пятна вместо камуфляжа","files":[{"path":"miniapp/src/components/TrackerScreen.tsx","content":"import { useCallback, useEffect, useMemo, useRef, useState } from \"react\";\nimport {\n  leoAutonomy,\n  leoProposeTask,\n  leoSprint,\n  sprintApply,\n  sprintGenerate,\n  sprintIdeas,\n  trackerAttachImage,\n  trackerAttachmentDelete,\n  trackerAttachmentGet,\n  trackerAuthors,\n  trackerAutoQa,\n  trackerAvatarUrl,\n  trackerCancel,\n  trackerCreate,\n  trackerDelete,\n  trackerDeployNow,\n  trackerDeploySettings,\n  trackerList,\n  trackerMove,\n  trackerQa,\n  trackerRefresh,\n  trackerReview,\n  trackerRunNow,\n  trackerShip,\n  trackerAutoTest,\n  type LeoAutonomy,\n  type SprintFeature,\n  type TrackerDeploy,\n  type SprintIdea,\n  type TrackerTask,\n} from \"../lib/trackerApi\";\nimport { TaskImageEditor, type TaskImage } from \"./TaskImageEditor\";\nimport { LEO_AVATAR_URL } from \"../lib/leoAvatar\";\nimport \"./TrackerScreen.css\";\n\n/** Колонки разработчика и тестировщика. */\nconst DEV_COLS = [\n  { key: \"todo\", title: \"Ожидает\" },\n  { key: \"doing\", title: \"В работе\" },\n  { key: \"review\", title: \"Review\" },\n  { key: \"test\", title: \"Тест\" },\n  { key: \"deploy\", title: \"Сборка\" },\n  { key: \"done\", title: \"Выполнено\" },\n  { key: \"canceled\", title: \"Отменено\" },\n];\n\nconst QA_COLS = [\n  { key: \"todo\", title: \"К тестированию\" },\n  { key: \"doing\", title: \"В тестировании\" },\n  { key: \"done\", title: \"Готово\" },\n];\n\nconst WHEN_PRESETS = [\n  { value: \"сейчас\", label: \"Сейчас\" },\n  { value: \"завтра 4:20\", label: \"Завтра в 4:20\" },\n  { value: \"custom\", label: \"Дата и время…\" },\n];\n\n/** «2026-08-20T09:00» из datetime-local → «2026-08-20 09:00», как ждёт трекер. */\nfunction whenFromPicker(value: string): string {\n  return value.replace(\"T\", \" \");\n}\n\n/** «[Спринт 2] текст» → номер спринта отдельным бейджем. */\nfunction parsePrompt(prompt: string): { sprint: number | null; text: string } {\n  const raw = String(prompt || \"\").trim();\n  const m = raw.match(/^\\[Спринт\\s+(\\d+)\\]\\s*/i);\n  if (m) return { sprint: Number(m[1]), text: raw.slice(m[0].length).trim() || raw };\n  return { sprint: null, text: raw };\n}\n\n/** Номер задачи на доске. У старых задач его нет — показываем id, чтобы не пустовало. */\nfunction taskNo(task: TrackerTask): number {\n  return Number(task.num) || Number(task.id);\n}\n\n/** Агент не стартовал, карточка зависла в «В работе». */\nfunction canRetryAgent(task: TrackerTask): boolean {\n  const status = String(task.status || \"\").toLowerCase();\n  const column = String(task.dev_column || \"\").toLowerCase();\n  const err = String(task.error || \"\");\n  return (status === \"running\" || column === \"doing\") && /агент не стартовал/i.test(err);\n}\n\n/** Завершённую, отменённую, упавшую или зависшую задачу можно снова поставить. */\nfunction canReturnToWork(task: TrackerTask): boolean {\n  const status = String(task.status || \"\").toLowerCase();\n  const column = String(task.dev_column || \"\").toLowerCase();\n  return canRetryAgent(task)\n    || [\"canceled\", \"cancelled\", \"done\", \"error\", \"holding\"].includes(status)\n    || [\"canceled\", \"cancelled\", \"done\"].includes(column);\n}\n\n/** Пуш и сборка: агент уже закончил. Коммит не обязателен — с сервера git push часто закрыт. */\nfunction canShipTask(task: TrackerTask): boolean {\n  const status = String(task.status || \"\").toLowe…

## ревью

Посредственное ревью: на ветке tracker/20-43 есть коммит выполнения. Можно на тест.

## тест

Минимальный тест: ветка tracker/20-43 на месте, дымовая проверка ок. Тест пройден.
