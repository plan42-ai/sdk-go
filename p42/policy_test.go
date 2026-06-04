package p42

import (
	"testing"
)

func TestWildcardActionBitVectorIsAllOnes(t *testing.T) {
	bv, ok := ActionToBit[Action("*")]
	if !ok {
		t.Fatal("wildcard '*' not found in ActionToBit map")
	}
	if !bv.AllOnes() {
		t.Fatalf("expected all ones for wildcard, got High=%d Low=%d", bv.High, bv.Low)
	}
	if bv.High != -1 || bv.Low != -1 {
		t.Fatalf("expected {-1, -1}, got {%d, %d}", bv.High, bv.Low)
	}
}

func TestWildcardTokenTypeBitVectorIsAllOnes(t *testing.T) {
	bv, ok := TokenTypeToBit[TokenType("*")]
	if !ok {
		t.Fatal("wildcard '*' not found in TokenTypeToBit map")
	}
	if !bv.AllOnes() {
		t.Fatalf("expected all ones for wildcard, got %d", bv)
	}
}
