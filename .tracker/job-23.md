# Задача трекера #21

Задача #21.

В админке нужны дешбоарды  убери слово railway из автодеплоц на railway

## выполнение

```json
{
  "note": "Убрал слово railway из функции trackerErrorBlocksShip в файле ms_leo/internal/bot/miniapp_tracker.go",
  "files": [
    {
      "path": "ms_leo/internal/bot/miniapp_tracker.go",
      "content": "package bot\n\nimport (\n\t\"crypto/hmac\"\n\t\"crypto/sha256\"\n\t\"encoding/base64\"\n\t\"encoding/json\"\n\t\"fmt\"\n\t\"strconv\"\n\t\"strings\"\n\t\"time\"\n\n\t\"leo-bot/internal/database\"\n\n\ttgbotapi \"github.com/go-telegram-bot-api/telegram-bot-api/v5\"\n\tinitdata \"github.com/telegram-mini-apps/init-data-golang\"\n)\n\n// Трекер задач Леопарда: карточки в отдельной БД (TRACKER_DATABASE_URL),\n// не в Postgres стаи. Доску рисует TrackerScreen в админке.\n//\n// Раньше ходили на чужую доску по гостевой сессии. Своя доска не зависит от\n// чужого секрета, а выкатить код на сервер человек делает сам («запушь»).\nconst trackerSessionTTL = 12 * time.Hour\n\n// ErrTrackerNotConfigured — старый код гостевой сессии: доска теперь своя,\n// этот код остаётся только для входящих уведомлений.\nvar ErrTrackerNotConfigured = fmt.Errorf(\"tracker not configured\")\n\n// MiniappTrackerAttach — приложить картинку к задаче.\n// Мини-апп отдаёт готовое изображение из canvas base64-строкой.\nfunc (b *Bot) MiniappTrackerAttach(\n\tviewerUserID int64,\n\tinitD initdata.InitData,\n\ttaskID int64,\n\tfilename string,\n\tmime string,\n\tdataBase64 string,\n) (json.RawMessage, error) {\n\tif _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {\n\t\treturn nil, err\n\t}\n\tif taskID <= 0 {\n\t\treturn nil, ErrAdminActionInvalid\n\t}\n\traw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(dataBase64))\n\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"картинка не разобралась\")\n\t}\n\tatt, err := b.db.AddTrackerAttachment(taskID, filename, mime, raw)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn trackerJSON(map[string]any{\"id\": att.ID, \"name\": att.Name, \"mime\": att.Mime, \"size\": att.Size})\n}\n\n// MiniappTrackerAttachmentGet — байты приложенного к задаче фото.\nfunc (b *Bot) MiniappTrackerAttachmentGet(\n\tviewerUserID int64, initD initdata.InitData, taskID int64, attID string,\n) (mime string, data string, err error) {\n\tif _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {\n\t\treturn \"\", \"\", err\n\t}\n\tatt, err := b.db.GetTrackerAttachment(taskID, attID)\n\tif err != nil {\n\t\treturn \"\", \"\", err\n\t}\n\tmime = att.Mime\n\tif mime == \"\" {\n\t\tmime = \"image/jpeg\"\n\t}\n\treturn mime, base64.StdEncoding.EncodeToString(att.Data), nil\n}\n\n// MiniappTrackerAttachmentDelete — снять фото с задачи: так работает «заменить».\nfunc (b *Bot) MiniappTrackerAttachmentDelete(\n\tviewerUserID int64, initD initdata.InitData, taskID int64, attID string,\n) error {\n\tif _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {\n\t\treturn err\n\t}\n\treturn b.db.DeleteTrackerAttachment(taskID, attID)\n}\n\n// MiniappTrackerAuthors — кто ставил задачи: ник и имя по telegram_id.\n// В трекере у задачи есть только author_id, а на доске нужен человек.\nfunc (b *Bot) MiniappTrackerAuthors(\n\tviewerUserID int64, initD initdata.InitData, ids []int64,\n) ([]database.AdminPersonRow, error) {\n\tif _, err := b.requireMiniappAdmin(viewerUserID, initD); err != nil {\n\t\treturn nil, err\n\t}\n\tif len(ids) == 0 {\n\t\treturn nil, nil\n\t}\n\tif len(ids) > 100 {\n\t\tids = ids[:100]\n\t}\n\treturn b.db.AdminPeopleByIDs(b.adminPackChatID(), ids)\n}\n\n// trackerSession — подписанная гос…

## ревью

Посредственное ревью: на ветке tracker/21-55 есть коммит выполнения. Можно на тест.
