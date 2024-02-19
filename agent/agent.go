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

// PrepareEquation prepares the equation for evaluation.
// It removes all spaces and replaces commas with dots.
// It also removes unnecessary outer parentheses.
// For example, "((1+2))" becomes "(1+2)" and "(((1+2)))" becomes "(1+2)".
func PrepareEquation(equation string) string {
	// Remove all spaces from the equation
	equation = strings.ReplaceAll(equation, " ", "")
	// Replace all commas with dots
	equation = strings.ReplaceAll(equation, ",", ".")
	// Loop until there are no unnecessary outer parentheses
	for {
		// If the equation does not start and end with parentheses, return the equation
		if equation[0] != '(' || equation[len(equation)-1] != ')' {
			return equation
		}
		// Initialize a counter for the number of open parentheses
		parenthesis := 0
		// Iterate over the characters in the equation
		for i := 0; i < len(equation); i++ {
			// If the current character is an open parenthesis, increment the counter
			if equation[i] == '(' {
				parenthesis++
			} else if equation[i] == ')' {
				// If the current character is a close parenthesis, decrement the counter
				parenthesis--
				// If the counter reaches zero, and we are not at the end of the equation, return the equation
				if parenthesis == 0 {
					if i == len(equation)-1 {
						// If we are at the end of the equation, remove the outer parentheses and continue the loop
						equation = equation[1 : len(equation)-1]
					} else {
						return equation
					}
				}
			}
		}
	}
}

// ValidEquation checks if the equation is valid.
// It removes all spaces and replaces commas with dots.
// It then checks if the equation is valid from the start to the end index.
// It returns true if the equation is valid, and false otherwise.
func ValidEquation(equation string, start, end int) bool {
	// If the end index is less than or equal to the start index, return false
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
// It first prepares the equation by removing unnecessary outer parentheses.
// Then it iterates over the characters in the equation.
// If it encounters an open parenthesis, it skips to the corresponding close parenthesis.
// If it encounters an operator, and it's not the first character, it checks the operator's priority.
// If the operator's priority is less than or equal to the current operator's priority, it updates the last operator and its priority.
// The function returns the index of the last operator in the equation.
// | (1+2)+(3+4) | -1/35 | 1*2+3 | 1+2+3 |
// |      ^          ^        ^       ^
func LastOperation(equation string) int {
	// Prepare the equation by removing unnecessary outer parentheses
	equation = PrepareEquation(equation)
	// Initialize the index of the last operator and its priority
	lastOperator := -1
	operatorPriority := 2
	// Iterate over the characters in the equation
	for i := 0; i < len(equation); i++ {
		c := rune(equation[i])
		if c == '(' {
			// If the current character is an open parenthesis, skip to the corresponding close parenthesis
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
			// If the current character is an operator, and it's not the first character, check its priority
			if priority(c) <= operatorPriority {
				// If the operator's priority is less than or equal to the current operator's priority, update the last operator and its priority
				lastOperator = i
				operatorPriority = priority(c)
			}
		}
	}
	// Return the index of the last operator
	return lastOperator
}

func Evaluate(equationID int) error {
	database, _ := db.Connect("data.db")
	defer func(database *db.DB) {
		err := database.Close()
		if err != nil {
			return
		}
	}(database)
	err := database.UpdateEquation(equationID, "Computing", 0)
	if err != nil {
		return err
	}
	mu := &sync.Mutex{}
	equation := database.GetEquationText(equationID)
	var result float64
	result, err = evaluateRec(database, equationID, equation, mu)
	if err != nil {
		err = database.UpdateEquation(equationID, fmt.Sprintf("Error %s", err), 0)
		if err != nil {
			return err
		}
		return err
	}
	err = database.UpdateEquation(equationID, "Computed", result)
	if err != nil {
		return err
	}
	return nil
}

// evaluateRec recursively evaluates the given equation.
// It first prepares the equation by removing unnecessary outer parentheses.
// Then it finds the last operator in the equation.
// If there is no operator, it parses the equation as a float64 and returns the result.
// If there is an operator, it splits the equation into two parts at the operator and recursively evaluates each part.
// It then performs the operation indicated by the operator on the results of the two parts.
// The function returns the result of the operation and any error that occurred during the process.
func evaluateRec(database *db.DB, equationID int, equation string, mu *sync.Mutex) (float64, error) {
	var err error = nil
	// Prepare the equation by removing unnecessary outer parentheses
	equation = PrepareEquation(equation)
	// Find the last operator in the equation
	lastOperator := LastOperation(equation)
	if lastOperator == -1 {
		// If there is no operator, parse the equation as a float64 and return the result
		value := 0.0
		value, err = strconv.ParseFloat(equation, 64)
		if err != nil {
			return 0, err
		}
		return value, nil
	}
	// Create channels to receive the results of the recursive evaluations
	lChan := make(chan float64)
	rChan := make(chan float64)
	// Recursively evaluate the left part of the equation
	go func() {
		lValue, _ := evaluateRec(database, equationID, equation[:lastOperator], mu)
		lChan <- lValue
	}()
	// Recursively evaluate the right part of the equation
	go func() {
		rValue, _ := evaluateRec(database, equationID, equation[lastOperator+1:], mu)
		rChan <- rValue
	}()
	// Receive the results of the recursive evaluations
	left := <-lChan
	right := <-rChan
	// Find an empty computer to perform the operation
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
	// Perform the operation indicated by the operator on the results of the two parts
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
			err = database.UpdateComputer(emptyComputer, 0)
			if err != nil {
				return 0, err
			}
			return 0, errors.New("division by zero")
		}
		mu.Lock()
		durationTime, _ := database.GetOperationTime("/")
		mu.Unlock()
		time.Sleep(time.Duration(durationTime) * time.Millisecond)
		result = left / right
	}
	// Update the computer to be empty again
	mu.Lock()
	err = database.UpdateComputer(emptyComputer, 0)
	if err != nil {
		return 0, err
	}
	mu.Unlock()
	// Return the result of the operation
	return result, nil
}
