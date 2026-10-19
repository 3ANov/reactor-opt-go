package app

import "reactoropt/internal/auth"

func (service *Service) Setup(login, password string) (State, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, err := service.auth.CreateFirstAdministrator(login, password); err != nil {
		return State{}, err
	}
	return service.login(login, password)
}

func (service *Service) login(login, password string) (State, error) {
	session, err := service.auth.Login(login, password)
	if err != nil {
		return State{}, err
	}
	service.logout()
	service.token = session.Token
	return service.bootstrap()
}

func (service *Service) Login(login, password string) (State, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.login(login, password)
}

func (service *Service) logout() {
	if service.cancel != nil {
		service.cancel()
	}
	service.auth.Logout(service.token)
	service.token = ""
	service.job = JobState{ID: service.job.ID + 1, Status: "idle"}
}

func (service *Service) Logout() {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.logout()
}

func (service *Service) Users() ([]auth.User, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.auth.ListUsers(service.token)
}

func (service *Service) CreateUser(login, password, role string) (auth.User, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.auth.CreateUser(service.token, login, password, role)
}

func (service *Service) UpdateUser(id, role string, active bool) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.auth.UpdateUser(service.token, id, role, active)
}

func (service *Service) ResetPassword(id, password string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.auth.ResetPassword(service.token, id, password)
}

func (service *Service) ChangePassword(oldPassword, newPassword string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.auth.ChangePassword(service.token, oldPassword, newPassword); err != nil {
		return err
	}
	service.logout()
	return nil
}
