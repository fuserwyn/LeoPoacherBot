package trainingmap

// Карта тренировок профиля: фиксированный круг узлов (виды спорта).
// Прогресс круга = workouts_total % len(Path); следующий узел — первый незакрытый.

type NodeDef struct {
	ID    string
	Label string
	Emoji string
	X     float64
	Y     float64
}

// Path — порядок узлов на карте. Координаты в % viewBox (0–100), как на фронте.
var Path = []NodeDef{
	{ID: "run", Label: "Бег", Emoji: "🏃", X: 16, Y: 16},
	{ID: "strength", Label: "Силовая", Emoji: "🏋️", X: 50, Y: 10},
	{ID: "yoga", Label: "Йога", Emoji: "🧘", X: 84, Y: 18},
	{ID: "walk", Label: "Ходьба", Emoji: "🚶", X: 78, Y: 46},
	{ID: "hiit", Label: "HIIT", Emoji: "⚡", X: 42, Y: 42},
	{ID: "bike", Label: "Велосипед", Emoji: "🚴", X: 18, Y: 62},
	{ID: "stretch", Label: "Растяжка", Emoji: "🧎", X: 48, Y: 78},
	{ID: "swim", Label: "Плавание", Emoji: "🏊", X: 84, Y: 82},
}

type NodeView struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Emoji  string  `json:"emoji"`
	Index  int     `json:"index"`
	Status string  `json:"status"` // done | next | remaining
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

type Snapshot struct {
	WorkoutsTotal int        `json:"workouts_total"`
	Completed     int        `json:"completed"`
	Remaining     int        `json:"remaining"`
	NextIndex     int        `json:"next_index"`
	Lap           int        `json:"lap"`
	Nodes         []NodeView `json:"nodes"`
}

func SnapshotFor(workoutsTotal int) Snapshot {
	n := len(Path)
	if workoutsTotal < 0 {
		workoutsTotal = 0
	}
	if n == 0 {
		return Snapshot{WorkoutsTotal: workoutsTotal, Lap: 1, Nodes: []NodeView{}}
	}
	completed := workoutsTotal % n
	remaining := n - completed
	nextIndex := completed
	lap := workoutsTotal/n + 1
	nodes := make([]NodeView, n)
	for i, def := range Path {
		status := "remaining"
		if i < completed {
			status = "done"
		} else if i == nextIndex {
			status = "next"
		}
		nodes[i] = NodeView{
			ID:     def.ID,
			Label:  def.Label,
			Emoji:  def.Emoji,
			Index:  i,
			Status: status,
			X:      def.X,
			Y:      def.Y,
		}
	}
	return Snapshot{
		WorkoutsTotal: workoutsTotal,
		Completed:     completed,
		Remaining:     remaining,
		NextIndex:     nextIndex,
		Lap:           lap,
		Nodes:         nodes,
	}
}
