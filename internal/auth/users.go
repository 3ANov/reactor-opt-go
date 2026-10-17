package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func normalizeLogin(login string) (string, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if len(login) < 1 || len(login) > 64 {
		return "", fmt.Errorf("Логин должен содержать от 1 до 64 символов")
	}
	for _, character := range login {
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' ||
			character == '_' || character == '.' || character == '-') {
			return "", fmt.Errorf("В логине разрешены латинские буквы, цифры, точка, дефис и подчёркивание")
		}
	}
	return login, nil
}

func newAccount(login, password, role string) (account, error) {
	login, err := normalizeLogin(login)
	if err != nil {
		return account{}, err
	}
	if role != "admin" && role != "user" {
		return account{}, fmt.Errorf("Неизвестная роль")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return account{}, err
	}
	return account{
		User:         User{ID: rand.Text(), Login: login, Role: role, Active: true},
		PasswordHash: hash,
	}, nil
}

const insertAccount = `
INSERT INTO users (id, login, password_hash, role, active, session_version)
VALUES (?, ?, ?, ?, 1, 0)`

func (service *Service) NeedsAdministrator() (bool, error) {
	var count int
	err := service.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count == 0, err
}

func (service *Service) CreateFirstAdministrator(login, password string) (User, error) {
	userAccount, err := newAccount(login, password, "admin")
	if err != nil {
		return User{}, err
	}
	err = service.immediate(func(connection *sql.Conn) error {
		var count int
		if err := connection.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("Первый администратор уже создан")
		}
		_, err := connection.ExecContext(context.Background(), insertAccount,
			userAccount.ID, userAccount.Login, userAccount.PasswordHash, userAccount.Role)
		return err
	})
	return userAccount.User, err
}

func (service *Service) administrator(connection *sql.Conn, token string) (account, error) {
	userAccount, err := service.resolve(connection, token)
	if err != nil {
		return account{}, err
	}
	if userAccount.Role != "admin" {
		return account{}, fmt.Errorf("Требуются права администратора")
	}
	return userAccount, nil
}

func (service *Service) ListUsers(token string) ([]User, error) {
	users := []User{}
	err := service.immediate(func(connection *sql.Conn) error {
		if _, err := service.administrator(connection, token); err != nil {
			return err
		}
		rows, err := connection.QueryContext(context.Background(), `SELECT id,login,role,active FROM users ORDER BY login`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var user User
			if err := rows.Scan(&user.ID, &user.Login, &user.Role, &user.Active); err != nil {
				return err
			}
			users = append(users, user)
		}
		return rows.Err()
	})
	return users, err
}

func (service *Service) CreateUser(token, login, password, role string) (User, error) {
	userAccount, err := newAccount(login, password, role)
	if err != nil {
		return User{}, err
	}
	err = service.immediate(func(connection *sql.Conn) error {
		if _, err := service.administrator(connection, token); err != nil {
			return err
		}
		var count int
		if err := connection.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM users WHERE login=?`, userAccount.Login).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("Такой логин уже существует")
		}
		_, err := connection.ExecContext(context.Background(), insertAccount,
			userAccount.ID, userAccount.Login, userAccount.PasswordHash, userAccount.Role)
		return err
	})
	return userAccount.User, err
}

func (service *Service) UpdateUser(token, id, role string, active bool) error {
	if role != "admin" && role != "user" {
		return fmt.Errorf("Неизвестная роль")
	}
	return service.immediate(func(connection *sql.Conn) error {
		if _, err := service.administrator(connection, token); err != nil {
			return err
		}
		userAccount, err := readAccount(connection, byID, id)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("Пользователь не найден")
		}
		if err != nil {
			return err
		}
		if userAccount.Role == role && userAccount.Active == active {
			return nil
		}
		if userAccount.Role == "admin" && userAccount.Active && (role != "admin" || !active) {
			var count int
			if err := connection.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM users WHERE role='admin' AND active=1`).Scan(&count); err != nil {
				return err
			}
			if count <= 1 {
				return fmt.Errorf("Нельзя отключить или понизить последнего активного администратора")
			}
		}
		_, err = connection.ExecContext(context.Background(), `UPDATE users SET role=?,active=?,session_version=session_version+1 WHERE id=?`, role, active, id)
		return err
	})
}

func (service *Service) ResetPassword(token, id, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return service.immediate(func(connection *sql.Conn) error {
		actor, err := service.administrator(connection, token)
		if err != nil {
			return err
		}
		if actor.ID == id {
			return fmt.Errorf("Для своего пароля используйте смену пароля с проверкой текущего")
		}
		result, err := connection.ExecContext(context.Background(), `UPDATE users SET password_hash=?,session_version=session_version+1 WHERE id=?`, hash, id)
		if err != nil {
			return err
		}
		affectedRows, err := result.RowsAffected()
		if affectedRows == 0 && err == nil {
			return fmt.Errorf("Пользователь не найден")
		}
		return err
	})
}

func (service *Service) ChangePassword(token, oldPassword, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return service.immediate(func(connection *sql.Conn) error {
		userAccount, err := service.resolve(connection, token)
		if err != nil {
			return err
		}
		if !VerifyPassword(oldPassword, userAccount.PasswordHash) {
			return fmt.Errorf("Текущий пароль указан неверно")
		}
		_, err = connection.ExecContext(context.Background(), `UPDATE users SET password_hash=?,session_version=session_version+1 WHERE id=?`, hash, userAccount.ID)
		return err
	})
}
