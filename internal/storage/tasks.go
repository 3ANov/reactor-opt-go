package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"reactoropt/internal/model"
)

const taskColumns = `
    id, name, description, objective, direction,
    variables_json, parameters_json, constraints_json, derived_metrics_json,
    is_builtin, objective_label, objective_unit`

const insertTask = `INSERT INTO tasks (` + taskColumns + `) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const updateTask = `
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    description = excluded.description,
    objective = excluded.objective,
    direction = excluded.direction,
    variables_json = excluded.variables_json,
    parameters_json = excluded.parameters_json,
    constraints_json = excluded.constraints_json,
    derived_metrics_json = excluded.derived_metrics_json,
    objective_label = excluded.objective_label,
    objective_unit = excluded.objective_unit
WHERE tasks.is_builtin = 0`

func taskValues(task model.Task) ([]any, error) {
	values := []any{task.ID, task.Name, task.Description, task.Objective, task.Direction}
	for _, value := range []any{task.Variables, task.Coefficients, task.Constraints, task.DerivedMetrics} {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		values = append(values, string(encoded))
	}
	return append(values, task.Builtin, task.ObjectiveLabel, task.ObjectiveUnit), nil
}

func (store *Store) Tasks() ([]model.Task, error) {
	rows, err := store.DB.Query(`SELECT ` + taskColumns + ` FROM tasks ORDER BY is_builtin DESC,name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []model.Task{}
	for rows.Next() {
		task, err := readTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (store *Store) Task(id string) (model.Task, error) {
	row := store.DB.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=?`, id)
	task, err := readTask(row)
	if err == sql.ErrNoRows {
		return model.Task{}, fmt.Errorf("Задача не найдена")
	}
	return task, err
}

type taskScanner interface {
	Scan(...any) error
}

func readTask(row taskScanner) (model.Task, error) {
	var task model.Task
	var variablesJSON, parametersJSON, constraintsJSON, metricsJSON string
	if err := row.Scan(
		&task.ID, &task.Name, &task.Description, &task.Objective, &task.Direction,
		&variablesJSON, &parametersJSON, &constraintsJSON, &metricsJSON,
		&task.Builtin, &task.ObjectiveLabel, &task.ObjectiveUnit,
	); err != nil {
		return task, err
	}
	fields := []struct {
		data   string
		target any
	}{
		{data: variablesJSON, target: &task.Variables},
		{data: parametersJSON, target: &task.Coefficients},
		{data: constraintsJSON, target: &task.Constraints},
		{data: metricsJSON, target: &task.DerivedMetrics},
	}
	for _, field := range fields {
		if len(field.data) > 65536 {
			return task, fmt.Errorf("Задача %s содержит слишком большие данные", task.ID)
		}
		if err := json.Unmarshal([]byte(field.data), field.target); err != nil {
			return task, fmt.Errorf("Задача %s: %w", task.ID, err)
		}
	}
	return task, nil
}

func (store *Store) SaveTask(task model.Task) error {
	return store.SaveTasks([]model.Task{task})
}

func (store *Store) SaveTasks(tasks []model.Task) error {
	ids := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		if ids[task.ID] {
			return fmt.Errorf("Повтор ID задания")
		}
		ids[task.ID] = true
		if task.Builtin {
			return fmt.Errorf("Встроенную задачу можно только скопировать")
		}
		if err := task.Validate(); err != nil {
			return err
		}
	}
	transaction, err := store.DB.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	for _, task := range tasks {
		values, err := taskValues(task)
		if err != nil {
			return err
		}
		result, err := transaction.Exec(insertTask+updateTask, values...)
		if err != nil {
			return err
		}
		affectedRows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affectedRows == 0 {
			return fmt.Errorf("Встроенную задачу нельзя изменять; создайте копию с другим ID")
		}
	}
	return transaction.Commit()
}

func (store *Store) DeleteTask(id string) error {
	transaction, err := store.DB.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var builtin bool
	if err := transaction.QueryRow(`SELECT is_builtin FROM tasks WHERE id=?`, id).Scan(&builtin); err != nil {
		return fmt.Errorf("Задача не найдена: %w", err)
	}
	if builtin {
		return fmt.Errorf("Встроенную задачу нельзя удалить")
	}
	if _, err = transaction.Exec(`DELETE FROM tasks WHERE id=?`, id); err != nil {
		return err
	}
	return transaction.Commit()
}
