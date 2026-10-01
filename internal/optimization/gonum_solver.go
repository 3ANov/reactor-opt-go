package optimization

import (
	"context"
	"math"
	"math/rand/v2"

	"gonum.org/v1/gonum/diff/fd"
	"gonum.org/v1/gonum/optimize"
)

func (evaluator *evaluation) runGonumRegion(region parameterization, randomSource rand.Source) (string, bool) {
	evaluator.source = region.source
	evaluator.beginStage()
	initialEvaluations := evaluator.result.Evaluations
	converger := &iterationConverger{parameters: evaluator.params, region: region}
	libraryResult, err := evaluator.minimizeRegion(region, converger, randomSource)
	evaluator.finishStage(evaluator.params.InitialStep)

	if evaluator.result.Method == "cma_es" {
		evaluator.result.Iterations += (evaluator.result.Evaluations - initialEvaluations) / evaluator.params.Population
	}
	status := "Failure"
	if libraryResult != nil {
		status = libraryResult.Status.String()
		if evaluator.result.Method == "bfgs" {
			evaluator.result.Iterations += max(0, libraryResult.MajorIterations-1)
		}
		evaluator.result.LibraryIterations += libraryResult.MajorIterations
		evaluator.result.LibraryFunctionCalls += libraryResult.FuncEvaluations
		evaluator.result.LibraryGradientCalls += libraryResult.GradEvaluations
		if libraryResult.Status == optimize.IterationLimit && !evaluator.shouldStop() {
			evaluator.stop("iteration_limit", "Достигнут лимит итераций")
		}
	}
	if err != nil {
		evaluator.result.LibraryErrors = append(evaluator.result.LibraryErrors, err.Error())
	}
	converged := libraryResult != nil && err == nil && converger.matched && !evaluator.shouldStop()
	if converged {
		evaluator.recordConvergence(converger.change)
	}
	return status, converged
}

func (evaluator *evaluation) minimizeRegion(region parameterization, converger optimize.Converger, randomSource rand.Source) (*optimize.Result, error) {
	parameters := evaluator.params
	objective := func(coordinates []float64) float64 {
		return evaluator.scoreCoordinates(region, coordinates)
	}
	problem := optimize.Problem{Func: objective, Status: evaluator.gonumStatus}
	settings := &optimize.Settings{
		Concurrent:      1,
		MajorIterations: parameters.MaxIterations - evaluator.result.Iterations,
		Converger:       converger,
	}
	var solver optimize.Method
	if evaluator.result.Method == "cma_es" {
		length := 0.0
		for _, first := range region.points {
			for _, second := range region.points {
				length = math.Max(length, distance(first, second))
			}
		}
		// Convert the physical scale to internal coordinates.
		scale := math.Max(1e-3, math.Min(2, parameters.InitialStep/length))
		solver = &optimize.CmaEsChol{
			InitStepSize: scale,
			Population:   parameters.Population,
			// Compare generations; the evaluator keeps the best overall candidate.
			ForgetBest: true,
			Src:        randomSource,
		}
	} else {
		// BFGS counts the initial location as a major iteration before taking a step.
		settings.MajorIterations++
		problem.Grad = func(gradient, coordinates []float64) {
			numericalGradient(gradient, objective, coordinates, parameters.GradientStep)
		}
		settings.GradientThreshold = parameters.GradientTolerance
		solver = &optimize.BFGS{GradStopThreshold: parameters.GradientTolerance}
	}
	return optimize.Minimize(problem, region.initial(parameters.Start), settings, solver)
}

func (evaluator *evaluation) scoreCoordinates(region parameterization, coordinates []float64) float64 {
	if evaluator.shouldStop() {
		return 0
	}
	for _, coordinate := range coordinates {
		if math.IsNaN(coordinate) || math.IsInf(coordinate, 0) {
			evaluator.stop("failed", "Метод получил неконечную координату")
			return 0
		}
	}
	point := region.point(coordinates)
	// Only rounding errors are corrected; the transformation itself is feasible.
	if !evaluator.problem.Domain.Contains(point) {
		point = evaluator.problem.Domain.Project(point)
	}
	score, ok := evaluator.score(point)
	if !ok {
		return 0
	}
	return score
}

func (evaluator *evaluation) gonumStatus() (optimize.Status, error) {
	if !evaluator.shouldStop() {
		return optimize.NotTerminated, nil
	}
	switch evaluator.result.Status {
	case "budget_exhausted":
		return optimize.FunctionEvaluationLimit, nil
	case "cancelled":
		return optimize.Failure, context.Canceled
	default:
		return optimize.Failure, evaluator.err
	}
}

func numericalGradient(gradient []float64, objective func([]float64) float64, coordinates []float64, step float64) {
	fd.Gradient(gradient, objective, coordinates, &fd.Settings{Formula: fd.Central, Step: step, Concurrent: false})
}
