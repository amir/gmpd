package util

import (
	"fmt"
	"testing"
)

func TestAlaki(t *testing.T) {
	tok := NewTokenizer("amir \"mohammad saied\" \"hassan gholi\" man")
	var argv []string
	argv = append(argv, tok.NextWord())
	argv = append(argv, tok.NextParam())
	argv = append(argv, tok.NextParam())
	argv = append(argv, tok.NextParam())
	fmt.Printf("%v\n", argv)
}
