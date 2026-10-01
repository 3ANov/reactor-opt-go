package optimization

import (
	"math"

	"reactoropt/internal/constraints"
)

type parameterization struct {
	points []constraints.Point
	source string
}

func logistic(coordinate float64) float64 {
	if coordinate >= 0 {
		return 1 / (1 + math.Exp(-coordinate))
	}
	exponential := math.Exp(coordinate)
	return exponential / (1 + exponential)
}

func logit(fraction float64) float64 {
	fraction = math.Max(1e-6, math.Min(1-1e-6, fraction))
	return math.Log(fraction / (1 - fraction))
}

// Logistic coordinates keep probes inside a feasible triangle or segment.
// Vertices and boundary segments are checked separately.
func (region parameterization) point(coordinates []float64) constraints.Point {
	fraction := logistic(coordinates[0])
	if len(region.points) == 2 {
		return interpolate(region.points[0], region.points[1], fraction)
	}
	edgeFraction := logistic(coordinates[1])
	pointOnEdge := interpolate(region.points[1], region.points[2], edgeFraction)
	return interpolate(region.points[0], pointOnEdge, fraction)
}

func (region parameterization) initial(start *constraints.Point) []float64 {
	if len(region.points) == 2 {
		fraction := 0.5
		if start != nil {
			first, second := region.points[0], region.points[1]
			projected := ontoSegment(*start, first, second)
			length := distance(first, second)
			if length > 0 {
				fraction = distance(first, projected) / length
			}
		}
		return []float64{logit(fraction)}
	}
	fraction, edgeFraction := region.triangleFractions(start)
	return []float64{logit(fraction), logit(edgeFraction)}
}

func (region parameterization) triangleFractions(start *constraints.Point) (float64, float64) {
	if start == nil {
		return 2.0 / 3, 0.5
	}
	first, second, third := region.points[0], region.points[1], region.points[2]
	determinant := cross(first, second, third)
	if determinant == 0 {
		return 2.0 / 3, 0.5
	}
	secondWeight := cross(first, *start, third) / determinant
	thirdWeight := cross(first, second, *start) / determinant
	if !(secondWeight >= 0 && thirdWeight >= 0 && secondWeight+thirdWeight <= 1) {
		return 2.0 / 3, 0.5
	}
	fraction := secondWeight + thirdWeight
	if fraction > 0 {
		return fraction, thirdWeight / fraction
	}
	return fraction, 0.5
}

func searchRegions(vertices []constraints.Point, method string) []parameterization {
	regions := []parameterization{}
	for index := 1; index+1 < len(vertices); index++ {
		regions = append(regions, parameterization{
			points: []constraints.Point{vertices[0], vertices[index], vertices[index+1]},
			source: method + "_interior",
		})
	}
	edges := len(vertices)
	if edges == 2 {
		edges = 1
	}
	for index := 0; index < edges; index++ {
		regions = append(regions, parameterization{
			points: []constraints.Point{vertices[index], vertices[(index+1)%len(vertices)]},
			source: method + "_boundary",
		})
	}
	return regions
}
