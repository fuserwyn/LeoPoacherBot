package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"leo-bot/internal/database"
)

const trackerAutoDeployKey = "auto_deploy"

func (b *Bot) trackerAutoDeployEnabled() bool {
	if b == nil || b.db == nil {
		return false
	}
	value, err := b.db.GetTrackerSetting(trackerAutoDeployKey)
	if err != nil {
		if b.logger != nil {
			b.logger.Warnf("трекер: не прочитать настройку автодеплоя: %v", err)
		}
		return true
	}
	return !strings.EqualFold(strings.TrimSpace(value), "off")
}

func (b *Bot) setTrackerAutoDeploy(on bool, by int64) error {
	if b == nil || b.db == nil {
		return fmt.Errorf("база недоступна")
	}
	value := "off"
	if on {
		value = "on"
	}
	return b.db.SetTrackerSetting(trackerAutoDeployKey, value, by)
}

func (b *Bot) trackerRailwayReady() bool {
	return b != nil && b.config != nil &&
		strings.TrimSpace(b.config.RailwayToken) != "" &&
		strings.TrimSpace(b.config.RailwayProjectID) != ""
}
