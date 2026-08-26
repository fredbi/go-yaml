// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import "github.com/go-openapi/go-yaml/token"

// InvalidTokenError is what a scanner stopped on, and why.
type InvalidTokenError struct {
	Token   *token.Token
	Message string
}

func (e *InvalidTokenError) Error() string {
	return e.Message
}

// ErrInvalidToken reports msg against tk.
func ErrInvalidToken(msg string, tk *token.Token) *InvalidTokenError {
	return &InvalidTokenError{
		Token:   tk,
		Message: msg,
	}
}
