// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import "github.com/go-openapi/go-yaml/token"

// InvalidTokenError reports where and why a scan stopped.
type InvalidTokenError struct {
	Token   *token.Token
	Message string
}

func (e *InvalidTokenError) Error() string {
	return e.Message
}

// ErrInvalidToken builds an [InvalidTokenError] reporting an error message msg against the token tk.
func ErrInvalidToken(msg string, tk *token.Token) *InvalidTokenError {
	return &InvalidTokenError{
		Token:   tk,
		Message: msg,
	}
}
