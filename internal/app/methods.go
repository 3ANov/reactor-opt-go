package app

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"reactoropt/internal/optimization"
)

func (service *Service) methods() ([]optimization.Definition, error) {
	definitions := optimization.Methods()
	for index, definition := range definitions {
		var data string
		err := service.store.DB.QueryRow(`SELECT defaults_json FROM methods WHERE id=?`, definition.ID).Scan(&data)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		parameters := definition.Defaults
		if err := json.Unmarshal([]byte(data), &parameters); err != nil {
			return nil, fmt.Errorf("Повреждены настройки метода %s", definition.Name)
		}
		parameters = parameters.Normalized()
		if err := parameters.Validate(definition.ID); err != nil {
			return nil, err
		}
		definitions[index].Defaults = parameters
	}
	return definitions, nil
}

func (service *Service) Methods() ([]optimization.Definition, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, err := service.current(); err != nil {
		return nil, err
	}
	return service.methods()
}

func (service *Service) SaveMethodSettings(id string, parameters optimization.Parameters) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	user, err := service.current()
	if err != nil {
		return err
	}
	if user.Role != "admin" {
		return fmt.Errorf("Требуются права администратора")
	}
	for _, definition := range optimization.Methods() {
		if definition.ID != id {
			continue
		}
		parameters = parameters.Normalized()
		if err := parameters.Validate(id); err != nil {
			return err
		}
		data, err := json.Marshal(parameters)
		if err != nil {
			return err
		}
		_, err = service.store.DB.Exec(`
            INSERT INTO methods (id, name, description, category, enabled, defaults_json)
            VALUES (?, ?, ?, 'builtin', 1, ?)
            ON CONFLICT(id) DO UPDATE SET defaults_json = excluded.defaults_json
        `, id, definition.Name, definition.Description, string(data))
		return err
	}
	return fmt.Errorf("Неизвестный метод %q", id)
}
