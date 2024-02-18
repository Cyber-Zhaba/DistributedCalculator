package agent

import "testing"

func TestValidEquation(t *testing.T) {
	testCases := []struct {
		equation string
		start    int
		end      int
		want     bool
	}{
		// Test symbols [Only digits, operators and parentheses are allowed]
		{"1.2+3,4/5*6", 0, 11, true},
		{"a+b=c", 0, 5, false},
		{"1+2=3", 0, 5, false},
		// Test parentheses [Parentheses must be balanced]
		{"(1+2)*3", 0, 7, true},
		{"(1+2*3", 0, 6, false},
		{"1+2)*3", 0, 6, false},
		{"1+)2*3(", 0, 7, false},
		{"1+(2+(3+4)+5)", 0, 13, true},
		// Test operators [Operators must be between two numbers]
		// Except + and - that can be at the beginning of the equation
		{"1+2*3", 0, 5, true},
		{"1+*2", 0, 4, false},
		{"1+2*", 0, 4, false},
		{"-1+2*3", 0, 6, true},
		{"+1+*2", 0, 5, false},
		{"+1+(-1)", 0, 7, true},
		// Test numbers [Only digits and a single dot are allowed]
		{"1.2.3", 0, 5, false},
		{"1..2", 0, 4, false},
		// Test spaces [Spaces are allowed and should be ignored]
		{"1 + 2 * 3", 0, 9, true},
		{"1       +2 -     3", 0, 15, true},
	}

	for _, tc := range testCases {
		got := ValidEquation(tc.equation, tc.start, tc.end)
		if got != tc.want {
			t.Errorf("ValidEquation(%q, %d, %d) = %v; want %v", tc.equation, tc.start, tc.end, got, tc.want)
		}
	}
}

func TestLastOperation(t *testing.T) {
	testCases := []struct {
		operation string
		want      int
	}{
		// Test without parentheses
		{"1", -1},
		{"-1", -1},
		{"1+2", 1},
		{"-1*2", 2},
		{"1+2*3", 1},
		{"1+2*3+4", 5},
		{"-1+2*3+4/5", 6},
		// Test with parentheses
		{"(1+2)*3", 5},
		{"(1+2)*3+4", 7},
		{"(1+(2+3)+(4+5))", 7},
	}

	for _, tc := range testCases {
		got := LastOperation(tc.operation)
		if got != tc.want {
			t.Errorf("LastOperation(%v) = %d; want %d", tc.operation, got, tc.want)
		}
	}
}
