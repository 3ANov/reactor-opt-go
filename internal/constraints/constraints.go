package constraints

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

type Point [2]float64
type Domain struct {
	Bounds       [2][2]float64
	Inequalities [][3]float64
}

var ErrEmptyDomain = errors.New("Допустимая область пуста")

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func (domain Domain) validateValues() error {
	for _, bounds := range domain.Bounds {
		if !finite(bounds[0]) || !finite(bounds[1]) || bounds[0] > bounds[1] {
			return fmt.Errorf("Некорректные границы переменных")
		}
	}
	for _, coefficients := range domain.Inequalities {
		for _, value := range coefficients {
			if !finite(value) {
				return fmt.Errorf("Коэффициенты ограничений должны быть конечными")
			}
		}
	}
	return nil
}

func (domain Domain) Validate() error {
	if err := domain.validateValues(); err != nil {
		return err
	}
	if len(domain.Vertices()) == 0 {
		return ErrEmptyDomain
	}
	return nil
}

func normalize(coefficients [3]float64) [3]float64 {
	scale := math.Max(math.Abs(coefficients[0]), math.Abs(coefficients[1]))
	if scale == 0 {
		if coefficients[2] == 0 {
			return coefficients
		}
		return [3]float64{0, 0, math.Copysign(1, coefficients[2])}
	}
	if !finite(coefficients[2] / scale) {
		return coefficients
	}
	return [3]float64{coefficients[0] / scale, coefficients[1] / scale, coefficients[2] / scale}
}

func (domain Domain) Contains(point Point) bool {
	for axis, coordinate := range point {
		if !finite(coordinate) || coordinate < domain.Bounds[axis][0] || coordinate > domain.Bounds[axis][1] {
			return false
		}
	}
	for _, original := range domain.Inequalities {
		coefficients := normalize(original)
		ax, by := coefficients[0]*point[0], coefficients[1]*point[1]
		tolerance := 1e-12 * math.Max(math.Abs(coefficients[2]), math.Max(math.Abs(ax), math.Abs(by)))
		if !finite(ax+by) || ax+by > coefficients[2]+tolerance {
			return false
		}
	}
	return true
}

func (domain Domain) Vertices() []Point {
	if domain.validateValues() != nil {
		return nil
	}
	boundaries := make([][3]float64, 0, len(domain.Inequalities)+4)
	for _, coefficients := range domain.Inequalities {
		boundaries = append(boundaries, normalize(coefficients))
	}
	boundaries = append(boundaries,
		[3]float64{-1, 0, -domain.Bounds[0][0]},
		[3]float64{1, 0, domain.Bounds[0][1]},
		[3]float64{0, -1, -domain.Bounds[1][0]},
		[3]float64{0, 1, domain.Bounds[1][1]},
	)
	var points []Point
	for i, first := range boundaries {
		for _, second := range boundaries[i+1:] {
			determinant := first[0]*second[1] - first[1]*second[0]
			if determinant == 0 {
				continue
			}
			x := (first[2]*second[1] - first[1]*second[2]) / determinant
			y := (first[0]*second[2] - first[2]*second[0]) / determinant
			point := Point{
				math.Max(domain.Bounds[0][0], math.Min(domain.Bounds[0][1], x)),
				math.Max(domain.Bounds[1][0], math.Min(domain.Bounds[1][1], y)),
			}
			if math.Hypot(point[0]-x, point[1]-y) <= 1e-10 && domain.Contains(point) {
				points = append(points, point)
			}
		}
	}
	return convexHull(points)
}

func convexHull(points []Point) []Point {
	sort.Slice(points, func(i, j int) bool {
		return points[i][0] < points[j][0] || points[i][0] == points[j][0] && points[i][1] < points[j][1]
	})
	uniqueVertices := points[:0]
	for _, point := range points {
		if len(uniqueVertices) == 0 || uniqueVertices[len(uniqueVertices)-1] != point {
			uniqueVertices = append(uniqueVertices, point)
		}
	}
	if len(uniqueVertices) <= 2 {
		return uniqueVertices
	}
	var lower, upper []Point
	for _, point := range uniqueVertices {
		for len(lower) >= 2 && orientation(lower[len(lower)-2], lower[len(lower)-1], point) <= 0 {
			lower = lower[:len(lower)-1]
		}
		lower = append(lower, point)
	}
	for i := len(uniqueVertices) - 1; i >= 0; i-- {
		point := uniqueVertices[i]
		for len(upper) >= 2 && orientation(upper[len(upper)-2], upper[len(upper)-1], point) <= 0 {
			upper = upper[:len(upper)-1]
		}
		upper = append(upper, point)
	}
	return append(lower[:len(lower)-1], upper[:len(upper)-1]...)
}

func orientation(origin, first, second Point) float64 {
	dx, dy := first[0]-origin[0], first[1]-origin[1]
	ex, ey := second[0]-origin[0], second[1]-origin[1]
	value := dx*ey - dy*ex
	// Rescale only underflowed orientation tests to recover the cross-product sign.
	if value == 0 {
		scale := math.Max(math.Max(math.Abs(dx), math.Abs(dy)), math.Max(math.Abs(ex), math.Abs(ey)))
		if scale > 0 && scale < 1e-150 {
			return (dx/scale)*(ey/scale) - (dy/scale)*(ex/scale)
		}
	}
	return value
}

func (domain Domain) Project(point Point) Point {
	if domain.Contains(point) {
		return point
	}
	vertices := domain.Vertices()
	if len(vertices) == 0 {
		return Point{math.NaN(), math.NaN()}
	}
	best := vertices[0]
	bestDistance := math.Inf(1)
	for i, edgeStart := range vertices {
		edgeEnd := vertices[(i+1)%len(vertices)]
		length := math.Hypot(edgeEnd[0]-edgeStart[0], edgeEnd[1]-edgeStart[1])
		fraction := 0.0
		if length > 0 {
			dx, dy := (edgeEnd[0]-edgeStart[0])/length, (edgeEnd[1]-edgeStart[1])/length
			fraction = ((point[0]-edgeStart[0])*dx + (point[1]-edgeStart[1])*dy) / length
		}
		fraction = math.Max(0, math.Min(1, fraction))
		candidate := Point{edgeStart[0] + fraction*(edgeEnd[0]-edgeStart[0]), edgeStart[1] + fraction*(edgeEnd[1]-edgeStart[1])}
		candidateDistance := math.Hypot(candidate[0]-point[0], candidate[1]-point[1])
		if candidateDistance < bestDistance {
			best, bestDistance = candidate, candidateDistance
		}
	}
	for i := range best {
		best[i] = math.Max(domain.Bounds[i][0], math.Min(domain.Bounds[i][1], best[i]))
	}
	if domain.Contains(best) {
		return best
	}
	best = vertices[0]
	for _, vertex := range vertices {
		if math.Hypot(vertex[0]-point[0], vertex[1]-point[1]) < math.Hypot(best[0]-point[0], best[1]-point[1]) {
			best = vertex
		}
	}
	return best
}
