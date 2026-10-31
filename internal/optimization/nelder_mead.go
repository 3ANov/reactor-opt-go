package optimization

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"

	"reactoropt/internal/constraints"
	"reactoropt/internal/model"
)

type NelderMead struct{}

func (NelderMead) ID() string { return "nelder_mead" }

type scoredPoint struct {
	score float64
	point constraints.Point
}
type projector func(constraints.Point) constraints.Point

func simplexDiameter(simplex []scoredPoint) float64 {
	largestDistance := 0.0
	for _, first := range simplex {
		for _, second := range simplex {
			largestDistance = math.Max(largestDistance, distance(first.point, second.point))
		}
	}
	return largestDistance
}

func sortSimplex(simplex []scoredPoint) {
	sort.Slice(simplex, func(i, j int) bool {
		if simplex[i].score != simplex[j].score {
			return simplex[i].score < simplex[j].score
		}
		if simplex[i].point[0] != simplex[j].point[0] {
			return simplex[i].point[0] < simplex[j].point[0]
		}
		return simplex[i].point[1] < simplex[j].point[1]
	})
}

func scorePoint(evaluator *evaluation, point constraints.Point, project projector) (scoredPoint, bool) {
	if project != nil {
		point = project(point)
	}
	score, ok := evaluator.score(point)
	return scoredPoint{score, point}, ok
}

// Infeasible 2D trials use a barrier; 1D trials stay on the feasible segment.
func nelderMeadIteration(evaluator *evaluation, simplex []scoredPoint, project projector) []scoredPoint {
	sortSimplex(simplex)
	worstIndex := len(simplex) - 1
	best := simplex[0]
	secondWorst := simplex[worstIndex-1]
	worst := simplex[worstIndex]
	dimension := float64(worstIndex)
	centroid := constraints.Point{}
	for _, vertex := range simplex[:worstIndex] {
		centroid[0] += vertex.point[0] / dimension
		centroid[1] += vertex.point[1] / dimension
	}

	reflectedPoint := interpolate(centroid, worst.point, -1)
	reflected, ok := scorePoint(evaluator, reflectedPoint, project)
	if !ok {
		return simplex
	}
	if reflected.score < best.score {
		expandedPoint := interpolate(centroid, worst.point, -2)
		expanded, ok := scorePoint(evaluator, expandedPoint, project)
		if ok && expanded.score < reflected.score {
			simplex[worstIndex] = expanded
		} else {
			simplex[worstIndex] = reflected
		}
		return simplex
	}
	if reflected.score < secondWorst.score {
		simplex[worstIndex] = reflected
		return simplex
	}
	if reflected.score < worst.score {
		contractedPoint := interpolate(centroid, worst.point, -0.5)
		outsideContracted, ok := scorePoint(evaluator, contractedPoint, project)
		if !ok {
			return simplex
		}
		if outsideContracted.score <= reflected.score {
			simplex[worstIndex] = outsideContracted
			return simplex
		}
	} else {
		contractedPoint := interpolate(centroid, worst.point, 0.5)
		insideContracted, ok := scorePoint(evaluator, contractedPoint, project)
		if !ok {
			return simplex
		}
		if insideContracted.score < worst.score {
			simplex[worstIndex] = insideContracted
			return simplex
		}
	}

	for i := 1; i < len(simplex); i++ {
		shrunkPoint := interpolate(best.point, simplex[i].point, 0.5)
		shrunk, ok := scorePoint(evaluator, shrunkPoint, project)
		if !ok {
			break
		}
		simplex[i] = shrunk
	}
	return simplex
}

