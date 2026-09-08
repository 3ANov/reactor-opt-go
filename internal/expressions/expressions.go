package expressions

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/parser/lexer"
)

var functions = map[string]int{
	"sin": 1, "cos": 1, "tan": 1, "sqrt": 1, "exp": 1,
	"log": 1, "log10": 1, "abs": 1, "min": 2, "max": 2,
}

var identifierPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,31}$`)

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func ValidName(name string) bool {
	if !identifierPattern.MatchString(name) || functions[name] != 0 {
		return false
	}
	switch name {
	case "pi", "e", "True", "False", "None":
		return false
	}
	return true
}

type node struct {
	kind, name string
	value      float64
	children   []*node
	hasPower   bool
}

type Expression struct{ root *node }

type validator struct {
	names map[string]bool
	nodes int
}

func Compile(source string, names []string) (*Expression, error) {
	if len(source) == 0 || len(source) > 2048 {
		return nil, fmt.Errorf("Формула должна содержать от 1 до 2048 символов")
	}
	check := validator{names: make(map[string]bool, len(names))}
	for _, name := range names {
		if !ValidName(name) || check.names[name] {
			return nil, fmt.Errorf("Недопустимое или повторяющееся имя: %s", name)
		}
		check.names[name] = true
	}
	normalized, err := normalizeSource(source)
	if err != nil {
		return nil, err
	}
	tree, err := parser.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("Неверная формула: %w", err)
	}
	root, err := check.build(tree.Node, 1)
	if err != nil {
		return nil, err
	}
	return &Expression{root: root}, nil
}

func normalizeSource(source string) (string, error) {
	if strings.Contains(source, "//") || strings.Contains(source, "/*") {
		return "", fmt.Errorf("Комментарии в формулах запрещены")
	}
	tokens, err := lexer.Lex(file.NewSource(source))
	if err != nil {
		return "", fmt.Errorf("Неверная формула: %w", err)
	}
	var normalized strings.Builder
	depth := 0
	for _, token := range tokens {
		text := token.Value
		switch token.Kind {
		case lexer.EOF:
			continue
		case lexer.Identifier:
		case lexer.Number:
			value, err := strconv.ParseFloat(text, 64)
			if err != nil || strings.ContainsAny(text, "_xXoObB") || !finite(value) || math.Abs(value) > 1e100 {
				return "", fmt.Errorf("Недопустимая числовая константа: %s", text)
			}
			// Expr uses integers by default; task formulas use float64 throughout.
			text = strconv.FormatFloat(value, 'g', -1, 64)
			if !strings.ContainsAny(text, ".eE") {
				text += ".0"
			}
		case lexer.Operator, lexer.Bracket:
			switch text {
			case "+", "-", "*", "/", "**", ",":
			case "(":
				depth++
				if depth > 128 {
					return "", fmt.Errorf("Слишком глубокая формула")
				}
			case ")":
				depth--
			default:
				return "", fmt.Errorf("Недопустимая операция: %s", text)
			}
		default:
			return "", fmt.Errorf("Разрешены только числа, имена и математические операции")
		}
		normalized.WriteString(text)
		normalized.WriteByte(' ')
	}
	return normalized.String(), nil
}

// Only allowlisted numeric nodes reach our evaluator; Expr's VM is never used.
func (check *validator) build(parsed ast.Node, depth int) (*node, error) {
	check.nodes++
	if check.nodes > 128 {
		return nil, fmt.Errorf("Формула содержит более 128 узлов")
	}
	if depth > 25 {
		return nil, fmt.Errorf("Глубина формулы превышает 24 уровня")
	}

	result := &node{}
	var children []ast.Node
	switch parsed := parsed.(type) {
	case *ast.FloatNode:
		result.kind, result.value = "number", parsed.Value
	case *ast.IdentifierNode:
		result.kind, result.name = "name", parsed.Value
		switch parsed.Value {
		case "pi":
			result.kind, result.value = "number", math.Pi
		case "e":
			result.kind, result.value = "number", math.E
		default:
			if !check.names[parsed.Value] {
				return nil, fmt.Errorf("Неизвестное имя: %s", parsed.Value)
			}
		}
	case *ast.UnaryNode:
		if parsed.Operator != "+" && parsed.Operator != "-" {
			return nil, fmt.Errorf("Недопустимая унарная операция")
		}
		result.kind = "u" + parsed.Operator
		children = []ast.Node{parsed.Node}
	case *ast.BinaryNode:
		switch parsed.Operator {
		case "+", "-", "*", "/", "**":
		default:
			return nil, fmt.Errorf("Недопустимая операция: %s", parsed.Operator)
		}
		result.kind = parsed.Operator
		children = []ast.Node{parsed.Left, parsed.Right}
	case *ast.CallNode:
		function, ok := parsed.Callee.(*ast.IdentifierNode)
		if !ok {
			return nil, fmt.Errorf("Недопустимый вызов функции")
		}
		result.kind, result.name = "call", function.Value
		children = parsed.Arguments
	case *ast.BuiltinNode:
		result.kind, result.name = "call", parsed.Name
		children = parsed.Arguments
	default:
		return nil, fmt.Errorf("Неподдерживаемая конструкция формулы")
	}

	if result.kind == "call" {
		arity, ok := functions[result.name]
		if !ok {
			return nil, fmt.Errorf("Неизвестная функция: %s", result.name)
		}
		if len(children) != arity {
			return nil, fmt.Errorf("Функция %s требует %d аргументов", result.name, arity)
		}
	}
	result.hasPower = result.kind == "**"
	for _, child := range children {
		checked, err := check.build(child, depth+1)
		if err != nil {
			return nil, err
		}
		result.children = append(result.children, checked)
		result.hasPower = result.hasPower || checked.hasPower
	}
	if result.kind == "**" && result.children[1].hasPower {
		return nil, fmt.Errorf("Вложенная степень в показателе запрещена")
	}
	return result, nil
}

func (expression *Expression) Eval(values map[string]float64) (float64, error) {
	value, err := evaluate(expression.root, values)
	if err == nil && math.Abs(value) > 1e100 {
		return 0, fmt.Errorf("Величина результата превышает 10¹⁰⁰")
	}
	return value, err
}

func evaluate(currentNode *node, values map[string]float64) (float64, error) {
	var left, right float64
	var err error
	if len(currentNode.children) > 0 {
		left, err = evaluate(currentNode.children[0], values)
		if err != nil {
			return 0, err
		}
	}
	if len(currentNode.children) > 1 {
		right, err = evaluate(currentNode.children[1], values)
		if err != nil {
			return 0, err
		}
	}
	value := 0.0
	switch currentNode.kind {
	case "number":
		value = currentNode.value
	case "name":
		var ok bool
		value, ok = values[currentNode.name]
		if !ok {
			return 0, fmt.Errorf("Не задано значение %s", currentNode.name)
		}
	case "+":
		value = left + right
	case "-":
		value = left - right
	case "*":
		value = left * right
	case "/":
		if right == 0 {
			return 0, fmt.Errorf("Деление на ноль")
		}
		value = left / right
	case "u+":
		value = left
	case "u-":
		value = -left
	case "**":
		if math.Abs(right) > 100 {
			return 0, fmt.Errorf("Модуль показателя степени превышает 100")
		}
		value = math.Pow(left, right)
	case "call":
		switch currentNode.name {
		case "sin":
			value = math.Sin(left)
		case "cos":
			value = math.Cos(left)
		case "tan":
			value = math.Tan(left)
		case "sqrt":
			value = math.Sqrt(left)
		case "exp":
			value = math.Exp(left)
		case "log":
			value = math.Log(left)
		case "log10":
			value = math.Log10(left)
		case "abs":
			value = math.Abs(left)
		case "min":
			value = math.Min(left, right)
		case "max":
			value = math.Max(left, right)
		}
	default:
		return 0, fmt.Errorf("Неизвестная операция")
	}
	if !finite(value) {
		return 0, fmt.Errorf("Формула дала неопределённое или бесконечное значение")
	}
	return value, nil
}
