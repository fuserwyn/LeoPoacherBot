package models

import (
	"time"
)

// MessageLog представляет запись о сообщении пользователя
type MessageLog struct {
	UserID             int64     `json:"user_id" db:"user_id"`
	ChatID             int64     `json:"chat_id" db:"chat_id"`
	Username           string    `json:"username" db:"username"`
	Calories           int       `json:"calories" db:"calories"`
	StreakDays         int       `json:"streak_days" db:"streak_days"`
	LastTrainingDate   *string   `json:"last_training_date" db:"last_training_date"`
	LastMessage        string    `json:"last_message" db:"last_message"`
	HasTrainingDone    bool      `json:"has_training_done" db:"has_training_done"`
	HasSickLeave       bool      `json:"has_sick_leave" db:"has_sick_leave"`
	HasHealthy         bool      `json:"has_healthy" db:"has_healthy"`
	IsDeleted          bool      `json:"is_deleted" db:"is_deleted"`
	TimerStartTime     *string   `json:"timer_start_time" db:"timer_start_time"`
	SickLeaveStartTime *string   `json:"sick_leave_start_time" db:"sick_leave_start_time"`
	SickLeaveEndTime   *string   `json:"sick_leave_end_time" db:"sick_leave_end_time"`
	SickTime           *string   `json:"sick_time" db:"sick_time"`
	RestTimeTillDel    *string   `json:"rest_time_till_del" db:"rest_time_till_del"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

// TrainingLog представляет отчет о тренировке
type TrainingLog struct {
	UserID     int64     `json:"user_id" db:"user_id"`
	Username   string    `json:"username" db:"username"`
	LastReport string    `json:"last_report" db:"last_report"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// ChatMember представляет участника чата
type ChatMember struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Status   string `json:"status"`
}

// TimerInfo представляет информацию о таймере
type TimerInfo struct {
	UserID         int64
	ChatID         int64
	Username       string
	WarningTask    chan bool
	RemovalTask    chan bool
	TimerStartTime string
}
