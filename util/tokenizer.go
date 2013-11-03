// Package util provides string utilities required by gmpd
package util

import (
	"unicode"
	"unicode/utf8"
)

const (
	eol = -1
)

type pos int

// Tokenizer holds the state of the scanner.
type Tokenizer struct {
	input string // the string being scanned
	pos   pos    // current position in the input
	width pos    // width of last rune read from input
}

// NewTokenizer allocates a new Tokenizer with the given input.
func NewTokenizer(input string) *Tokenizer {
	return &Tokenizer{input, 0, 0}
}

// NextWord returns the next word from the input.
func (t *Tokenizer) NextWord() string {
	var runes []rune
	for t.peek() != eol {
		if !isSpace(t.peek()) {
			runes = append(runes, t.next())
		} else {
			t.next()
			return string(runes)
		}
	}
	t.consumeSpaces()
	return string(runes)
}

// NextParam returns the next param from the input
// A word if not quoted, or a couple of words if they're quoted.
func (t *Tokenizer) NextParam() string {
	if t.peek() == '"' {
		return t.nextString()
	}

	return t.NextWord()
}

// nextString returns the next quoted words in the input.
func (t *Tokenizer) nextString() string {
	var runes []rune
	if t.next() != '"' {
		return ""
	}
	for t.peek() != '"' {
		if t.peek() == '\\' {
			t.next()
		} else {
			runes = append(runes, t.next())
		}
	}
	t.next()
	t.consumeSpaces()
	return string(runes)
}

// next returns the next rune in the input.
func (t *Tokenizer) next() rune {
	if int(t.pos) >= len(t.input) {
		return eol
	}
	r, s := utf8.DecodeRuneInString(t.input[t.pos:])
	t.width = pos(s)
	t.pos += t.width
	return r
}

// consumeSpaces consumes all spaces between words.
func (t *Tokenizer) consumeSpaces() {
	for isSpace(t.peek()) {
		t.next()
	}
}

// peek returns but does not consume the next rune in the input.
func (t *Tokenizer) peek() rune {
	r := t.next()
	t.backup()
	return r
}

// backup steps back one rune.
func (t *Tokenizer) backup() {
	t.pos -= t.width
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isAlphaNumeric reports whether r is an alphabetic, or digit.
func isAlphaNumeric(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
