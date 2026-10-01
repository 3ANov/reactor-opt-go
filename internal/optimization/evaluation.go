package optimization

import (
	"context"
	"fmt"
	"math"

	"reactoropt/internal/constraints"
	"reactoropt/internal/model"
)

type evaluation struct {
	ctx        context.Context
	problem    *model.Problem
	params     Parameters
	result     Result
	stageStart int
	source     string
	err        error
}

const historyLimit = 2000

func newEvaluation(ctx context.Context, problem *model.Problem, parameters Parameters, method string) *evaluation {
	return &evaluation{
		ctx:     ctx,
		problem: problem,
		params:  parameters,
		result: Result{
			Method:       method,
			History:      []BestPoint{},
			StageHistory: []Stage{},
			Metrics:      []model.MetricValue{},
		},
	}
}

func (evaluator *evaluation) stop(status, reason string) {
	if evaluator.result.Status == "" {
		evaluator.result.Status = status
		evaluator.result.Reason = reason
	}
}

func (evaluator *evaluation) shouldStop() bool {
	if evaluator.result.Status != "" {
		return true
	}
	if evaluator.ctx.Err() != nil {
		evaluator.stop("cancelled", "Расчёт отменён")
	} else if evaluator.result.Evaluations >= evaluator.params.MaxEvaluations {
		evaluator.stop("budget_exhausted", "Достигнут лимит вычислений")
	}
	return evaluator.result.Status != ""
}

func (evaluator *evaluation) canIterate() bool {
	if evaluator.shouldStop() {
		return false
	}
	if evaluator.result.Iterations >= evaluator.params.MaxIterations {
		evaluator.stop("iteration_limit", "Достигнут лимит итераций")
		return false
	}
	return true
}

// Single-threaded solvers count every objective call here, including gradient probes.
func (evaluator *evaluation) score(point constraints.Point) (float64, bool) {
	if evaluator.shouldStop() {
		return 0, false
	}
	if !evaluator.problem.Domain.Contains(point) {
		return math.Inf(1), true
	}
	evaluator.result.Evaluations++
	value, err := evaluator.problem.Objective(point)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		if err == nil {
			err = fmt.Errorf("Целевая функция вернула неконечное значение")
		}
		evaluator.err = err
		evaluator.stop("failed", err.Error())
		return 0, false
	}
	directionSign := 1.0
	if evaluator.problem.Task.Direction == "max" {
		directionSign = -1
	}
	minimizationScore := directionSign * value
	if evaluator.result.Value != nil && minimizationScore >= directionSign*(*evaluator.result.Value) {
		return minimizationScore, true
	}
	bestPoint, bestValue := point, value
	evaluator.result.Point = &bestPoint
	evaluator.result.Value = &bestValue
	evaluator.result.Source = evaluator.source
	if !evaluator.params.KeepHistory {
		return minimizationScore, true
	}
	evaluator.result.History = append(evaluator.result.History, BestPoint{
		Point:      point,
		Value:      value,
		Evaluation: evaluator.result.Evaluations,
		Stage:      evaluator.result.Stages,
	})
	if len(evaluator.result.History) > historyLimit {
		evaluator.result.History = evaluator.result.History[1:]
		evaluator.result.HistoryTruncated = true
	}
	return minimizationScore, true
}

func (evaluator *evaluation) beginStage() {
	evaluator.result.Stages++
	evaluator.stageStart = evaluator.result.Evaluations
}

func (evaluator *evaluation) finishStage(step float64) {
	if !evaluator.params.KeepHistory {
		return
	}
	evaluator.result.StageHistory = append(evaluator.result.StageHistory, Stage{
		Number:      evaluator.result.Stages,
		Step:        step,
		Evaluations: evaluator.result.Evaluations - evaluator.stageStart,
		BestPoint:   evaluator.result.Point,
		BestValue:   evaluator.result.Value,
	})
	if len(evaluator.result.StageHistory) > historyLimit {
		evaluator.result.StageHistory = evaluator.result.StageHistory[1:]
		evaluator.result.HistoryTruncated = true
	}
}

func (evaluator *evaluation) complete() (Result, error) {
	if evaluator.result.Point == nil && evaluator.result.Status == "" {
		evaluator.stop("no_feasible_point", "Нет вычисленной допустимой точки")
	}
	if evaluator.result.Value != nil {
		metrics, err := evaluator.problem.Metrics(*evaluator.result.Value)
		if err != nil {
			evaluator.err = err
			evaluator.result.Status = "failed"
			evaluator.result.Reason = err.Error()
		} else {
			evaluator.result.Metrics = metrics
		}
	}
	if evaluator.result.Status != "converged" {
		evaluator.result.FunctionChange = nil
		evaluator.result.CoordinateChange = nil
	}
	return evaluator.result, evaluator.err
}
