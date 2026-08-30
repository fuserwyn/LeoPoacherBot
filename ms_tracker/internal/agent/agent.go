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

func donateRubFromPrompt(prompt string) []int {
	prompt = strings.ToLower(prompt)
	var nums []int
	for _, s := range []string{"100", "200", "300", "500", "1000", "5000"} {
		if strings.Contains(prompt, s) {
			nums = append(nums, atoi(s))
		}
	}
	return nums
}

func atoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}