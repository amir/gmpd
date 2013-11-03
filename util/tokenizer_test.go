package util

import (
	"testing"
)

func TestSearch(t *testing.T) {
	tok := NewTokenizer("search any \"Comfortably Numb\"")
	command := tok.NextParam()
	if command != "search" {
		t.Errorf("Command = %s, want %s", command, "search")
	}
	tok.NextParam()
	query := tok.NextParam()
	if query != "Comfortably Numb" {
		t.Errorf("Query = %s, want %s", query, "Comfortably Numb")
	}
}
