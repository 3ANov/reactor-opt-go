package app

import (
	"context"
	"fmt"

	"reactoropt/internal/constraints"
)

type PlotData struct {
	X         []float64           `json:"x"`
	Y         []float64           `json:"y"`
	Z         [][]*float64        `json:"z"`
	FeasibleZ [][]*float64        `json:"feasible_z"`
	Polygon   []constraints.Point `json:"polygon"`
}

func (service *Service) Sample(ctx context.Context, taskID string, resolution int) (PlotData, error) {
	service.mu.Lock()
	if _, err := service.current(); err != nil {
		service.mu.Unlock()
		return PlotData{}, err
	}
	task, err := service.store.Task(taskID)
	service.mu.Unlock()
	if err != nil {
		return PlotData{}, err
	}
	if resolution < 11 || resolution > 201 {
		return PlotData{}, fmt.Errorf("Разрешение графика должно быть от 11 до 201")
	}
	problem, err := task.Compile()
	if err != nil {
		return PlotData{}, err
	}
	plot := PlotData{
		Polygon:   problem.Domain.Vertices(),
		X:         make([]float64, resolution),
		Y:         make([]float64, resolution),
		Z:         make([][]*float64, resolution),
		FeasibleZ: make([][]*float64, resolution),
	}
	for i := 0; i < resolution; i++ {
		fraction := float64(i) / float64(resolution-1)
		plot.X[i] = task.Variables[0].Min + fraction*(task.Variables[0].Max-task.Variables[0].Min)
		plot.Y[i] = task.Variables[1].Min + fraction*(task.Variables[1].Max-task.Variables[1].Min)
	}
	for row, y := range plot.Y {
		if ctx.Err() != nil {
			return PlotData{}, fmt.Errorf("Построение графика отменено")
		}
		plot.Z[row] = make([]*float64, resolution)
		plot.FeasibleZ[row] = make([]*float64, resolution)
		for column, x := range plot.X {
			point := constraints.Point{x, y}
			value, err := problem.Objective(point)
			if err != nil {
				continue
			}
			plot.Z[row][column] = &value
			if problem.Domain.Contains(point) {
				plot.FeasibleZ[row][column] = &value
			}
		}
	}
	return plot, nil
}
