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

func TestUndeterminedParamsEndWithQuotedParam(t *testing.T) {
	tok := NewTokenizer("a \"b c\" d \"e f\" g \"h i\"")
	param := tok.NextParam()
	for param != "" {
		param = tok.NextParam()
	}
}

func TestUndeterminedParamsEndWithNotQuotedParam(t *testing.T) {
	tok := NewTokenizer("a \"b c\" d \"e f\" g")
	param := tok.NextParam()
	for param != "" {
		param = tok.NextParam()
	}
}

func TestSingleWord(t *testing.T) {
	tok := NewTokenizer("status")
	command := tok.NextParam()
	if command != "status" {
		t.Errorf("Command = %s, Expected: status", command)
	}
}
