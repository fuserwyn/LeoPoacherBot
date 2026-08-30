package bot

import (
	"encoding/json"
	"testing"
	"time"

	"leo-bot/internal/database"
)

// Планировщик возвращается к карточке в «Сборке» каждые 15 секунд. Один и тот
// же шаг не должен множиться: иначе лог задачи вытесняется служебной строкой.
func TestAppendTrackerStepOnce(t *testing.T) {
	card := &database.TrackerTask{}
	for i := 0; i < 5; i++ {
		appendTrackerStepOnce(card, "автодеплой не настроен")
	}
	if len(card.Steps) != 1 {
		t.Fatalf("шаг повторился: %v", card.Steps)
	}
	appendTrackerStepOnce(card, "Railway: собираем MiniApp")
	appendTrackerStepOnce(card, "автодеплой не настроен")
	if len(card.Steps) != 3 {
		t.Fatalf("не тот же шаг подряд — должен писаться: %v", card.Steps)
	}
}

// Заказ сборки переживает рестарт бота: пересборка ms_leo убивает горутину
// ожидания, и после старта мы должны узнать свой деплой, а не заказать новый.
func TestTrackerDeployOrderRoundTrip(t *testing.T) {
	raw, err := json.Marshal(trackerDeployOrder{
		Ordered: true,
		Pinned:  map[string]string{"MiniApp": "dep-1", "ms_leo": "dep-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var back trackerDeployOrder
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Ordered || back.Pinned["ms_leo"] != "dep-2" {
		t.Fatalf("заказ не восстановился: %+v", back)
	}
	if key := trackerDeployOrderKey(7); key != "deploy_order_7" {
		t.Fatalf("ключ заказа: %q", key)
	}
}

func TestStandWaitPinned(t *testing.T) {
	deploys := []standDeploy{
		{ID: "old", Status: "SUCCESS", Service: "MiniApp"},
		{ID: "mine", Status: "BUILDING", Service: "MiniApp"},
	}
	if out := standWaitPinned(deploys, "mine"); out.Done || out.Err != nil {
		t.Fatalf("сборка ещё идёт: %+v", out)
	}

	deploys[1].Status = "SUCCESS"
	if out := standWaitPinned(deploys, "mine"); !out.Done || out.Err != nil {
		t.Fatalf("наш деплой собрался: %+v", out)
	}

	deploys[1].Status = "FAILED"
	out := standWaitPinned(deploys, "mine")
	if out.Err == nil || out.Done || out.FailedID != "mine" || out.FailedSvc != "MiniApp" {
		t.Fatalf("наш деплой упал: %+v", out)
	}

	// Заказ ещё не появился в списке Railway — ждём, а не закрываем по чужому.
	if out := standWaitPinned(deploys, "not-yet"); out.Done || out.Err != nil {
		t.Fatalf("деплоя пока не видно: %+v", out)
	}
}

// Заказанный деплой отменяет поблажку «стенд и так живой»: пока он не собрался,
// карточка не «выполнено», сколько бы старых SUCCESS ни лежало рядом.
func TestStandWaitServicesPinnedIgnoresOldSuccess(t *testing.T) {
	started := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	since := started.Add(-45 * time.Second)
	old := started.Add(-3 * time.Minute)
	by := map[string][]standDeploy{
		"MiniApp": {
			{ID: "old", Status: "SUCCESS", CreatedAt: old},
			{ID: "mine", Status: "DEPLOYING", CreatedAt: started.Add(5 * time.Second)},
		},
	}
	pinned := map[string]string{"MiniApp": "mine"}
	now := started.Add(trackerStandSkipGrace + time.Minute)
	if out := standWaitServices(by, pinned, since, started, now); out.Done || out.Err != nil {
		t.Fatalf("ждём свой деплой: %+v", out)
	}

	by["MiniApp"][1].Status = "SUCCESS"
	if out := standWaitServices(by, pinned, since, started, now); !out.Done || out.Err != nil {
		t.Fatalf("свой деплой собрался: %+v", out)
	}
}

// Сорванную сборку чинят повторным выкатом, поэтому она не должна запирать
// карточку: «Задеплоить» обязано доходить до Railway, а не отвечать skipped.
func TestTrackerErrorBlocksShipAllowsBuildRetry(t *testing.T) {
	retryable := []string{
		"сборка на стенде: MiniApp: деплой failed",
		"сборка на стенде: не уложилась в 10 минут",
		"Railway не принял заказ сборки",
	}
	for _, e := range retryable {
		if trackerErrorBlocksShip(e) {
			t.Fatalf("повтор выката должен быть возможен: %q", e)
		}
	}
	if !trackerErrorBlocksShip("Агент не стартовал: openrouter 504") {
		t.Fatal("срыв агента всё ещё блокирует выкат")
	}
	// Конфликт с main руками разводит человек, повтором мёрджа он не лечится.
	if !trackerErrorBlocksShip("не влили в main: конфликт с main") {
		t.Fatal("конфликт мёрджа блокирует автоматический выкат")
	}
	if trackerErrorBlocksShip("") {
		t.Fatal("пустая ошибка ничего не блокирует")
	}
}

// Сначала заказываем чужие сборки, свою — последней: пересборка ms_leo гасит
// этот процесс, и заказ MiniApp после неё уже никто бы не отправил.
func TestTrackerDeployQueuePutsSelfLast(t *testing.T) {
	queue := trackerDeployQueue([]standService{
		{ID: "1", Name: "ms_leo"},
		{ID: "2", Name: "MiniApp"},
	})
	if len(queue) != 2 || queue[0].Name != "MiniApp" || queue[1].Name != "ms_leo" {
		t.Fatalf("бот должен быть последним: %+v", queue)
	}
	if !trackerDeploySelfService("ms_leo") || !trackerDeploySelfService("ms-leo-main") ||
		trackerDeploySelfService("MiniApp") {
		t.Fatal("не тот сервис считаем своим")
	}
}

func TestRailwayDeployID(t *testing.T) {
	if got := railwayDeployID([]byte(`{"serviceInstanceDeploy":"dep-1"}`)); got != "dep-1" {
		t.Fatalf("string: %s", got)
	}
	if got := railwayDeployID([]byte(`{"serviceInstanceDeployV2":{"id":"dep-2"}}`)); got != "dep-2" {
		t.Fatalf("obj: %s", got)
	}
}
