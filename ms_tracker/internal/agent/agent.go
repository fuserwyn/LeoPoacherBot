package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"leo-tracker/internal/config"
	"leo-tracker/internal/store"
)

func donateStarsFromPrompt(prompt string) []string {
	stars := []string{}
	lines := strings.Split(prompt, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Донат 1000") {
			stars = append(stars, "⭐️")
		}
	}
	return stars
}
