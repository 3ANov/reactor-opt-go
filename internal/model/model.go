package model

import (
	"reactoropt/internal/constraints"
	"reactoropt/internal/expressions"
)

type Variable struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Unit        string  `json:"unit"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
}
type LinearConstraint struct {
	Coefficients map[string]float64 `json:"coefficients"`
	Operator     string             `json:"operator"`
	Value        float64            `json:"value"`
}
type Metric struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Expression string             `json:"expression"`
	Unit       string             `json:"unit"`
	Parameters map[string]float64 `json:"parameters"`
	Symbol     string             `json:"symbol"`
}
type MetricValue struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Symbol string  `json:"symbol"`
}
type Task struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Objective      string             `json:"objective"`
	Direction      string             `json:"direction"`
	Variables      []Variable         `json:"variables"`
	Coefficients   map[string]float64 `json:"coefficients"`
	Constraints    []LinearConstraint `json:"constraints"`
	DerivedMetrics []Metric           `json:"derived_metrics"`
	Builtin        bool               `json:"builtin"`
	ObjectiveLabel string             `json:"objective_label"`
	ObjectiveUnit  string             `json:"objective_unit"`
}
type compiledMetric struct {
	definition Metric
	expression *expressions.Expression
}
type Problem struct {
	Task      Task
	Domain    constraints.Domain
	Objective func(constraints.Point) (float64, error)
	metrics   []compiledMetric
}
