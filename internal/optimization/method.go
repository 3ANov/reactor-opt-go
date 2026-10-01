package optimization

import (
	"context"
	"fmt"
	"math"

	"reactoropt/internal/constraints"
	"reactoropt/internal/model"
)

type Parameters struct {
	Precision         float64            `json:"precision"`
	InitialStep       float64            `json:"initial_step"`
	MaxEvaluations    int                `json:"max_evaluations"`
	KeepHistory       bool               `json:"keep_history"`
	FunctionPrecision float64            `json:"function_precision"`
	StoppingCriterion string             `json:"stopping_criterion"`
	GradientStep      float64            `json:"gradient_step"`
	GradientTolerance float64            `json:"gradient_tolerance"`
	MaxIterations     int                `json:"max_iterations"`
	Population        int                `json:"population"`
	Seed              uint64             `json:"seed"`
	Start             *constraints.Point `json:"start"`
}

func DefaultParameters() Parameters {
	return Parameters{
		Precision:         0.01,
		InitialStep:       1,
		MaxEvaluations:    10000,
		FunctionPrecision: 1e-6,
		StoppingCriterion: "coordinates",
		GradientStep:      6e-6,
		GradientTolerance: 1e-7,
		MaxIterations:     2000,
		Population:        12,
		Seed:              42,
	}
}

func (parameters Parameters) Validate(method string) error {
	parameters = parameters.Normalized()
	positive := func(value float64) bool { return value > 0 && !math.IsInf(value, 0) && !math.IsNaN(value) }
	if parameters.MaxEvaluations < 1 || parameters.MaxEvaluations > 10000000 {
		return fmt.Errorf("Лимит вычислений должен быть от 1 до 10 000 000")
	}
	if parameters.MaxIterations < 1 || parameters.MaxIterations > 1000000 {
		return fmt.Errorf("Лимит итераций должен быть от 1 до 1 000 000")
	}
	switch parameters.StoppingCriterion {
	case "evaluations", "coordinates", "function":
	default:
		return fmt.Errorf("Неизвестный критерий остановки")
	}
	if !positive(parameters.InitialStep) {
		return fmt.Errorf("Начальный шаг должен быть конечным и положительным")
	}
	if !positive(parameters.Precision) || !positive(parameters.FunctionPrecision) {
		return fmt.Errorf("Точность должна быть конечной и положительной")
	}
	if method == "bfgs" && (!positive(parameters.GradientStep) || parameters.GradientStep > 0.1 ||
		!positive(parameters.GradientTolerance)) {
		return fmt.Errorf("Проверьте шаг и порог численного градиента")
	}
	if method == "cma_es" && (parameters.Population < 4 || parameters.Population > 1024) {
		return fmt.Errorf("Размер популяции должен быть от 4 до 1024")
	}
	if parameters.Start != nil {
		for _, value := range *parameters.Start {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("Начальная точка должна быть конечной")
			}
		}
	}
	return nil
}

type BestPoint struct {
	Point      constraints.Point `json:"point"`
	Value      float64           `json:"value"`
	Evaluation int               `json:"evaluation"`
	Stage      int               `json:"stage"`
}

type Stage struct {
	Number      int                `json:"number"`
	Step        float64            `json:"step"`
	Evaluations int                `json:"evaluations"`
	BestPoint   *constraints.Point `json:"best_point"`
	BestValue   *float64           `json:"best_value"`
}

type Result struct {
	Method               string              `json:"method"`
	Status               string              `json:"status"`
	Reason               string              `json:"reason"`
	Point                *constraints.Point  `json:"point"`
	Value                *float64            `json:"value"`
	Evaluations          int                 `json:"evaluations"`
	Iterations           int                 `json:"iterations"`
	FunctionChange       *float64            `json:"function_change"`
	CoordinateChange     *float64            `json:"coordinate_change"`
	Stages               int                 `json:"stages"`
	LibraryIterations    int                 `json:"library_iterations"`
	LibraryFunctionCalls int                 `json:"library_function_calls"`
	LibraryGradientCalls int                 `json:"library_gradient_calls"`
	LibraryStatus        string              `json:"library_status"`
	LibraryErrors        []string            `json:"library_errors"`
	Source               string              `json:"source"`
	History              []BestPoint         `json:"history"`
	StageHistory         []Stage             `json:"stage_history"`
	HistoryTruncated     bool                `json:"history_truncated"`
	Metrics              []model.MetricValue `json:"metrics"`
}

type Method interface {
	ID() string
	Run(context.Context, *model.Problem, Parameters) (Result, error)
}

type Definition struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Defaults    Parameters `json:"defaults"`
}

func Methods() []Definition {
	defaults := DefaultParameters()
	return []Definition{
		{
			ID:   "nelder_mead",
			Name: "Метод Нелдера — Мида",
			Description: "Локальный симплексный поиск из центра, вершин и вдоль рёбер. " +
				"Изменения функции и координат проверяются для всех вершин соседних симплексов. " +
				"Глобальная оптимальность не гарантируется.",
			Defaults: defaults,
		},
		{
			ID:   "bfgs",
			Name: "Метод BFGS",
			Description: "Локальный квазиньютоновский поиск с численным градиентом. " +
				"Допустимые треугольники и рёбра параметризуются; вершины проверяются отдельно. " +
				"Сходимость относится к локальным запускам.",
			Defaults: defaults,
		},
		{
			ID:   "cma_es",
			Name: "Метод CMA-ES",
			Description: "Стохастический популяционный поиск в допустимых треугольниках и на рёбрах. " +
				"Результат зависит от зерна; глобальная оптимальность не гарантируется.",
			Defaults: defaults,
		},
	}
}

func Select(id string) (Method, error) {
	switch id {
	case "nelder_mead":
		return NelderMead{}, nil
	case "bfgs":
		return BFGS{}, nil
	case "cma_es":
		return CMAES{}, nil
	default:
		return nil, fmt.Errorf("Неизвестный метод %q", id)
	}
}
