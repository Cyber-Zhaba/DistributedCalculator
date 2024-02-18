package agent

import (
	"DistributedCalculator/db"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

func isOperator(c rune) bool {
	return c == '+' || c == '-' || c == '*' || c == '/'
}

func priority(c rune) int {
	if c == '+' || c == '-' {
		return 1
	}
	if c == '*' || c == '/' {
		return 2
	}
	return 0
}

func PrepareEquation(equation string) string {
	equation = strings.ReplaceAll(equation, " ", "")
	equation = strings.ReplaceAll(equation, ",", ".")
	for {
		if equation[0] != '(' || equation[len(equation)-1] != ')' {
			return equation
		}
		parenthesis := 0
		for i := 0; i < len(equation); i++ {
			if equation[i] == '(' {
				parenthesis++
			} else if equation[i] == ')' {
				parenthesis--
				if parenthesis == 0 {
					if i == len(equation)-1 {
						equation = equation[1 : len(equation)-1]
					} else {
						return equation
					}
				}
			}
		}
	}
}

func ValidEquation(equation string, start, end int) bool {
	if end-start <= 0 {
		return false
	}
	// Remove all spaces from the equation.
	equation = strings.ReplaceAll(equation, " ", "")
	equation = strings.ReplaceAll(equation, ",", ".")
	end = min(end, len(equation))
	seenDot := false
	for i := start; i < end; i++ {
		if equation[i] == '(' {
			seenDot = false
			validInner := false
			parenthesis := 1
			for j := i + 1; j < end; j++ {
				if equation[j] == '(' {
					parenthesis++
				} else if equation[j] == ')' {
					parenthesis--
					if parenthesis == 0 {
						if !ValidEquation(equation, i+1, j) {
							return false
						}
						i = j + 1
						validInner = true
						break
					}
				}
			}
			if !validInner {
				return false
			}
		} else if equation[i] == ')' {
			return false
		} else if isOperator(rune(equation[i])) {
			seenDot = false
			if i == start {
				if equation[i] == '*' || equation[i] == '/' {
					return false
				}
			} else {
				if isOperator(rune(equation[i-1])) {
					return false
				}
			}
			if i == end-1 || isOperator(rune(equation[i+1])) {
				return false
			}
		} else if !('0' <= equation[i] && equation[i] <= '9') {
			if equation[i] == '.' && !seenDot {
				seenDot = true
			} else {
				return false
			}
		}
	}
	return true
}

// LastOperation returns the index of the last operator in the equation.
// | (1+2)+(3+4) | -1/35 | 1*2+3 | 1+2+3 |
// |      ^          ^        ^       ^
func LastOperation(equation string) int {
	equation = PrepareEquation(equation)
	lastOperator := -1
	operatorPriority := 2
	for i := 0; i < len(equation); i++ {
		c := rune(equation[i])
		if c == '(' {
			parenthesis := 1
			for j := i + 1; j < len(equation); j++ {
				if equation[j] == '(' {
					parenthesis++
				} else if equation[j] == ')' {
					parenthesis--
					if parenthesis == 0 {
						i = j
						break
					}
				}
			}
		} else if isOperator(c) && i != 0 {
			if priority(c) <= operatorPriority {
				lastOperator = i
				operatorPriority = priority(c)
			}
		}
	}
	return lastOperator
}
func Evaluate(equationID int) error {
	database, _ := db.Connect("data.db")
	defer database.Close()
	err := database.UpdateEquation(equationID, "Computing", 0)
	if err != nil {
		return err
	}
	mu := &sync.Mutex{}
	equation := database.GetEquation(equationID)
	var result float64
	result, err = evaluateRec(database, equationID, equation, mu)
	if err != nil {
		database.UpdateEquation(equationID, fmt.Sprintf("Error %s", err), 0)
		return err
	}
	err = database.UpdateEquation(equationID, "Computed", result)
	if err != nil {
		return err
	}
	return nil
}

func evaluateRec(database *db.DB, equationID int, equation string, mu *sync.Mutex) (float64, error) {
	var err error = nil
	equation = PrepareEquation(equation)
	lastOperator := LastOperation(equation)
	if lastOperator == -1 {
		value := 0.0
		value, err = strconv.ParseFloat(equation, 64)
		if err != nil {
			return 0, err
		}
		return value, nil
	}
	lChan := make(chan float64)
	rChan := make(chan float64)
	go func() {
		lValue, _ := evaluateRec(database, equationID, equation[:lastOperator], mu)
		lChan <- lValue
	}()
	go func() {
		rValue, _ := evaluateRec(database, equationID, equation[lastOperator+1:], mu)
		rChan <- rValue
	}()
	left := <-lChan
	right := <-rChan
	emptyComputer := 0
	for {
		mu.Lock()
		emptyComputer, err = database.GetEmptyComputer()
		mu.Unlock()
		if err != nil {
			return 0, err
		}
		if emptyComputer != 0 {
			mu.Lock()
			err = database.UpdateComputer(emptyComputer, equationID)
			mu.Unlock()
			if err != nil {
				return 0, err
			}
			break
		} else {
			time.Sleep(5 * time.Millisecond)
		}
	}
	var result float64
	switch equation[lastOperator] {
	case '+':
		mu.Lock()
		durationTime, _ := database.GetOperationTime("+")
		mu.Unlock()
		time.Sleep(time.Duration(durationTime) * time.Millisecond)
		result = left + right
	case '-':
		mu.Lock()
		durationTime, _ := database.GetOperationTime("-")
		mu.Unlock()
		time.Sleep(time.Duration(durationTime) * time.Millisecond)
		result = left - right
	case '*':
		mu.Lock()
		durationTime, _ := database.GetOperationTime("*")
		mu.Unlock()
		time.Sleep(time.Duration(durationTime) * time.Millisecond)
		result = left * right
	case '/':
		if right == 0 {
			database.UpdateComputer(emptyComputer, 0)
			return 0, errors.New("division by zero")
		}
		mu.Lock()
		durationTime, _ := database.GetOperationTime("/")
		mu.Unlock()
		time.Sleep(time.Duration(durationTime) * time.Millisecond)
		result = left / right
	}
	mu.Lock()
	database.UpdateComputer(emptyComputer, 0)
	mu.Unlock()
	return result, nil
}
