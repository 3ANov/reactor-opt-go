package optimization

import (
	"context"
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

// Infeasible 2D trials use a barrier; edge trials are projected onto their segment.
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
	evaluator.beginStage()
	for _, vertex := range vertices {
		if _, ok := evaluator.score(vertex); !ok {
			break
		}
	}
	evaluator.finishStage(parameters.InitialStep)
	if evaluator.shouldStop() {
		return evaluator.complete()
	}
	if len(vertices) == 1 {
		evaluator.stop("converged", "Допустимая область состоит из одной точки")
		return evaluator.complete()
	}
	project := problem.Domain.Project
	center := constraints.Point{}
	for _, vertex := range vertices {
		center[0] += vertex[0] / float64(len(vertices))
		center[1] += vertex[1] / float64(len(vertices))
	}
	allConverged := true
	if len(vertices) > 2 {
		wideVertex := vertices[1]
		for _, vertex := range vertices[2:] {
			if math.Abs(cross(center, vertices[0], vertex)) > math.Abs(cross(center, vertices[0], wideVertex)) {
				wideVertex = vertex
			}
		}
		initialSimplex := []constraints.Point{
			project(center),
			project(towards(center, vertices[0], parameters.InitialStep)),
			project(towards(center, wideVertex, parameters.InitialStep)),
		}
		if !minimizeSimplex(evaluator, initialSimplex, nil) {
			allConverged = false
		}
		for _, vertex := range vertices {
			if !evaluator.canIterate() {
				break
			}
			distantVertex := vertices[0]
			for _, candidate := range vertices {
				if distance(vertex, candidate) > distance(vertex, distantVertex) {
					distantVertex = candidate
				}
			}
			wideVertex = vertices[0]
			for _, candidate := range vertices {
				if math.Abs(cross(vertex, distantVertex, candidate)) > math.Abs(cross(vertex, distantVertex, wideVertex)) {
					wideVertex = candidate
				}
			}
			initialSimplex = []constraints.Point{
				vertex,
				project(towards(vertex, distantVertex, parameters.InitialStep)),
				project(towards(vertex, wideVertex, parameters.InitialStep)),
			}
			if !minimizeSimplex(evaluator, initialSimplex, nil) {
				allConverged = false
			}
		}
	}
	edgeCount := len(vertices)
	if edgeCount == 2 {
		edgeCount = 1
	}
	for i := 0; i < edgeCount; i++ {
		if !evaluator.canIterate() {
			break
		}
		edgeStart, edgeEnd := vertices[i], vertices[(i+1)%len(vertices)]
		midpoint := interpolate(edgeStart, edgeEnd, 0.5)
		projectToEdge := func(candidate constraints.Point) constraints.Point {
			return project(ontoSegment(candidate, edgeStart, edgeEnd))
		}
		initialSegment := []constraints.Point{
			midpoint,
			towards(midpoint, edgeEnd, parameters.InitialStep),
		}
		if !minimizeSimplex(evaluator, initialSegment, projectToEdge) {
			allConverged = false
		}
	}
	if evaluator.canIterate() {
		if allConverged {
			evaluator.stop("converged", convergenceReason(parameters))
		} else {
			evaluator.stop("stalled", "Симплексный поиск остановился до выполнения выбранного критерия")
		}
	}
	return evaluator.complete()
}
