package optimization

import (
	"math"

	"gonum.org/v1/gonum/optimize"
)

func (parameters Parameters) Normalized() Parameters {
	switch parameters.StoppingCriterion {
	case "accuracy":
		parameters.StoppingCriterion = "coordinates"
	case "iterations":
		parameters.StoppingCriterion = "evaluations"
	}
	return parameters
}

type iterationChange struct {
	function    float64
	coordinates float64
}

func changesBetween(previous, current []scoredPoint) (iterationChange, bool) {
	change := iterationChange{}
	if len(previous) == 0 || len(previous) != len(current) {
		return change, false
	}
	for i := range previous {
		functionChange := math.Abs(current[i].score - previous[i].score)
		coordinateChange := distance(current[i].point, previous[i].point)
		if math.IsNaN(functionChange) || math.IsInf(functionChange, 0) ||
			math.IsNaN(coordinateChange) || math.IsInf(coordinateChange, 0) {
			return iterationChange{}, false
		}
		change.function = math.Max(change.function, functionChange)
		change.coordinates = math.Max(change.coordinates, coordinateChange)
	}
	return change, true
}

func (change iterationChange) meets(parameters Parameters) bool {
	switch parameters.StoppingCriterion {
	case "function":
		return change.function <= parameters.FunctionPrecision
	case "coordinates":
		return change.coordinates <= parameters.Precision
	default:
		return false
	}
}

func (evaluator *evaluation) recordConvergence(change iterationChange) {
	if evaluator.result.FunctionChange == nil || change.function > *evaluator.result.FunctionChange {
		value := change.function
		evaluator.result.FunctionChange = &value
	}
	if evaluator.result.CoordinateChange == nil || change.coordinates > *evaluator.result.CoordinateChange {
		value := change.coordinates
		evaluator.result.CoordinateChange = &value
	}
}

func convergenceReason(parameters Parameters) string {
	if parameters.StoppingCriterion == "function" {
		return "Изменение функции между итерациями не превышает ε"
	}
	return "Изменение координат между итерациями не превышает δ"
}

type iterationConverger struct {
	parameters  Parameters
	region      parameterization
	previous    scoredPoint
	initialized bool
	change      iterationChange
	matched     bool
}

func (converger *iterationConverger) Init(int) {
	converger.initialized = false
	converger.matched = false
	converger.change = iterationChange{}
}

func (converger *iterationConverger) Converged(location *optimize.Location) optimize.Status {
	current := scoredPoint{score: location.F, point: converger.region.point(location.X)}
	if !converger.initialized {
		converger.previous = current
		converger.initialized = true
		return optimize.NotTerminated
	}
	change, valid := changesBetween([]scoredPoint{converger.previous}, []scoredPoint{current})
	converger.previous = current
	if !valid || !change.meets(converger.parameters) {
		return optimize.NotTerminated
	}
	converger.change = change
	converger.matched = true
	if converger.parameters.StoppingCriterion == "function" {
		return optimize.FunctionConvergence
	}
	return optimize.StepConvergence
}
