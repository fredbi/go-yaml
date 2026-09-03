// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/internal/tokenarena"
	"github.com/go-openapi/go-yaml/token"
)

// reader turns a document into the tokens the descent walks, as the descent
// asks for them.
//
// It scans a run into the arena, groups that run, and hands the result on a
// token at a time. Nothing reads further than the descent has asked, so the
// scanner, the grouping and the parse all run at once and the tape may be
// filled again behind them.
//
// A document opens at "---" or at the first token of the stream, and closes at
// "...", at the "---" opening the next one, or at the end. The parse asks for
// those three in turn: [reader.openDocument], then [reader.bodyToken] until it
// says the document has run out, then [reader.closeDocument].
type reader struct {
	scan  *scanner.Scanner
	arena *tokenarena.TokenArena[tapeToken]
	g     grouper

	// run holds the tokens read and not yet grouped, at most batch of them.
	// out holds what the grouping made of the last run, from at onward.
	run   []*tapeToken
	out   []*tapeToken
	at    int
	batch int

	// seq is the place on the tape of the next token read.
	seq int
	// drained says the scanner has no more to give.
	drained bool
	// keepComments says the mode asked for them. The rest are dropped as they
	// arrive and never reach the grouping.
	keepComments bool

	// afterHeader and afterEnd hold the marker just read, so the token after it
	// can be held against the line the marker stands on.
	afterHeader, afterEnd *tapeToken
	// taken says a token was read at all, since an empty stream is one empty
	// document and a "..." closing nothing is none. ended says the document
	// being read has run out. tail says the empty document at the end of a
	// stream has been handed over.
	taken, ended, tail bool

	// err is a refusal bodyToken could not report, since the descent's pull
	// answers with a token or nothing.
	err error
}

// newReader returns a reader over what scan hands out.
//
// estimate is how many tokens the document is guessed to hold; it sizes buffers
// and nothing else.
func newReader(scan *scanner.Scanner, arena *tokenarena.TokenArena[tapeToken], batch, estimate int, keepComments bool) *reader {
	r := &reader{
		scan:         scan,
		arena:        arena,
		g:            newGrouper(estimate),
		run:          make([]*tapeToken, 0, batch),
		batch:        batch,
		keepComments: keepComments,
	}
	if keepComments {
		// Taken here rather than where the first comment arrives, so that the
		// parser may hold the same map from the start. A parse dropping
		// comments takes none.
		r.g.lineComments = make(map[*tapeToken]*token.Token)
	}

	return r
}

// peek returns the next grouped token without taking it, grouping another run
// where it has none in hand.
func (r *reader) peek() (*tapeToken, error) {
	for r.at >= len(r.out) {
		if r.drained {
			return nil, nil
		}
		if err := r.fill(); err != nil {
			return nil, err
		}
	}

	return r.out[r.at], nil
}

// take returns the next grouped token and steps past it.
func (r *reader) take() (*tapeToken, error) {
	tk, err := r.peek()
	if err != nil || tk == nil {
		return nil, err
	}
	r.at++
	r.taken = true

	return tk, nil
}

// openDocument begins the next document and returns the "---" that opened it,
// where there is one. ok is false at the end of the stream.
func (r *reader) openDocument() (*token.Token, bool, error) {
	// A "..." standing where nothing is open closes nothing: l-yaml-stream
	// admits a run of suffixes and only the first closes anything.
	for {
		tk, err := r.peek()
		if err != nil {
			return nil, false, err
		}
		if tk == nil || tk.Type() != token.DocumentEndType {
			break
		}
		if _, err := r.take(); err != nil {
			return nil, false, err
		}
		r.afterEnd = tk
		if err := r.judgeNext(); err != nil {
			return nil, false, err
		}
	}

	tk, err := r.peek()
	if err != nil {
		return nil, false, err
	}
	r.ended = false

	if tk == nil {
		// The stream ends here. It holds one last document -- the empty one --
		// unless something was read from it.
		if r.taken || r.tail {
			return nil, false, nil
		}
		r.tail = true

		return nil, true, nil
	}
	if tk.Type() != token.DocumentHeaderType {
		return nil, true, nil
	}

	head, err := r.take()
	if err != nil {
		return nil, false, err
	}
	r.afterHeader = head

	return head.RawToken(), true, r.judgeNext()
}

