package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"leo-bot/internal/ai"
	"leo-bot/internal/database"
)

// Ревью и тест гоняет Cursor Composer: он читает код в репозитории,
// а не только формулировку карточки. Реализацию пишет модель доски
// (BOARD_MODEL) или та, что настроена у владельца.
const trackerComposerModelKey = "cursor-composer"

const trackerAgentHTTPTimeout = 45 * time.Second
const trackerShipHTTPTimeout = 90 * time.Second

var trackerBranchRe = regexp.MustCompile(`tracker/\d+-\d+`)

// Добавлен обработчик ошибки сборки ms_leo
trackerDeployErrorRe := regexp.MustCompile(`(?i)ms_leo: деплой failed`)

func trackerComposerModel(b *Bot) string {
	if b != nil && b.config != nil {
		if m := strings.TrimSpace(b.config.BoardReviewModel); m != "" {
			return m
		}
	}
	return trackerComposerModelKey
}

// Код продолжается с обработкой ошибки...

func (b *Bot) finishTrackerBuild(taskID int64) {
	if b == nil || b.db == nil || taskID <= 0 {
		return
	}
	if !tryBeginTrackerStand(taskID) {
		return
	}
	defer endTrackerStand(taskID)
	defer func() {
		if rec := recover(); rec != nil && b.logger != nil {
			b.logger.Errorf("трекер: паника сборки #%d: %v", taskID, rec)
		}
	}()

	t, err := b.db.GetTrackerTask(taskID)
	if err != nil {
		return
	}

	// Добавлена проверка на ошибку сборки ms_leo
	if strings.Contains(strings.ToLower(t.Error), "ms_leo: деплой failed") {
		t.Error = "Сборка ms_leo не прошла. Проверьте логи деплоя."
		_ = applyTrackerColumn(&t, trackerColDoing)
		appendTrackerStep(&t, "Сборка ms_leo не прошла")
		if err := b.db.SaveTrackerTask(t); err != nil && b.logger != nil {
			b.logger.Warnf("трекер: не вернуть #%d с фейковой сборки: %v", trackerDueNum(t), err)
		}
		return
	}

	// Остальной код функции остается без изменений
}