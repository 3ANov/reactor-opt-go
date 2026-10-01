package optimization

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"reactoropt/internal/model"
)

type BFGS struct{}
type CMAES struct{}

func (BFGS) ID() string  { return "bfgs" }
func (CMAES) ID() string { return "cma_es" }

func (BFGS) Run(ctx context.Context, problem *model.Problem, params Parameters) (Result, error) {
	return gonumRun(ctx, problem, params, "bfgs")
}

func (CMAES) Run(ctx context.Context, problem *model.Problem, params Parameters) (Result, error) {
	return gonumRun(ctx, problem, params, "cma_es")
}

func gonumRun(ctx context.Context, problem *model.Problem, parameters Parameters, method string) (Result, error) {
	parameters = parameters.Normalized()
	if err := parameters.Validate(method); err != nil {
		return Result{}, err
	}
	if parameters.Start != nil && !problem.Domain.Contains(*parameters.Start) {
		return Result{}, fmt.Errorf("Начальная точка находится вне допустимой области")
	}
	evaluator := newEvaluation(ctx, problem, parameters, method)
	if evaluator.shouldStop() {
		return evaluator.complete()
	}
	vertices := problem.Domain.Vertices()
	if len(vertices) == 0 {
		evaluator.stop("no_feasible_region", "Допустимых решений нет")
		return evaluator.complete()
	}
	evaluator.source = "vertex"
	evaluator.beginStage()
	for _, vertex := range vertices {
		if _, ok := evaluator.score(vertex); !ok {
			break
		}
	}
	evaluator.finishStage(0)
	if evaluator.shouldStop() {
		return evaluator.complete()
	}
	if len(vertices) == 1 {
		evaluator.stop("converged", "Допустимая область состоит из одной точки")
		return evaluator.complete()
	}
	statuses := []string{}
	allConverged := true
	randomSource := rand.NewPCG(parameters.Seed, parameters.Seed^0x9e3779b97f4a7c15)
	for _, region := range searchRegions(vertices, method) {
		if !evaluator.canIterate() {
			break
		}
		status, converged := evaluator.runGonumRegion(region, randomSource)
		statuses = append(statuses, status)
		allConverged = allConverged && converged
	}
	evaluator.result.LibraryStatus = strings.Join(statuses, ", ")
	if evaluator.canIterate() {
		if allConverged {
			evaluator.stop("converged", convergenceReason(parameters))
		} else {
			evaluator.stop("stalled", "Часть локальных запусков остановилась до выполнения выбранного критерия; сохранён лучший проверенный допустимый результат")
		}
	}
	return evaluator.complete()
}