// bodyToken draws the next token of the document being read, and reports false
// where the document ends: at "...", at the "---" opening the next one, or at
// the end of the stream.
//
// It is what the descent's run pulls from, so it answers with a token or
// nothing and keeps a refusal in err for the parse to find.
func (r *reader) bodyToken() (*tapeToken, bool) {
	if r.ended || r.err != nil {
		return nil, false
	}

	tk, err := r.peek()
	if err != nil {
		r.err = err

		return nil, false
	}
	if tk == nil {
		r.ended = true

		return nil, false
	}

	switch tk.Type() {
	case token.DocumentHeaderType, token.DocumentEndType:
		r.ended = true

		return nil, false
	}

	if _, err := r.take(); err != nil {
		r.err = err

		return nil, false
	}

	return tk, true
}

// closeDocument finishes the document being read and returns the "..." that
// ended it, where there is one.
func (r *reader) closeDocument() (*token.Token, error) {
	if r.err != nil {
		return nil, r.err
	}

	tk, err := r.peek()
	if err != nil || tk == nil || tk.Type() != token.DocumentEndType {
		return nil, err
	}

	end, err := r.take()
	if err != nil {
		return nil, err
	}
	r.afterEnd = end

	return end.RawToken(), r.judgeNext()
}

// judgeNext holds the token after a marker against the line the marker stands
// on. It reads one token ahead and takes none.
func (r *reader) judgeNext() error {
	tk, err := r.peek()
	if err != nil || tk == nil {
		r.afterHeader, r.afterEnd = nil, nil

		return err
	}

	switch {
	case r.afterHeader != nil && r.afterHeader.Line() == tk.Line():
		switch tk.GroupType() {
		case TokenGroupMapKey, TokenGroupMapKeyValue:
			return yamlerrors.NewSyntax("value cannot be placed after document separator", tk.RawToken())
		}
		if tk.Type() == token.SequenceEntryType {
			return yamlerrors.NewSyntax("value cannot be placed after document separator", tk.RawToken())
		}
	case r.afterEnd != nil && r.afterEnd.Line() == tk.Line():
		// "..." ends the document and takes the rest of its line: only a
		// comment may follow it there. On the next line a new document begins,
		// and it may be a bare one.
		return yamlerrors.NewSyntax("unexpected end content", tk.RawToken())
	}
	r.afterHeader, r.afterEnd = nil, nil

	return nil
}

// fill reads one run of tokens and keeps what the grouping makes of it.
func (r *reader) fill() error {
	for len(r.run) < r.batch {
		tk, ok := r.scan.NextToken()
		if !ok {
			r.drained = true

			break
		}
		if !r.keepComments && tk.Type == token.CommentType {
			continue
		}

		held, _ := r.arena.Add(tapeToken{})
		held.Raw(tk, r.seq)
		r.seq++

		if tk.Type == token.InvalidType {
			// A token the scanner refused carries no reason of its own:
			// Scanner.Err has it, and it names the character or the header
			// option that was wrong. Reporting "found an invalid token"
			// instead loses that.
			if scanErr := r.scan.Err(); scanErr != nil {
				return scanErr
			}

			return yamlerrors.NewSyntax("found an invalid token", held.RawToken())
		}
		r.run = append(r.run, held)
	}
	if err := r.scan.Err(); err != nil {
		return err
	}

	g := &r.g
	g.ending = r.drained

	// Each pass reads what the one before it left and hands on what it made of
	// it, keeping on the grouper what it cannot settle yet, so a group
	// straddling the join between two runs is grouped as one.
	// The first two stages read one token at a time, the rest still take the
	// run whole. What the machine hands out is the run the passes then read.
	machine := g.out(len(r.run))
	for _, tk := range r.run {
		machine = g.feed(tk, machine)
	}
	if g.ending {
		machine = g.finish(machine)
	}

	out := g.groupDirectives(machine)
	if g.err != nil {
		return g.err
	}

	// out comes from a buffer the next run writes over, so what is left of the
	// one before is kept and this run put after it.
	kept := append([]*tapeToken(nil), r.out[r.at:]...)
	r.out = append(append(r.out[:0], kept...), out...)
	r.at = 0
	r.run = r.run[:0]

	return nil
}
