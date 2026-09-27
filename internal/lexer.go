package internal

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

const (
	// TokUnknown etc. are used to control the flow of the parser itself, with error reporting, termination, etc.
	TokUnknown TokKind = iota
	TokErr
	TokEof

	// TokIden etc. represent "variable" data that may need to be parsed later
	TokIden
	TokInteger
	TokFloat
	TokString
	TokTag

	// TokSemicolon etc. are special character tokens used to control termination of ASTs
	TokSemicolon
	TokComma
	TokLBrace
	TokRBrace
	TokLParen
	TokRParen
	TokLBrack
	TokRBrack
	TokEqual

	// TokRequired etc., are "literal" tokens which represent extract symbols for controlling AST creation
	TokRequired
	TokOptional
	TokDeprecated
	TokStruct
	TokUnion
	TokEnum
	TokReturns
	TokRpc
	TokImport
	TokMessage
	TokService

	// TokComment can be an "expected" token, but is never emitted for the parser to consume
	TokComment

	// TokField etc. these are "fake" tokens which represents multiple "literal" tokens, which the parser may expect, but will never attempt to consume
	TokField
	TokTypeRef
	TokTypeDef
	TokCase
	TokOption
)

const (
	numeric              = "1234567890"
	control              = "=()[]{};,/|"
	whitespace           = " \t\r\n\f"
	whitespaceAndControl = whitespace + control
	newline              = "\r\n"
)

type TokKind int

func (k TokKind) String() string {
	switch k {
	case TokErr:
		return "error"
	case TokEof:
		return "eof"
	case TokIden:
		return "iden"
	case TokInteger:
		return "integer"
	case TokFloat:
		return "float"
	case TokString:
		return "string"
	case TokTag:
		return "ord"
	case TokSemicolon:
		return "';'"
	case TokComma:
		return "','"
	case TokLBrace:
		return "'{'"
	case TokRBrace:
		return "'}'"
	case TokLParen:
		return "'('"
	case TokRParen:
		return "')'"
	case TokLBrack:
		return "'['"
	case TokRBrack:
		return "']'"
	case TokEqual:
		return "'='"
	case TokMessage:
		return "message"
	case TokService:
		return "service"
	case TokRequired:
		return "required"
	case TokOptional:
		return "optional"
	case TokDeprecated:
		return "deprecated"
	case TokStruct:
		return "struct"
	case TokUnion:
		return "union"
	case TokEnum:
		return "enum"
	case TokReturns:
		return "returns"
	case TokRpc:
		return "rpc"
	case TokImport:
		return "import"
	case TokComment:
		return "'//'"
	case TokTypeRef:
		return "type"
	case TokTypeDef:
		return "typedef"
	case TokField:
		return "struct field"
	case TokCase:
		return "enum case"
	case TokOption:
		return "union option"
	default:
		panic(fmt.Sprintf("assertion error: string func: unknown token: %d", k))
	}
}

type TokVal struct {
	Kind     TokKind
	Str      string
	Expected TokKind // the expected token whenever an error is occurred, only populated for Kind of TokErr
	Int      big.Int // populated for tokens with unbounded integer values (TokOrd, TokInteger)
	Float64  float64 // populated for tokens with float values (TokFloat)
}

func (t TokVal) String() string {
	switch t.Kind {
	case TokUnknown:
		return "<unknown>"
	case TokEof:
		return "<eof>"
	default:
		return t.Str
	}
}

type Token struct {
	TokVal
	Positions
}

func (t Token) String() string {
	return fmt.Sprintf("'%s'", t.TokVal.String())
}

type Lexer struct {
	input        string
	currentIndex int
	lastIndex    int
	tokens       []Token
}

const eof = 0

func makeLexer(input string) Lexer {
	return Lexer{input: input, tokens: make([]Token, 0)}
}

// peek checks the character at the current offset, propagating an eof signal if the end of the string has been reached
func (lex *Lexer) peek() rune {
	if lex.currentIndex >= len(lex.input) {
		return eof
	}
	// note(Joseph): Doesn't parse multiple byte Unicode characters properly - we never check for Unicode chars, byte by byte lexing is simpler
	// A big assumption here is that we never need to use peek() to check a Unicode character
	return rune(lex.input[lex.currentIndex])
}

