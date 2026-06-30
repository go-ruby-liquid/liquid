// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

// Error is a Liquid error. Type names the gem error class (SyntaxError,
// ArgumentError, ZeroDivisionError, …) and Message is the rendered text. In Lax
// mode a runtime Error renders inline as "Liquid error: <Message>".
type Error struct {
	Type    string
	Message string
}

func (e *Error) Error() string { return "Liquid " + lowerType(e.Type) + ": " + e.Message }

// inlineMessage is the text the gem writes into the output for a recoverable
// runtime error in Lax mode: "Liquid error: <message>".
func (e *Error) inlineMessage() string { return "Liquid error: " + e.Message }

func lowerType(t string) string {
	switch t {
	case "SyntaxError":
		return "syntax error"
	default:
		return "error"
	}
}

func syntaxErr(msg string) *Error { return &Error{Type: "SyntaxError", Message: msg} }
func argErr(msg string) *Error    { return &Error{Type: "ArgumentError", Message: msg} }
