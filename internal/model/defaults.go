package model

import (
	_ "embed"
	"encoding/json"
)

//go:embed default_tasks.json
var defaults []byte

func DefaultTasks() []Task {
	var tasks []Task
	if err := json.Unmarshal(defaults, &tasks); err != nil {
		panic(err)
	}
	return tasks
}
