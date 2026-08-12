package database

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Каскад полного удаления — это ~30 захардкоженных SQL по всей схеме, и опечатка в имени
// колонки вылезла бы только в проде (так уже было: AdminDeleteAllPackGroupMessagesByUser
// годами ходил в miniapp_pack_group_chat.user_id, которой нет — колонка называется from_user_id).
// Тест гоняет каждый count/delete против настоящей схемы.
//
// Запуск: TEST_DATABASE_URL=postgres://user@localhost:5432/leo_purge_test?sslmode=disable go test ./internal/database/ -run Purge
func TestAdminUserPurgeSpecsMatchSchema(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — schema check skipped")
	}

	d, err := New(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer d.Close()
	if err := d.CreateTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const probeUserID = int64(-987654321) // заведомо несуществующий: ничего не удалим

	for _, spec := range adminUserPurgeSpecs() {
		countQ := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, spec.table, spec.where)
		var n int64
		if err := d.db.QueryRow(countQ, probeUserID).Scan(&n); err != nil {
			t.Errorf("count %s (%s): %v", spec.table, spec.label, err)
		}
		deleteQ := fmt.Sprintf(`EXPLAIN DELETE FROM %s WHERE %s`, spec.table, spec.where)
		rows, err := d.db.Query(deleteQ, probeUserID)
		if err != nil {
			t.Errorf("delete %s (%s): %v", spec.table, spec.label, err)
			continue
		}
		rows.Close()
	}
}

