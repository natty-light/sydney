package types

import (
	"testing"
)

func TestString(t *testing.T) {
	tests := []struct {
		t             Type
		expectedValue string
	}{
		{
			ArrayType{ElemType: Int},
			"array<int>",
		},
		{
			FunctionType{Return: Int, Params: make([]Type, 0)},
			"func<() -> int>",
		},
		{
			FunctionType{Return: Bool, Params: []Type{Bool, Int}},
			"func<(bool, int) -> bool>",
		},
	}

	for _, test := range tests {
		if test.t.Signature() != test.expectedValue {
			t.Fatalf("Signature mismatch: expected %s, got %s", test.expectedValue, test.t.Signature())
		}
	}
}