// next checks the character at the current offset and advances to the next byte
func (lex *Lexer) next() rune {
	prev := lex.peek()
	lex.consume()
	return prev
}

// take takes the next character and returns if it is equal to a target character
func (lex *Lexer) take(ch rune) bool {
	return ch == lex.next()
}

// jump sets the offset to a specific index and returns the char at that offset
func (lex *Lexer) jump(index int) rune {
	lex.currentIndex = index
	if lex.currentIndex < lex.lastIndex {
		panic(fmt.Sprintf("assertion error: while lexing: cannot jump current index %d behind the previous index %d", index, lex.lastIndex))
	}
	return lex.peek()
}

// consume advances current offset to the next byte without checking the current byte-offset
func (lex *Lexer) consume() {
	lex.currentIndex++
}

// skip advances the last offset to the current offset - skipping whatever substring is being lexed
func (lex *Lexer) skip() {
	lex.lastIndex = lex.currentIndex
}

// span gets the substring between the last and current offset
func (lex *Lexer) span() string {
	// invariant: curr offset should always be ahead of start offset
	return lex.input[lex.lastIndex:lex.currentIndex]
}

func (lex *Lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, lex.peek()) {
		lex.consume()
		return true
	}
	return false
}

func (lex *Lexer) assert(valid string) bool {
	if strings.ContainsRune(valid, lex.peek()) {
		return true
	}
	lex.consume()
	return false
}

func (lex *Lexer) acceptWhile(valid string) {
	for strings.ContainsRune(valid, lex.peek()) {
		lex.consume()
	}
}

func (lex *Lexer) acceptUntil(invalid string) rune {
	// note(Joseph): strings.IndexAny is vectorized, preferred over using one by one "peek" lexing
	inputSubstr := lex.input[lex.currentIndex:]
	substrIndex := strings.IndexAny(inputSubstr, invalid)

	targetIndex := len(lex.input)
	if substrIndex != -1 {
		targetIndex = lex.currentIndex + substrIndex
	}

	return lex.jump(targetIndex)
}

func (lex *Lexer) getPositions() Positions {
	return Positions{Begin: lex.lastIndex, End: lex.currentIndex}
}

func (lex *Lexer) emitValue(tokVal TokVal) {
	lex.tokens = append(lex.tokens, Token{tokVal, lex.getPositions()})
	lex.skip()
}

func (lex *Lexer) emit(kind TokKind) {
	value := lex.span()
	lex.emitValue(TokVal{Kind: kind, Str: value})
}

func (lex *Lexer) emitText() {
	str := lex.span()

	kind := TokIden
	switch str {
	case "struct":
		kind = TokStruct
	case "union":
		kind = TokUnion
	case "enum":
		kind = TokEnum
	case "message":
		kind = TokMessage
	case "service":
		kind = TokService
	case "required":
		kind = TokRequired
	case "optional":
		kind = TokOptional
	case "deprecated":
		kind = TokDeprecated
	case "returns":
		kind = TokReturns
	case "rpc":
		kind = TokRpc
	case "import":
		kind = TokImport
	}

	lex.tokens = append(lex.tokens, Token{TokVal{Kind: kind, Str: str}, lex.getPositions()})
	lex.skip()
}

func (lex *Lexer) emitFloat(kind TokKind, f64 float64) {
	value := lex.span()
	lex.emitValue(TokVal{Kind: kind, Str: value, Float64: f64})
}

func (lex *Lexer) emitInteger(kind TokKind, i big.Int) {
	value := lex.span()
	lex.emitValue(TokVal{Kind: kind, Str: value, Int: i})
}

func (lex *Lexer) emitNext(kind TokKind) {
	lex.consume()
	lex.emit(kind)
}

func (lex *Lexer) emitError(expected TokKind) struct{} {
	// scan until a sentinel symbol
	if expected == TokComment {
		lex.acceptUntil(newline)
	} else {
		lex.acceptUntil(whitespaceAndControl)
	}

	value := lex.span()
	token := Token{TokVal{Kind: TokErr, Str: value, Expected: expected}, lex.getPositions()}

	lex.tokens = append(lex.tokens, token)
	lex.skip()

	return struct{}{}
}

