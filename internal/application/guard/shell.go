package guard

import (
	"errors"
	"regexp"
	"strings"
)

// token is a shell word or, when op is set, an operator.
type token struct {
	s  string
	op bool
}

var operators = []string{"&&", "||", ";;", ">>", ";", "|", "&", "\n", "(", ")", ">"}

var (
	heredocHead = regexp.MustCompile(`<<-?\s*(['"]?)(\w+)(['"]?)`)
	envPrefix   = regexp.MustCompile(`^[A-Za-z_]\w*=`)
	leaders     = map[string]bool{"do": true, "then": true, "else": true, "elif": true, "if": true, "while": true, "{": true, "!": true, "time": true}
)

var errUnterminated = errors.New("unterminated quote")

// tokenize splits a shell command into words and operators, honouring quotes.
func tokenize(command string) ([]token, error) {
	var tokens []token
	var word []byte
	var quote byte
	hasWord := false
	n := len(command)
	for i := 0; i < n; {
		ch := command[i]
		if quote != 0 {
			switch {
			case ch == quote:
				quote = 0
			case ch == '\\' && quote == '"' && i+1 < n:
				i++
				word = append(word, command[i])
			default:
				word = append(word, ch)
			}
			i++
			continue
		}
		if ch == '\'' || ch == '"' {
			quote, hasWord = ch, true
			i++
			continue
		}
		if ch == '\\' && i+1 < n {
			if command[i+1] != '\n' {
				word = append(word, command[i+1])
				hasWord = true
			}
			i += 2
			continue
		}
		if ch == '#' && !hasWord {
			end := strings.IndexByte(command[i:], '\n')
			if end < 0 {
				i = n
			} else {
				i += end
			}
			continue
		}
		op := ""
		for _, o := range operators {
			if strings.HasPrefix(command[i:], o) {
				op = o
				break
			}
		}
		if op != "" || ch == ' ' || ch == '\t' || ch == '\r' {
			if hasWord {
				tokens = append(tokens, token{s: string(word)})
				word, hasWord = nil, false
			}
			if op != "" {
				tokens = append(tokens, token{s: op, op: true})
				i += len(op)
			} else {
				i++
			}
			continue
		}
		word = append(word, ch)
		hasWord = true
		i++
	}
	if quote != 0 {
		return nil, errUnterminated
	}
	if hasWord {
		tokens = append(tokens, token{s: string(word)})
	}
	return tokens, nil
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// stripHeredocs replaces each heredoc (header line through terminator) with a
// newline. It ports the Python regex
// <<-?\s*(['"]?)(\w+)\1[^\n]*\n.*?\n\s*\2[ \t]*(?=\n|$) without backreferences.
func stripHeredocs(s string) string {
	var out strings.Builder
	pos := 0
	for pos < len(s) {
		loc := heredocHead.FindStringSubmatchIndex(s[pos:])
		if loc == nil {
			break
		}
		start, headEnd := pos+loc[0], pos+loc[1]
		q1, q2 := s[pos+loc[2]:pos+loc[3]], s[pos+loc[6]:pos+loc[7]]
		delim := s[pos+loc[4] : pos+loc[5]]
		end := -1
		if q1 == q2 || (q1 == "" && q2 != "") {
			if q1 == "" && q2 != "" {
				headEnd-- // the trailing quote is not part of the header match
			}
			end = heredocEnd(s, headEnd, delim)
		}
		if end < 0 {
			out.WriteString(s[pos : start+2])
			pos = start + 2
			continue
		}
		out.WriteString(s[pos:start])
		out.WriteString("\n")
		pos = end
	}
	out.WriteString(s[pos:])
	return out.String()
}

// heredocEnd returns where a heredoc whose header ends at from stops, or -1.
func heredocEnd(s string, from int, delim string) int {
	nl := strings.IndexByte(s[from:], '\n')
	if nl < 0 {
		return -1
	}
	for q := from + nl + 1; q < len(s); q++ {
		if s[q] != '\n' {
			continue
		}
		j := q + 1
		for j < len(s) && isSpace(s[j]) {
			j++
		}
		if !strings.HasPrefix(s[j:], delim) {
			continue
		}
		j += len(delim)
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if j == len(s) || s[j] == '\n' {
			return j
		}
	}
	return -1
}

type pipeline struct {
	stages     [][]string
	redirected bool
}

// pipelines groups tokens into pipelines of stages, marking redirected ones.
func pipelines(tokens []token) []pipeline {
	var result []pipeline
	var stages [][]string
	var stage []string
	redirected := false
	endStage := func() {
		if len(stage) > 0 {
			stages = append(stages, stage)
		}
		stage = nil
	}
	endPipeline := func() {
		endStage()
		if len(stages) > 0 {
			result = append(result, pipeline{stages, redirected})
		}
		stages, redirected = nil, false
	}
	for _, t := range tokens {
		switch {
		case !t.op:
			stage = append(stage, t.s)
		case t.s == "|":
			endStage()
		case t.s == ">" || t.s == ">>":
			redirected = true
		default:
			endPipeline()
		}
	}
	endPipeline()
	return result
}

// commandWords drops leading keywords and VAR= prefixes from a stage.
func commandWords(stage []string) []string {
	i := 0
	for i < len(stage) && (leaders[stage[i]] || envPrefix.MatchString(stage[i])) {
		i++
	}
	return stage[i:]
}
