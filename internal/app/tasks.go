package app

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"

	"reactoropt/internal/model"
)

func (service *Service) Tasks() ([]model.Task, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, err := service.current(); err != nil {
		return nil, err
	}
	return service.store.Tasks()
}

func (service *Service) requireTaskEditor() error {
	user, err := service.current()
	if err != nil {
		return err
	}
	if service.running {
		return fmt.Errorf("Дождитесь завершения расчёта")
	}
	if user.Role != "admin" {
		return fmt.Errorf("Изменение библиотеки задач доступно только администратору")
	}
	return nil
}

func (service *Service) SaveTask(task model.Task) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.requireTaskEditor(); err != nil {
		return err
	}
	return service.store.SaveTask(task)
}

func (service *Service) CopyTask(id string) (model.Task, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.requireTaskEditor(); err != nil {
		return model.Task{}, err
	}
	task, err := service.store.Task(id)
	if err != nil {
		return task, err
	}
	task.ID = "task-" + strings.ToLower(rand.Text())
	task.Name += " (копия)"
	task.Builtin = false
	return task, service.store.SaveTask(task)
}

func (service *Service) DeleteTask(id string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.requireTaskEditor(); err != nil {
		return err
	}
	return service.store.DeleteTask(id)
}

func (service *Service) ExportTask(id string) (string, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, err := service.current(); err != nil {
		return "", err
	}
	task, err := service.store.Task(id)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(task, "", "  ")
	return string(data), err
}

func (service *Service) ImportTasks(text string) ([]model.Task, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.requireTaskEditor(); err != nil {
		return nil, err
	}
	if len(text) > 65536 {
		return nil, fmt.Errorf("Размер JSON не должен превышать 64 КБ")
	}
	text = strings.TrimPrefix(strings.TrimSpace(text), "\ufeff")
	tasks := []model.Task{}
	var err error
	if strings.HasPrefix(text, "[") {
		err = json.Unmarshal([]byte(text), &tasks)
	} else {
		var task model.Task
		err = json.Unmarshal([]byte(text), &task)
		tasks = append(tasks, task)
	}
	if err != nil {
		return nil, fmt.Errorf("Неверный JSON: %w", err)
	}
	if len(tasks) == 0 || len(tasks) > 100 {
		return nil, fmt.Errorf("Нужен список от 1 до 100 задач")
	}
	for i := range tasks {
		tasks[i].Builtin = false
	}
	if err := service.store.SaveTasks(tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}