func (lex *Lexer) run() {
	for hasNext := true; hasNext; {
		hasNext = lex.lex()
	}
}

func (lex *Lexer) lexNumeric() struct{} {
	kind := TokInteger

	for {
		lex.acceptWhile(numeric)
		if lex.accept(".") {
			kind = TokFloat
		} else if lex.assert(whitespaceAndControl) {
			break
		} else {
			// stop at first invalid non-numeric
			return lex.emitError(kind)
		}
	}

	numericStr := lex.span()
	switch kind {
	case TokInteger:
		integer, ok := new(big.Int).SetString(numericStr, 10)
		if !ok {
			return lex.emitError(kind)
		}
		lex.emitInteger(kind, *integer)
	case TokFloat:
		f64, err := strconv.ParseFloat(numericStr, 64)
		if err != nil {
			return lex.emitError(kind)
		}
		lex.emitFloat(kind, f64)
	default:
		panic(fmt.Sprintf("assertion error: numeric kind is unexpected: %s", kind))
	}

	return struct{}{}
}

func (lex *Lexer) lexComment() struct{} {
	lex.next()
	if !lex.take('/') {
		return lex.emitError(TokComment)
	}
	lex.acceptUntil(newline)
	lex.skip()
	return struct{}{}
}

func (lex *Lexer) lexTag() struct{} {
	kind := TokTag
	lex.next()
	lex.acceptWhile(numeric)
	if !lex.assert(whitespaceAndControl) {
		return lex.emitError(kind)
	}

	// tag must be at least 2 characters long
	tagStr := lex.span()
	if len(tagStr) <= 1 {
		return lex.emitError(kind)
	}

	// invariant: tag is at least 2 characters long
	tagInt, ok := new(big.Int).SetString(tagStr[1:], 10)
	if !ok {
		return lex.emitError(kind)
	}

	lex.emitInteger(TokTag, *tagInt)

	return struct{}{}
}

func (lex *Lexer) lexText() {
	lex.acceptUntil(whitespaceAndControl)
	lex.emitText()
}

const stringTerminators = "\"\\"

func (lex *Lexer) lexString() struct{} {
	lex.next()

	kind := TokString

Loop:
	for {
		ch := lex.acceptUntil(stringTerminators)
		if ch == -1 {
			return lex.emitError(kind)
		}
		lex.consume()

		switch ch {
		case '"':
			// stop lexing the string when a terminating quote is found
			break Loop
		case '\\':
			// an escape sequence is found, then the next character cannot terminate the string - so skip it
			lex.consume()
		default:
			panic(fmt.Sprintf("assertion error: expected acceptUntil to return a value within %s, got %c", stringTerminators, ch))
		}
	}

	lex.emit(TokString)
	return struct{}{}
}

func (lex *Lexer) lex() bool {
	lex.acceptWhile(whitespace)
	lex.skip()

	ch := lex.peek()
	switch ch {
	case eof:
		lex.emit(TokEof)
		return false
	case '=':
		lex.emitNext(TokEqual)
	case '{':
		lex.emitNext(TokLBrace)
	case '}':
		lex.emitNext(TokRBrace)
	case '(':
		lex.emitNext(TokLParen)
	case ')':
		lex.emitNext(TokRParen)
	case '[':
		lex.emitNext(TokLBrack)
	case ']':
		lex.emitNext(TokRBrack)
	case ';':
		lex.emitNext(TokSemicolon)
	case ',':
		lex.emitNext(TokComma)
	case '/':
		lex.lexComment()
	case '@':
		lex.lexTag()
	case '"':
		lex.lexString()
	default:
		if lex.accept(numeric) {
			lex.lexNumeric()
		} else if !unicode.IsControl(ch) && !unicode.IsPunct(ch) && !unicode.IsSpace(ch) {
			lex.lexText()
		} else {
			lex.emitError(TokUnknown)
		}
	}
	return true
}