// Живой каскад: заводим двух юзеров с переплетённым контентом, стираем одного и проверяем,
// что от него не осталось ничего, а сосед цел. Именно тут ловятся ошибки в подзапросах
// «чужие комментарии под моим постом» и в порядке удаления (FK).
func TestAdminPurgeUserEverywhereCascade(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — schema check skipped")
	}

	d, err := New(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer d.Close()
	if err := d.CreateTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const (
		victim = int64(-777001) // его стираем
		friend = int64(-777002) // он должен выжить
		pack   = int64(-100777)
	)
	// Чистый старт, даже если предыдущий прогон упал на середине.
	for _, id := range []int64{victim, friend} {
		if _, err := d.AdminPurgeUserEverywhere(id); err != nil {
			t.Fatalf("pre-clean %d: %v", id, err)
		}
	}
	t.Cleanup(func() {
		for _, id := range []int64{victim, friend} {
			_, _ = d.AdminPurgeUserEverywhere(id)
		}
	})

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.db.Exec(q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	mustID := func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := d.db.QueryRow(q, args...).Scan(&id); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
		return id
	}

	// Профили стаи + личка (личка = chat_id равен user_id).
	for _, id := range []int64{victim, friend} {
		mustExec(`INSERT INTO training_state (user_id, chat_id, username, last_message) VALUES ($1,$2,'seed','')`, id, pack)
		mustExec(`INSERT INTO training_state (user_id, chat_id, username, last_message) VALUES ($1,$1,'seed','')`, id)
	}

	victimPost := mustID(`INSERT INTO user_messages (user_id, chat_id, message_text, message_type) VALUES ($1,$2,'victim post','training_done') RETURNING id`, victim, pack)
	friendPost := mustID(`INSERT INTO user_messages (user_id, chat_id, message_text, message_type) VALUES ($1,$2,'friend post','training_done') RETURNING id`, friend, pack)

	// Комментарий друга под постом жертвы — уезжает вместе с постом.
	friendOnVictim := mustID(`INSERT INTO miniapp_training_feed_thread (pack_chat_id, user_message_id, from_user_id, message_text) VALUES ($1,$2,$3,'nice') RETURNING id`, pack, victimPost, friend)
	// Комментарий жертвы под постом друга — уезжает как её собственный.
	victimOnFriend := mustID(`INSERT INTO miniapp_training_feed_thread (pack_chat_id, user_message_id, from_user_id, message_text) VALUES ($1,$2,$3,'thanks') RETURNING id`, pack, friendPost, victim)
	// Комментарий друга под постом друга — должен выжить.
	friendOnFriend := mustID(`INSERT INTO miniapp_training_feed_thread (pack_chat_id, user_message_id, from_user_id, message_text) VALUES ($1,$2,$3,'my own') RETURNING id`, pack, friendPost, friend)

	mustExec(`INSERT INTO miniapp_training_feed_thread_likes (pack_chat_id, thread_reply_id, user_id) VALUES ($1,$2,$3)`, pack, friendOnVictim, friend)
	mustExec(`INSERT INTO miniapp_training_feed_thread_likes (pack_chat_id, thread_reply_id, user_id) VALUES ($1,$2,$3)`, pack, friendOnFriend, victim)
	mustExec(`INSERT INTO miniapp_training_thread_unread (recipient_user_id, pack_chat_id, thread_reply_id) VALUES ($1,$2,$3)`, victim, pack, friendOnVictim)
	mustExec(`INSERT INTO miniapp_training_feed_reactions (pack_chat_id, user_message_id, user_id, emoji) VALUES ($1,$2,$3,'🔥')`, pack, friendPost, victim)
	mustExec(`INSERT INTO miniapp_feed_poll_votes (user_message_id, user_id, option_index) VALUES ($1,$2,0)`, friendPost, victim)

	victimChatMsg := mustID(`INSERT INTO miniapp_pack_group_chat (pack_chat_id, from_user_id, message_text) VALUES ($1,$2,'hi') RETURNING id`, pack, victim)
	mustExec(`INSERT INTO miniapp_pack_group_reactions (pack_chat_id, pack_message_id, user_id, emoji) VALUES ($1,$2,$3,'👍')`, pack, victimChatMsg, friend)
	mustExec(`INSERT INTO miniapp_pack_group_unread (recipient_user_id, pack_chat_id, pack_message_id) VALUES ($1,$2,$3)`, friend, pack, victimChatMsg)

	// Жалоба друга на комментарий жертвы + прикреплённое уведомление админу.
	reportID := mustID(`INSERT INTO miniapp_feed_reports (pack_chat_id, reporter_user_id, target_type, user_message_id, thread_reply_id, target_user_id, target_text)
		VALUES ($1,$2,'thread_reply',$3,$4,$5,'bad') RETURNING id`, pack, friend, friendPost, victimOnFriend, victim)
	mustExec(`INSERT INTO miniapp_feed_report_admin_notifies (report_id, admin_user_id, chat_id, message_id) VALUES ($1,$2,$3,1)`, reportID, friend, friend)

	mustExec(`INSERT INTO miniapp_personal_chat (user_id, pack_chat_id, role, message_text) VALUES ($1,$2,'user','hey')`, victim, pack)
	mustExec(`INSERT INTO miniapp_support_chat (user_id, pack_chat_id, role, message_text) VALUES ($1,$2,'user','help')`, victim, pack)
	mustExec(`INSERT INTO miniapp_user_profile (user_id, pack_chat_id, display_name) VALUES ($1,$2,'Victim')`, victim, pack)
	mustExec(`INSERT INTO miniapp_friend_subscriptions (subscriber_id, target_id, pack_chat_id) VALUES ($1,$2,$3)`, friend, victim, pack)
	mustExec(`INSERT INTO training_sessions (user_id, chat_id, session_date) VALUES ($1,$2,'2026-08-12')`, victim, pack)
	mustExec(`INSERT INTO paywall_access_requests (user_id, monetized_chat_id) VALUES ($1,$2)`, victim, pack)
	mustExec(`INSERT INTO bot_visits (user_id) VALUES ($1)`, victim)
	mustExec(`INSERT INTO events (event_name, user_id) VALUES ('seed',$1)`, victim)
	mustExec(`INSERT INTO dynamic_admins (user_id, username, added_by, added_at) VALUES ($1,'victim',$2,NOW())`, victim, friend)
	mustExec(`INSERT INTO outbox_events (event_type, aggregate_key, payload, status, next_attempt_at)
		VALUES ('paywall_access_restore_requested','seed', json_build_object('user_id',$1::bigint)::jsonb,'pending',NOW())`, victim)

	report, err := d.AdminPurgeUserEverywhere(victim)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if report.Total == 0 {
		t.Fatal("purge report must count the seeded rows")
	}

	// Ничего от жертвы не осталось.
	after, err := d.AdminCountUserFootprint(victim)
	if err != nil {
		t.Fatalf("recount: %v", err)
	}
	if after.Total != 0 {
		for _, row := range after.NonEmpty() {
			t.Errorf("leftover in %s (%s): %d rows", row.Table, row.Label, row.Rows)
		}
		t.Fatalf("purge left %d rows behind", after.Total)
	}

	// Контент друга не задет, кроме того, что жил под постом жертвы.
	var n int64
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM user_messages WHERE id = $1`, friendPost).Scan(&n); err != nil || n != 1 {
		t.Fatalf("friend post must survive (n=%d, err=%v)", n, err)
	}
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM miniapp_training_feed_thread WHERE id = $1`, friendOnFriend).Scan(&n); err != nil || n != 1 {
		t.Fatalf("friend comment on own post must survive (n=%d, err=%v)", n, err)
	}
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM miniapp_training_feed_thread WHERE id = $1`, friendOnVictim).Scan(&n); err != nil || n != 0 {
		t.Fatalf("comment under purged post must be gone (n=%d, err=%v)", n, err)
	}
	// Жалоба удалена целиком — вместе с уведомлением админу (FK ON DELETE CASCADE).
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM miniapp_feed_report_admin_notifies WHERE report_id = $1`, reportID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("report notify must be gone (n=%d, err=%v)", n, err)
	}
	// Профиль друга остался — покажет, что WHERE не задел соседей.
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM training_state WHERE user_id = $1`, friend).Scan(&n); err != nil || n != 2 {
		t.Fatalf("friend training_state rows must survive (n=%d, err=%v)", n, err)
	}
}

// Полный прогон каскада на пустом юзере: проверяет и порядок удаления (FK), и коммит транзакции.
func TestAdminPurgeUserEverywhereRunsClean(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — schema check skipped")
	}

	d, err := New(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer d.Close()
	if err := d.CreateTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	report, err := d.AdminPurgeUserEverywhere(-987654321)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if report.Total != 0 {
		t.Fatalf("phantom user must have no rows, got %d", report.Total)
	}
	if len(report.Rows) != len(adminUserPurgeSpecs()) {
		t.Fatalf("report must cover every spec: got %d of %d", len(report.Rows), len(adminUserPurgeSpecs()))
	}
}
