package model

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"reactoropt/internal/constraints"
	"reactoropt/internal/expressions"
)

var metricID = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$`)

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func copyValues(source map[string]float64) map[string]float64 {
	values := make(map[string]float64, len(source)+2)
	maps.Copy(values, source)
	return values
}

func validateParameters(values map[string]float64, usedNames map[string]bool) ([]string, error) {
	if len(values) > 32 {
		return nil, fmt.Errorf("Допустимо до 32 параметров")
	}
	var names []string
	for name, value := range values {
		if !expressions.ValidName(name) || usedNames[name] {
			return nil, fmt.Errorf("Недопустимое или повторяющееся имя: %s", name)
		}
		if !finite(value) || math.Abs(value) > 1e12 {
			return nil, fmt.Errorf("Параметры должны быть конечными по модулю до 10¹²")
		}
		usedNames[name] = true
		names = append(names, name)
	}
	return names, nil
}

func (task Task) Compile() (*Problem, error) {
	if len(task.Variables) != 2 {
		return nil, fmt.Errorf("В задаче должны быть ровно две переменные")
	}
	if task.Direction != "min" && task.Direction != "max" {
		return nil, fmt.Errorf("Направление поиска: min или max")
	}
	problem := &Problem{Task: task}
	usedNames := map[string]bool{}
	variableNames := []string{}
	for index, variable := range task.Variables {
		if !expressions.ValidName(variable.Name) || usedNames[variable.Name] {
			return nil, fmt.Errorf("Недопустимое или повторяющееся имя переменной")
		}
		if !finite(variable.Min) || !finite(variable.Max) || variable.Min > variable.Max ||
			math.Abs(variable.Min) > 1e9 || math.Abs(variable.Max) > 1e9 {
			return nil, fmt.Errorf("Нужны конечные упорядоченные границы по модулю до 10⁹")
		}
		usedNames[variable.Name] = true
		variableNames = append(variableNames, variable.Name)
		problem.Domain.Bounds[index] = [2]float64{variable.Min, variable.Max}
	}
	parameterNames, err := validateParameters(task.Coefficients, usedNames)
	if err != nil {
		return nil, err
	}
	formula, err := expressions.Compile(task.Objective, append(variableNames, parameterNames...))
	if err != nil {
		return nil, fmt.Errorf("Целевая функция: %w", err)
	}
	problem.Domain.Inequalities, err = task.compileConstraints()
	if err != nil {
		return nil, err
	}
	if err := problem.Domain.Validate(); err != nil {
		return nil, err
	}
	parameterValues := copyValues(task.Coefficients)
	xName, yName := task.Variables[0].Name, task.Variables[1].Name
	problem.Objective = func(point constraints.Point) (float64, error) {
		values := copyValues(parameterValues)
		values[xName], values[yName] = point[0], point[1]
		return formula.Eval(values)
	}
	problem.metrics, err = task.compileMetrics()
	if err != nil {
		return nil, err
	}
	return problem, nil
}

func (task Task) compileConstraints() ([][3]float64, error) {
	if len(task.Constraints) > 64 {
		return nil, fmt.Errorf("Допустимо до 64 ограничений")
	}
	var inequalities [][3]float64
	for _, constraint := range task.Constraints {
		if len(constraint.Coefficients) != 2 {
			return nil, fmt.Errorf("В ограничении должны быть коэффициенты обеих переменных")
		}
		coefficients := [3]float64{0, 0, constraint.Value}
		for name, value := range constraint.Coefficients {
			switch name {
			case task.Variables[0].Name:
				coefficients[0] = value
			case task.Variables[1].Name:
				coefficients[1] = value
			default:
				return nil, fmt.Errorf("Неизвестная переменная ограничения")
			}
		}
		for _, value := range coefficients {
			if !finite(value) || math.Abs(value) > 1e12 {
				return nil, fmt.Errorf("Коэффициенты ограничений должны быть конечными по модулю до 10¹²")
			}
		}
		switch constraint.Operator {
		case "<=":
			inequalities = append(inequalities, coefficients)
		case ">=":
			inequalities = append(inequalities, [3]float64{-coefficients[0], -coefficients[1], -coefficients[2]})
		case "=":
			inequalities = append(inequalities, coefficients, [3]float64{-coefficients[0], -coefficients[1], -coefficients[2]})
		default:
			return nil, fmt.Errorf("Разрешены сравнения <=, >= и =")
		}
	}
	if len(inequalities) > 64 {
		return nil, fmt.Errorf("Равенство занимает два неравенства; допустимо до 64")
	}
	return inequalities, nil
}

func (task Task) compileMetrics() ([]compiledMetric, error) {
	if len(task.DerivedMetrics) > 16 {
		return nil, fmt.Errorf("Допустимо до 16 показателей")
	}
	var metrics []compiledMetric
	metricIDs := map[string]bool{}
	for _, metric := range task.DerivedMetrics {
		if !metricID.MatchString(metric.ID) || metricIDs[metric.ID] || strings.TrimSpace(metric.Name) == "" ||
			utf8.RuneCountInString(metric.Name) > 200 || utf8.RuneCountInString(metric.Unit) > 32 ||
			utf8.RuneCountInString(metric.Symbol) > 32 {
			return nil, fmt.Errorf("Проверьте идентификаторы, названия и единицы показателей")
		}
		metricIDs[metric.ID] = true
		parameterNames, err := validateParameters(metric.Parameters, map[string]bool{"C": true})
		if err != nil {
			return nil, err
		}
		expression, err := expressions.Compile(metric.Expression, append([]string{"C"}, parameterNames...))
		if err != nil {
			return nil, fmt.Errorf("Показатель %s: %w", metric.Name, err)
		}
		metric.Parameters = copyValues(metric.Parameters)
		metrics = append(metrics, compiledMetric{definition: metric, expression: expression})
	}
	return metrics, nil
}

func (problem *Problem) Metrics(value float64) ([]MetricValue, error) {
	metrics := []MetricValue{}
	for _, metric := range problem.metrics {
		values := copyValues(metric.definition.Parameters)
		values["C"] = value
		metricValue, err := metric.expression.Eval(values)
		if err != nil {
			return nil, fmt.Errorf("Показатель %s: %w", metric.definition.Name, err)
		}
		definition := metric.definition
		metrics = append(metrics, MetricValue{
			ID:     definition.ID,
			Name:   definition.Name,
			Value:  metricValue,
			Unit:   definition.Unit,
			Symbol: definition.Symbol,
		})
	}
	return metrics, nil
}
