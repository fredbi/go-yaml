// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package testintegration holds the tests that need third-party libraries.
//
// It is a module of its own so that those dependencies never reach the
// library's go.mod: go-yaml publishes no runtime dependencies, and
// go-openapi/core depends on it, so anything added there propagates.
//
// It is in go.work, so `go test work ./...` runs it along with everything else.
package testintegration
