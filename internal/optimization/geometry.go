package optimization

import (
	"math"

	"reactoropt/internal/constraints"
)

func distance(first, second constraints.Point) float64 {
	return math.Hypot(first[0]-second[0], first[1]-second[1])
}

// Negative fractions move away from end, as used in reflection and expansion.
func interpolate(start, end constraints.Point, fraction float64) constraints.Point {
	return constraints.Point{start[0] + fraction*(end[0]-start[0]), start[1] + fraction*(end[1]-start[1])}
}

func towards(start, target constraints.Point, step float64) constraints.Point {
	length := distance(start, target)
	if length == 0 {
		return start
	}
	return interpolate(start, target, math.Min(1, step/length))
}

func cross(origin, first, second constraints.Point) float64 {
	return (first[0]-origin[0])*(second[1]-origin[1]) - (first[1]-origin[1])*(second[0]-origin[0])
}

func ontoSegment(point, start, end constraints.Point) constraints.Point {
	length := distance(start, end)
	if length == 0 {
		return start
	}
	dx, dy := (end[0]-start[0])/length, (end[1]-start[1])/length
	fraction := ((point[0]-start[0])*dx + (point[1]-start[1])*dy) / length
	if fraction <= 0 {
		return start
	}
	if fraction >= 1 {
		return end
	}
	return interpolate(start, end, fraction)
}
