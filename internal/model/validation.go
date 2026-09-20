package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"reactoropt/internal/constraints"
)

var taskID = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)

func (task Task) Validate() error {
	if !taskID.MatchString(task.ID) || strings.TrimSpace(task.Name) == "" ||
		strings.TrimSpace(task.Description) == "" || utf8.RuneCountInString(task.Name) > 200 ||
		utf8.RuneCountInString(task.Description) > 4000 {
		return fmt.Errorf("Укажите ID, название и описание задачи допустимой длины")
	}
	if strings.TrimSpace(task.ObjectiveLabel) == "" || utf8.RuneCountInString(task.ObjectiveLabel) > 32 || utf8.RuneCountInString(task.ObjectiveUnit) > 32 {
		return fmt.Errorf("Обозначение функции и единицы должны быть длиной до 32 символов")
	}
	data, err := json.Marshal(task)
	if err != nil || len(data) > 65536 {
		return fmt.Errorf("Данные задания должны быть конечными и занимать до 64 КБ")
	}
	problem, err := task.Compile()
	if err != nil {
		return err
	}
	displayNames := map[string]bool{}
	for _, variable := range task.Variables {
		if strings.TrimSpace(variable.DisplayName) == "" || displayNames[variable.DisplayName] ||
			utf8.RuneCountInString(variable.DisplayName) > 32 || utf8.RuneCountInString(variable.Unit) > 32 {
			return fmt.Errorf("Переменным нужны уникальные подписи и единицы до 32 символов")
		}
		displayNames[variable.DisplayName] = true
	}
	points := problem.Domain.Vertices()
	center := constraints.Point{}
	for _, point := range points {
		center[0] += point[0] / float64(len(points))
		center[1] += point[1] / float64(len(points))
	}
	if problem.Domain.Contains(center) {
		points = append(points, center)
	}
	for column := 0; column < 5; column++ {
		for row := 0; row < 5; row++ {
			point := constraints.Point{
				task.Variables[0].Min + (task.Variables[0].Max-task.Variables[0].Min)*float64(column)/4,
				task.Variables[1].Min + (task.Variables[1].Max-task.Variables[1].Min)*float64(row)/4,
			}
			if problem.Domain.Contains(point) {
				points = append(points, point)
			}
		}
	}
	for _, point := range points {
		value, err := problem.Objective(point)
		if err != nil {
			return fmt.Errorf("Формула не определена в точке %v: %w", point, err)
		}
		if _, err := problem.Metrics(value); err != nil {
			return err
		}
	}
	return nil
}
