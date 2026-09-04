package scanner

import (
	"errors"
	"io"
	"iter"

	"github.com/go-openapi/go-yaml/token"
)

// NOTE(fred): these methods are used in various tests but are not essential to the scanner API.
// They should be pruned.

// Scan scans the next token and returns the token collection. The source end is indicated by io.EOF.
// TODO: is this used?
func (s *Scanner) Scan() (token.Tokens, error) {
	if err := s.initErr; err != nil {
		// The source is not a YAML stream at all, so there is nothing to
		// tokenize. Reported once, and the source is then spent: a caller that
		// loops until io.EOF would otherwise never reach it.
		s.initErr = nil
		s.ctx.idx = s.ctx.size

		var invalidTokenErr *InvalidTokenError
		if errors.As(err, &invalidTokenErr) {
			s.lookback.Derive(invalidTokenErr.Token)

			return token.Tokens{invalidTokenErr.Token}, err
		}

		return nil, err
	}

	if s.ctx.idx >= s.ctx.size {
		return nil, io.EOF
	}

	ctx := s.ctx
	var err error
	for ctx.next() {
		if err = s.scan(ctx); err != nil {
			break
		}
	}

	tokens := ctx.takeTokens()

	if err != nil {
		var invalidTokenErr *InvalidTokenError
		if errors.As(err, &invalidTokenErr) {
			s.lookback.Derive(invalidTokenErr.Token)
			tokens = append(tokens, invalidTokenErr.Token)
		}
		// What was refused is dropped along with the text read towards it, so a
		// caller that scans on reads the rest of the source afresh rather than
		// continuing the token the refusal interrupted.
		ctx.abandon()

		return tokens, err
	}

	return tokens, nil
}

// Next returns the next token of the source, and false when there is none.
//
// A source the scanner refuses stops it. The tokens read before the refusal are
// handed over first, then the token the refusal names, and then Next reports
// false for good; Err says what is wrong. Err is nil where the source simply
// ran out.
//
// Next and Scan read the same source and may be used together, but a scanner is
// normally driven by one or the other.
// TODO: is this used?
func (s *Scanner) Next() (*token.Token, bool) {
	if s.ctx == nil {
		return nil, false
	}

	for {
		if tk, ok := s.ctx.popToken(); ok {
			return tk, true
		}
		if s.err != nil {
			return nil, false
		}
		if err := s.initErr; err != nil {
			s.initErr = nil
			s.stop(err)

			continue
		}
		if !s.ctx.next() {
			return nil, false
		}
		if err := s.scan(s.ctx); err != nil {
			s.stop(err)

			continue
		}
	}
}

// All returns an iterator over the tokens of the source. Stopping early leaves
// the scanner where it stands, so a further Next reads on from there.
//
// The loop ends both on the end of the source and on a refusal, so call Err
// after it to tell the two apart.
// TODO: is this used?
func (s *Scanner) All() iter.Seq[*token.Token] {
	return func(yield func(*token.Token) bool) {
		for {
			tk, ok := s.Next()
			if !ok || !yield(tk) {
				return
			}
		}
	}
}
