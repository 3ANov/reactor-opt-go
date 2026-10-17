package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

type User struct {
	ID     string `json:"id"`
	Login  string `json:"login"`
	Role   string `json:"role"`
	Active bool   `json:"active"`
}

type Session struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type sessionRecord struct {
	UserID         string
	SessionVersion int64
}

type account struct {
	User
	PasswordHash   string
	SessionVersion int64
}

type Service struct {
	db        *sql.DB
	mu        sync.Mutex
	sessions  map[string]sessionRecord
	dummyHash string
}

func New(db *sql.DB) *Service {
	dummyHash, _ := HashPassword("invalid-user-password")
	return &Service{
		db:        db,
		sessions:  map[string]sessionRecord{},
		dummyHash: dummyHash,
	}
}

func (service *Service) immediate(action func(*sql.Conn) error) error {
	ctx := context.Background()
	connection, err := service.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer connection.ExecContext(ctx, "ROLLBACK")
	if err := action(connection); err != nil {
		return err
	}
	_, err = connection.ExecContext(ctx, "COMMIT")
	return err
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const byID = `SELECT id,login,password_hash,role,active,session_version FROM users WHERE id=?`

const byLogin = `SELECT id,login,password_hash,role,active,session_version FROM users WHERE login=?`

func readAccount(querier rowQuerier, query string, value any) (account, error) {
	var userAccount account
	err := querier.QueryRowContext(context.Background(), query, value).Scan(
		&userAccount.ID, &userAccount.Login, &userAccount.PasswordHash,
		&userAccount.Role, &userAccount.Active, &userAccount.SessionVersion,
	)
	return userAccount, err
}

func (service *Service) Login(login, password string) (Session, error) {
	login, err := normalizeLogin(login)
	if err != nil {
		return Session{}, fmt.Errorf("Неверный логин или пароль")
	}
	userAccount, err := readAccount(service.db, byLogin, login)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Session{}, err
	}
	hash := userAccount.PasswordHash
	if err != nil {
		hash = service.dummyHash
	}
	valid, settings := checkPassword(password, hash)
	if err != nil || !valid || !userAccount.Active {
		return Session{}, fmt.Errorf("Неверный логин или пароль")
	}
	if *settings != passwordParams {
		newHash, err := HashPassword(password)
		if err != nil {
			return Session{}, err
		}
		if _, err = service.db.Exec(`UPDATE users SET password_hash=? WHERE id=? AND password_hash=?`, newHash, userAccount.ID, userAccount.PasswordHash); err != nil {
			return Session{}, err
		}
	}
	// Each Text call supplies at least 128 random bits.
	token := rand.Text() + rand.Text()
	service.mu.Lock()
	service.sessions[token] = sessionRecord{
		UserID:         userAccount.ID,
		SessionVersion: userAccount.SessionVersion,
	}
	service.mu.Unlock()
	return Session{Token: token, User: userAccount.User}, nil
}

func (service *Service) Logout(token string) {
	service.mu.Lock()
	delete(service.sessions, token)
	service.mu.Unlock()
}

func (service *Service) resolve(querier rowQuerier, token string) (account, error) {
	service.mu.Lock()
	record, ok := service.sessions[token]
	service.mu.Unlock()
	if !ok {
		return account{}, fmt.Errorf("Необходимо войти в систему")
	}
	userAccount, err := readAccount(querier, byID, record.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return userAccount, err
	}
	if errors.Is(err, sql.ErrNoRows) || !userAccount.Active || userAccount.SessionVersion != record.SessionVersion {
		service.Logout(token)
		return account{}, fmt.Errorf("Сеанс завершён. Войдите снова")
	}
	return userAccount, nil
}

func (service *Service) Resolve(token string) (User, error) {
	userAccount, err := service.resolve(service.db, token)
	return userAccount.User, err
}
