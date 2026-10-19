package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"reactoropt/internal/constraints"
	"reactoropt/internal/model"
	"reactoropt/internal/optimization"
)

type JobState struct {
	ID     uint64               `json:"id"`
	Status string               `json:"status"`
	Result *optimization.Result `json:"result"`
	Error  string               `json:"error"`
}

type JobEvent struct {
	ID     uint64 `json:"id"`
	Status string `json:"status"`
}

func (service *Service) Start(taskID, direction, methodID string, parameters optimization.Parameters) (JobState, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, err := service.current(); err != nil {
		return JobState{}, err
	}
	if service.running || service.closing {
		return JobState{}, fmt.Errorf("Расчёт уже выполняется или приложение закрывается")
	}
	method, err := optimization.Select(methodID)
	if err != nil {
		return JobState{}, err
	}
	if err := parameters.Validate(methodID); err != nil {
		return JobState{}, err
	}
	task, err := service.store.Task(taskID)
	if err != nil {
		return JobState{}, err
	}
	task.Direction = direction
	problem, err := task.Compile()
	if err != nil {
		if !errors.Is(err, constraints.ErrEmptyDomain) {
			return JobState{}, err
		}
		service.job = JobState{
			ID:     service.job.ID + 1,
			Status: "done",
			Result: &optimization.Result{
				Method: methodID,
				Status: "no_feasible_region",
				Reason: err.Error(),
			},
		}
		return service.job, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.cancel = cancel
	service.running = true
	service.job = JobState{ID: service.job.ID + 1, Status: "running"}
	service.wg.Add(1)
	go service.runJob(ctx, cancel, service.job.ID, service.token, method, problem, parameters)
	return service.job, nil
}

func (service *Service) runJob(
	ctx context.Context,
	cancel context.CancelFunc,
	id uint64,
	token string,
	method optimization.Method,
	problem *model.Problem,
	parameters optimization.Parameters,
) {
	defer service.wg.Done()
	defer cancel()
	go service.watchSession(ctx, cancel, token)

	result, err := method.Run(ctx, problem, parameters)
	service.finishJob(id, token, result, err)
}

func (service *Service) watchSession(ctx context.Context, cancel context.CancelFunc, token string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := service.auth.Resolve(token); err != nil {
				cancel()
				return
			}
		}
	}
}

func (service *Service) finishJob(id uint64, token string, result optimization.Result, err error) {
	service.mu.Lock()
	service.running = false
	if service.job.ID != id || service.token != token || service.closing {
		service.mu.Unlock()
		return
	}
	service.job.Status = "done"
	service.job.Result = &result
	if err != nil {
		service.job.Error = err.Error()
	}
	service.mu.Unlock()
	if service.emit != nil {
		service.emit(JobEvent{ID: id, Status: "done"})
	}
}

func (service *Service) Cancel() error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, err := service.current(); err != nil {
		return err
	}
	if service.cancel != nil {
		service.cancel()
	}
	return nil
}

func (service *Service) Job() (JobState, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, err := service.current(); err != nil {
		if service.cancel != nil {
			service.cancel()
		}
		return JobState{}, err
	}
	return service.job, nil
}
