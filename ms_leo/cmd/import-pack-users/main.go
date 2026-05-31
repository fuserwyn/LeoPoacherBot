// Импорт пользователей из старого приложения по списку Telegram user_id.
//
// Заполняет: paywall_access_requests (если не -skip-paywall), training_state, miniapp_user_profile.
//
// Использование:
//
//	cd ms_leo
//	# из .env: DATABASE_URL, MONETIZED_CHAT_ID, PAYWALL_ENABLED
//	go run ./cmd/import-pack-users -ids 111,222,333
//	go run ./cmd/import-pack-users -file scripts/pack-user-ids.txt
//	go run ./cmd/import-pack-users -file ids.txt -dry-run
//
// Формат файла: одна строка = user_id или user_id<TAB|пробел>username
// Пустые строки и строки с # в начале пропускаются.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"leo-bot/internal/config"
	"leo-bot/internal/database"
)

func main() {
	var (
		idsCSV      string
		idsFile     string
		packIDFlag  int64
		skipPaywall bool
		noTimer     bool
		dryRun      bool
	)
	flag.StringVar(&idsCSV, "ids", "", "Telegram user_id через запятую")
	flag.StringVar(&idsFile, "file", "", "Файл: user_id или user_id username")
	flag.Int64Var(&packIDFlag, "pack-id", 0, "MONETIZED_CHAT_ID (перекрывает env)")
	flag.BoolVar(&skipPaywall, "skip-paywall", false, "Не писать paywall_access_requests")
	flag.BoolVar(&noTimer, "no-timer", false, "Не ставить timer_start_time (таймер стартует при первом заходе в miniapp)")
	flag.BoolVar(&dryRun, "dry-run", false, "Только показать, что будет сделано")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	packID := packIDFlag
	if packID == 0 {
		packID = cfg.MonetizedChatID
	}
	if packID == 0 {
		log.Fatal("задай MONETIZED_CHAT_ID в .env или флаг -pack-id")
	}

	skipPW := skipPaywall
	if !skipPW && !cfg.PaywallEnabled {
		log.Println("PAYWALL_ENABLED=false → paywall_access_requests пропускаем (или укажи -skip-paywall явно)")
		skipPW = true
	}

	rows, err := loadUserRows(idsCSV, idsFile)
	if err != nil {
		log.Fatalf("ids: %v", err)
	}
	if len(rows) == 0 {
		log.Fatal("список user_id пуст: укажи -ids или -file")
	}

	opts := database.ImportPackUserOpts{
		SkipPaywall: skipPW,
		StartTimer:  !noTimer,
	}

	fmt.Printf("pack_chat_id=%d users=%d skip_paywall=%v start_timer=%v dry_run=%v\n",
		packID, len(rows), skipPW, opts.StartTimer, dryRun)

	if dryRun {
		for _, r := range rows {
			fmt.Printf("  would import user_id=%d username=%q\n", r.UserID, displayName(r))
		}
		return
	}

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	var ok, skipped int
	for _, r := range rows {
		res, err := db.ImportPackUser(packID, database.ImportPackUserInput{
			UserID:   r.UserID,
			Username: r.Username,
		}, opts)
		if err != nil {
			log.Printf("FAIL user_id=%d: %v", r.UserID, err)
			continue
		}
		if res.PaywallInserted || res.TrainingInserted || res.ProfileInserted {
			ok++
			log.Printf("OK user_id=%d paywall=%v training=%v profile=%v",
				r.UserID, res.PaywallInserted, res.TrainingInserted, res.ProfileInserted)
		} else {
			skipped++
			log.Printf("SKIP user_id=%d (уже есть активные записи)", r.UserID)
		}
	}

	fmt.Printf("\nГотово: inserted=%d skipped=%d total=%d\n", ok, skipped, len(rows))
	fmt.Println("Пользователям: /start в боте или открыть miniapp — синхронизируется menu button.")
}

type userRow struct {
	UserID   int64
	Username string
}

func displayName(r userRow) string {
	if s := strings.TrimSpace(r.Username); s != "" {
		return s
	}
	return fmt.Sprintf("user%d", r.UserID)
}

func loadUserRows(idsCSV, idsFile string) ([]userRow, error) {
	var out []userRow
	seen := map[int64]struct{}{}

	add := func(uid int64, username string) error {
		if uid == 0 {
			return fmt.Errorf("invalid user id 0")
		}
		if _, ok := seen[uid]; ok {
			return nil
		}
		seen[uid] = struct{}{}
		out = append(out, userRow{UserID: uid, Username: strings.TrimSpace(username)})
		return nil
	}

	if strings.TrimSpace(idsCSV) != "" {
		for _, part := range strings.Split(idsCSV, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			uid, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse id %q: %w", part, err)
			}
			if err := add(uid, ""); err != nil {
				return nil, err
			}
		}
	}

	if idsFile != "" {
		f, err := os.Open(idsFile)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		lineNo := 0
		for sc.Scan() {
			lineNo++
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			uid, err := strconv.ParseInt(fields[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("file line %d: %w", lineNo, err)
			}
			uname := ""
			if len(fields) > 1 {
				uname = strings.Join(fields[1:], " ")
			}
			if err := add(uid, uname); err != nil {
				return nil, fmt.Errorf("file line %d: %w", lineNo, err)
			}
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}

	return out, nil
}
