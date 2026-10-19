package app

import (
	"context"
	"sync"

	"reactoropt/internal/auth"
	"reactoropt/internal/model"
	"reactoropt/internal/optimization"
	"reactoropt/internal/storage"
)

type State struct {
	NeedsAdmin bool                      `json:"needs_admin"`
	User       *auth.User                `json:"user"`
	Tasks      []model.Task              `json:"tasks"`
	Methods    []optimization.Definition `json:"methods"`
}

type Service struct {
	mu      sync.Mutex
	store   *storage.Store
	auth    *auth.Service
	token   string
	job     JobState
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	emit    func(JobEvent)
	closing bool
}

func New(store *storage.Store, emit func(JobEvent)) *Service {
	return &Service{
		store: store,
		auth:  auth.New(store.DB),
		job:   JobState{Status: "idle"},
		emit:  emit,
	}
}

func (service *Service) current() (auth.User, error) { return service.auth.Resolve(service.token) }

func (service *Service) bootstrap() (State, error) {
	state := State{Tasks: []model.Task{}, Methods: optimization.Methods()}
	var err error
	state.NeedsAdmin, err = service.auth.NeedsAdministrator()
	if err != nil {
		return state, err
	}
	if service.token == "" {
		return state, nil
	}
	user, err := service.current()
	if err != nil {
		service.logout()
		return state, nil
	}
	state.User = &user
	state.Tasks, err = service.store.Tasks()
	if err != nil {
		return state, err
	}
	state.Methods, err = service.methods()
	return state, err
}

func (service *Service) Bootstrap() (State, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.bootstrap()
}

//wails:ignore
func (service *Service) Close() error {
	service.mu.Lock()
	service.closing = true
	if service.cancel != nil {
		service.cancel()
	}
	service.mu.Unlock()
	service.wg.Wait()
	return service.store.Close()
}