func minimizeSimplex(evaluator *evaluation, points []constraints.Point, project projector) bool {
	if !evaluator.canIterate() {
		return false
	}
	evaluator.beginStage()
	simplex := []scoredPoint{}
	for _, point := range points {
		vertex, ok := scorePoint(evaluator, point, project)
		if !ok {
			break
		}
		simplex = append(simplex, vertex)
	}
	initialSimplex := make([]scoredPoint, len(points))
	for i, point := range points {
		initialSimplex[i].point = point
	}
	evaluator.finishStage(simplexDiameter(initialSimplex))
	if len(simplex) != len(points) {
		return false
	}
	if simplex[0].point == simplex[1].point ||
		(len(simplex) == 3 && distanceFromLine(simplex[2].point, simplex[0].point, simplex[1].point) == 0) {
		return false
	}
	recentStates := [][]scoredPoint{}
	for evaluator.canIterate() {
		sortSimplex(simplex)
		for _, previousSimplex := range recentStates {
			if slices.Equal(simplex, previousSimplex) {
				return false
			}
		}
		recentStates = append(recentStates, slices.Clone(simplex))
		if len(recentStates) > 32 {
			recentStates = recentStates[1:]
		}
		previousSimplex := slices.Clone(simplex)
		evaluator.beginStage()
		simplex = nelderMeadIteration(evaluator, simplex, project)
		evaluator.result.Iterations++
		evaluator.finishStage(simplexDiameter(simplex))
		sortSimplex(simplex)
		// The best vertex can stay fixed while the rest of the simplex still moves.
		change, valid := changesBetween(previousSimplex, simplex)
		if !evaluator.shouldStop() && valid && change.meets(evaluator.params) {
			evaluator.recordConvergence(change)
			return true
		}
	}
	return false
}

func (NelderMead) Run(ctx context.Context, problem *model.Problem, parameters Parameters) (Result, error) {
	parameters = parameters.Normalized()
	if err := parameters.Validate("nelder_mead"); err != nil {
		return Result{}, err
	}
	if parameters.Start != nil && !problem.Domain.Contains(*parameters.Start) {
		return Result{}, fmt.Errorf("Начальная точка находится вне допустимой области")
	}
	evaluator := newEvaluation(ctx, problem, parameters, "nelder_mead")
	evaluator.source = "nelder_mead"
	if evaluator.shouldStop() {
		return evaluator.complete()
	}
	vertices := problem.Domain.Vertices()
	if len(vertices) == 0 {
		evaluator.stop("no_feasible_region", "Допустимых решений нет")
		return evaluator.complete()
	}
	if len(vertices) == 1 {
		evaluator.beginStage()
		evaluator.score(vertices[0])
		evaluator.finishStage(0)
		if !evaluator.shouldStop() {
			evaluator.stop("converged", "Допустимая область состоит из одной точки")
		}
		return evaluator.complete()
	}
	start := constraints.Point{}
	for _, vertex := range vertices {
		start[0] += vertex[0] / float64(len(vertices))
		start[1] += vertex[1] / float64(len(vertices))
	}
	if parameters.Start != nil {
		start = *parameters.Start
	}
	start = problem.Domain.Project(start)
	simplex := initialSimplex(problem.Domain, vertices, start, parameters.InitialStep)
	var project projector
	if len(vertices) == 2 {
		project = problem.Domain.Project
	}
	converged := minimizeSimplex(evaluator, simplex, project)
	if evaluator.canIterate() {
		if converged {
			evaluator.stop("converged", convergenceReason(parameters))
		} else {
			evaluator.stop("stalled", "Симплексный поиск остановился до выполнения выбранного критерия")
		}
	}
	return evaluator.complete()
}

func initialSimplex(domain constraints.Domain, vertices []constraints.Point, start constraints.Point, step float64) []constraints.Point {
	if len(vertices) > 2 {
		first := domain.Project(constraints.Point{start[0] + step, start[1]})
		second := domain.Project(constraints.Point{start[0], start[1] + step})
		if distanceFromLine(second, start, first) > 0 {
			return []constraints.Point{start, first, second}
		}
	}

	// A start on the boundary can collapse the axis-aligned triangle after projection.
	distantVertex := vertices[0]
	for _, vertex := range vertices[1:] {
		if distance(start, vertex) > distance(start, distantVertex) {
			distantVertex = vertex
		}
	}
	first := domain.Project(towards(start, distantVertex, step))
	simplex := []constraints.Point{start, first}
	if len(vertices) == 2 {
		return simplex
	}
	wideVertex := vertices[0]
	for _, vertex := range vertices[1:] {
		if distanceFromLine(vertex, start, first) > distanceFromLine(wideVertex, start, first) {
			wideVertex = vertex
		}
	}
	second := domain.Project(towards(start, wideVertex, step))
	return append(simplex, second)
}
