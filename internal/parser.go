package internal

import (
	"fmt"
	"slices"
	"strings"
)

type Parser struct {
	tokens    []Token
	curr      int
	nodes     []DefNode
	errs      []ParseError
	hasEofErr bool // stores whether an error has been emitted after token stream has reached eof
}

func makeParser(tokens []Token) Parser {
	return Parser{tokens: tokens, hasEofErr: false, errs: make([]ParseError, 0)}
}

func Parse(spec string) ([]DefNode, []ParseError) {
	lex := makeLexer(spec)
	lex.run()

	p := makeParser(lex.tokens)
	p.parse()

	return p.nodes, p.errs
}

func MustParse(spec string) []DefNode {
	nodes, errs := Parse(spec)

	if len(errs) > 0 {
		panic(fmt.Sprintf("%+v", errs))
	}

	return nodes
}

func (p *Parser) next() Token {
	token := p.tokens[p.curr]
	if token.Kind != TokEof {
		p.curr++
	}
	return token
}

func (p *Parser) eat() {
	if p.peek().Kind != TokEof {
		p.curr++
	}
}

func (p *Parser) prev() {
	p.curr--
	if p.curr < 0 {
		panic("assertion error: current position in parser should never be less than 0")
	}
}

func (p *Parser) peek() Token {
	return p.tokens[p.curr]
}

func (p *Parser) expect(expected TokKind) (Token, ParseError) {
	token := p.next()
	if expected != token.Kind {
		return token, makeExpectErr(token, expected)
	}
	return token, ParseError{}
}

func (p *Parser) eatWhile(expected TokKind) (Token, bool) {
	firstToken := p.peek()
	ok := false
	for p.peek().Kind == expected {
		if !ok {
			ok = true
		}
		p.eat()
	}
	return firstToken, ok
}

func (p *Parser) expectChain(chain ...TokKind) ParseError {
	for _, expected := range chain {
		if _, err := p.expect(expected); err.isPresent() {
			return err
		}
	}
	return ParseError{}
}

func (p *Parser) skipUntil(eatTokens []TokKind, stopTokens []TokKind) {
	for {
		token := p.peek()
		if token.Kind == TokEof {
			return
		}
		matchIdx := slices.Index(eatTokens, token.Kind)
		if matchIdx != -1 {
			p.eatWhile(eatTokens[matchIdx])
			return
		}
		if slices.Index(stopTokens, token.Kind) != -1 {
			return
		}
		p.eat()
	}
}

// EatTokens sentinel tokens which are eaten during propagating an error
var EatTokens = []TokKind{TokSemicolon}

// StopTokens sentinel tokens which are stopped at during propagating an error
var StopTokens = []TokKind{TokLBrace, TokRBrace, TokService, TokRpc, TokRequired, TokOptional, TokDeprecated, TokMessage, TokStruct, TokUnion, TokEnum}

func (p *Parser) skipUntilSentinel() {
	p.skipUntil(EatTokens, StopTokens)
}

// StopFieldTokens sentinel tokens which are stopped at during propagating field-level errors
var StopFieldTokens = []TokKind{TokSemicolon}

func (p *Parser) skipUntilFieldSentinel() {
	p.skipUntil(nil, StopFieldTokens)
}

func (p *Parser) emitError(parseErr ParseError) {
	if p.hasEofErr {
		return
	}
	// don't emit anymore errors if a single eof err has been reached
	p.errs = append(p.errs, parseErr)
	p.hasEofErr = p.peek().Kind == TokEof
}

func (p *Parser) parse() {
	for {
		root, err := p.parseRoot()
		if err.errKind == EofErrKind {
			break
		}
		if err.isPresent() {
			p.emitError(err)
			p.skipUntilSentinel()
		}
		p.nodes = append(p.nodes, root)
	}
}

func (p *Parser) parseRoot() (DefNode, ParseError) {
	var defNode DefNode
	var err ParseError

	token := p.peek()
	switch token.Kind {
	case TokEof:
		return DefNode{}, makeEofErr()
	case TokMessage:
		p.eat()
		err = p.parseMessage(&defNode)
	case TokService:
		p.parseService(&defNode)
	case TokImport:
		p.parseImport(&defNode)
	case TokIden:
		p.parseProperty(&defNode)
	default:
		p.eat()
		err = makeExpectErr(token, TokMessage, TokService, TokImport, TokIden)
	}

	return defNode, err
}

func (p *Parser) propagateDefNode(propertyNode *DefNode, parseErr ParseError) struct{} {
	propertyNode.End = parseErr.actualToken.End
	propertyNode.Poisoned = true

	parseErr.addKind(propertyNode.Kind)
	p.skipUntilSentinel()
	p.emitError(parseErr)

	return struct{}{}
}

func (p *Parser) parseProperty(propertyNode *DefNode) struct{} {
	propertyNode.Kind = PropertyNodeKind

	var token Token

	token, err := p.expect(TokIden)
	if err.isPresent() {
		panic(fmt.Sprintf("assertion error: %s", err))
	}
	propertyNode.Begin = token.Begin
	propertyNode.Iden = token.Str

	if _, err := p.expect(TokEqual); err.isPresent() {
		return p.propagateDefNode(propertyNode, err)
	}

	str, err := p.parseString(&token)
	if err.isPresent() {
		return p.propagateDefNode(propertyNode, err)
	}
	propertyNode.End = token.End
	propertyNode.StrValue = str

	return struct{}{}
}

var escSeqTable = map[rune]rune{'\\': '\\', 'n': '\n', '\t': '\t', 'f': '\f', 'r': '\r', '"': '"'}

func (p *Parser) parseString(token *Token) (string, ParseError) {
	t, err := p.expect(TokString)
	if err.isPresent() {
		return "", err
	}
	*token = t

	if len(token.Str) < 2 {
		panic(fmt.Sprintf("assertion error: string must be at least length 2, was: %s", token.Str))
	}

	var sb strings.Builder

	isEscaped := false
	str := token.Str[1 : len(token.Str)-1]

	for _, ch := range str {
		if isEscaped {
			ch, ok := escSeqTable[ch]
			if !ok {
				return "", makeEscSeqErr(*token, ch)
			}
			isEscaped = false
			sb.WriteRune(ch)
		} else {
			if ch == '\\' {
				isEscaped = true
			} else {
				sb.WriteRune(ch)
			}
		}
	}

	return sb.String(), ParseError{}
}

func (p *Parser) parseImport(importNode *DefNode) struct{} {
	importNode.Kind = ImportNodeKind

	var token Token

	token, err := p.expect(TokImport)
	if err.isPresent() {
		panic(fmt.Sprintf("assertion error: %s", err))
	}
	importNode.Begin = token.Begin

	pathStr, err := p.parseString(&token)
	if err.isPresent() {
		return p.propagateDefNode(importNode, err)
	}
	importNode.End = token.End
	importNode.StrValue = pathStr

	return struct{}{}
}

const DefaultMSize = 16

func (p *Parser) parseMessageSize(callKind NodeKind) (uint64, ParseError) {
	if token := p.peek(); token.Kind != TokLBrack {
		return DefaultMSize, ParseError{} // defaults when size is not provided - struct will never use this
	}
	p.eat()

	token, err := p.expect(TokInteger)
	if err.isPresent() {
		return 0, err
	}
	size := token.Int.Uint64()

	if _, err := p.expect(TokRBrack); err.isPresent() {
		return 0, err
	}

	// if the next token is a struct, emit an error, but not return the error to caller, we wish to continue parsing
	if p.peek().Kind == TokStruct {
		p.emitError(makeKindErr(token, SizeErrKind).withKind(callKind))
	}
	return size, ParseError{}
}

func (p *Parser) parseMessage(messageNode *DefNode) ParseError {
	var token Token
	var err ParseError

	kind := MessageNodeKind

	// invariant: assume that 'errKind' token has been consumed
	token, err = p.expect(TokIden)
	if err.isPresent() {
		return err.withKind(kind)
	}
	name := token.Str

	size, err := p.parseMessageSize(kind)
	if err.isPresent() {
		return err.withKind(kind)
	}

	token = p.peek()
	switch token.Kind {
	case TokStruct:
		p.parseStruct(messageNode, name)
	case TokEnum:
		p.parseEnum(messageNode, name, size)
	case TokUnion:
		p.parseUnion(messageNode, name, size)
	default:
		p.eat()
		err = makeExpectErr(token, TokTypeDef).withKind(kind)
	}

	return err
}

func (p *Parser) parseStruct(structNode *DefNode, name string) struct{} {
	structNode.Kind = StructNodeKind
	structNode.Iden = name

	token, err := p.expect(TokStruct)
	if err.isPresent() {
		panic(fmt.Sprintf("assertion error: in struct: %s", err))
	}
	structNode.Begin = token.Begin

	typeParams, err := p.parseTypeParams()
	if err.isPresent() {
		return p.propagateDefNode(structNode, err)
	}
	structNode.TypeParams = typeParams

	if _, err := p.expect(TokLBrace); err.isPresent() {
		return p.propagateDefNode(structNode, err)
	}

	for {
		token := p.next()
		switch token.Kind {
		case TokOptional, TokRequired, TokDeprecated:
			p.prev()
			var fieldNode MemberNode
			p.parseField(&fieldNode)
			structNode.Members = append(structNode.Members, fieldNode)
		case TokMessage:
			var messageNode DefNode
			if err := p.parseMessage(&messageNode); err.isPresent() {
				p.propagateDefNode(structNode, err)
				continue
			}
			structNode.LocalDefs = append(structNode.LocalDefs, messageNode)
		case TokRBrace:
			structNode.End = token.End
			return struct{}{}
		default:
			p.propagateDefNode(structNode, makeExpectErr(token, TokField, TokMessage, TokRBrace))
			if token.Kind == TokEof {
				return struct{}{}
			}
		}
	}
}

func (p *Parser) propagateMemberNode(nodeKind NodeKind, memberNode *MemberNode, parseErr ParseError) struct{} {
	memberNode.End = parseErr.actualToken.End
	memberNode.Poisoned = true

	parseErr.addKind(nodeKind)
	p.skipUntilSentinel()
	p.emitError(parseErr)

	return struct{}{}
}

func (p *Parser) propagateField(fieldNode *MemberNode, parseErr ParseError) struct{} {
	return p.propagateMemberNode(FieldNodeKind, fieldNode, parseErr)
}

func (p *Parser) parseField(fieldNode *MemberNode) struct{} {
	var token Token
	var err ParseError

	token = p.next()
	fieldNode.Begin = token.Begin

	switch token.Kind {
	case TokRequired:
		fieldNode.Modifier = Required
	case TokOptional:
		fieldNode.Modifier = Optional
	case TokDeprecated:
		fieldNode.Modifier = Deprecated
	default:
		return p.propagateField(fieldNode, makeExpectErr(token, TokRequired, TokOptional, TokDeprecated))
	}

	token, err = p.expect(TokIden)
	if err.isPresent() {
		return p.propagateField(fieldNode, err)
	}
	fieldNode.Iden = token.Str

	tag, err := p.parseTag()
	if err.isPresent() {
		return p.propagateField(fieldNode, err)
	}
	fieldNode.Tag = tag

	typeNode, err := p.parseType()
	if err.isPresent() {
		return p.propagateField(fieldNode, err)
	}
	fieldNode.LeftType = typeNode

	token = p.peek()
	switch token.Kind {
	case TokEqual:
		p.parseValue(&fieldNode.DefaultValue)
	case TokSemicolon: // skip parsing if default value is not provided
	default:
		return p.propagateField(fieldNode, makeExpectErr(token, TokEqual, TokSemicolon))
	}

	firstToken, ok := p.eatWhile(TokSemicolon)
	if !ok {
		return p.propagateField(fieldNode, makeExpectErr(firstToken, TokSemicolon))
	}
	fieldNode.End = firstToken.End

	return struct{}{}
}

func (p *Parser) propagateValue(valueNode *ValueNode, parseErr ParseError) struct{} {
	valueNode.End = parseErr.actualToken.End
	valueNode.Poisoned = true

	parseErr.addKind(ValueNodeKind)
	p.skipUntilFieldSentinel()
	p.emitError(parseErr)

	return struct{}{}
}

func (p *Parser) parseValue(valueNode *ValueNode) struct{} {
	var token Token

	token, err := p.expect(TokEqual)
	if err.isPresent() {
		panic(fmt.Sprintf("assertion error: in value node: %s", err))
	}
	valueNode.Begin = token.Begin

	token = p.peek()
	switch token.Kind {
	case TokInteger:
		valueNode.Kind, valueNode.Int = IntInstanceKind, token.Int
		p.eat()
	case TokFloat:
		valueNode.Kind, valueNode.Float64 = Float64InstanceKind, token.Float64
		p.eat()
	case TokString:
		str, err := p.parseString(&token)
		if err.isPresent() {
			return p.propagateValue(valueNode, err)
		}
		valueNode.Kind, valueNode.Str = StringInstanceKind, str
	default:
		return p.propagateValue(valueNode, makeExpectErr(token, TokInteger, TokFloat, TokString))
	}
	valueNode.End = token.End

	return struct{}{}
}

func (p *Parser) parseUnion(unionNode *DefNode, name string, size uint64) struct{} {
	unionNode.Kind = UnionNodeKind
	unionNode.Iden = name
	unionNode.Size = size

	token, err := p.expect(TokUnion)
	if err.isPresent() {
		panic(fmt.Sprintf("assertion error: in union: %s", err))
	}
	unionNode.Begin = token.Begin

	typeParams, err := p.parseTypeParams()
	if err.isPresent() {
		return p.propagateDefNode(unionNode, err)
	}
	unionNode.TypeParams = typeParams

	if _, err := p.expect(TokLBrace); err.isPresent() {
		return p.propagateDefNode(unionNode, err)
	}

	for {
		token := p.next()
		switch token.Kind {
		case TokIden:
			p.prev()
			var optionNode MemberNode
			p.parseOption(&optionNode)
			unionNode.Members = append(unionNode.Members, optionNode)
		case TokMessage:
			var messageNode DefNode
			if err := p.parseMessage(&messageNode); err.isPresent() {
				p.propagateDefNode(unionNode, err)
				continue
			}
			unionNode.LocalDefs = append(unionNode.LocalDefs, messageNode)
		case TokRBrace:
			unionNode.End = token.End
			return struct{}{}
		default:
			p.propagateDefNode(unionNode, makeExpectErr(token, TokOption, TokMessage, TokRBrace))
			if token.Kind == TokEof {
				return struct{}{}
			}
		}
	}
}

func (p *Parser) propagateOption(optionNode *MemberNode, parseErr ParseError) struct{} {
	return p.propagateMemberNode(OptionNodeKind, optionNode, parseErr)
}

func (p *Parser) parseOption(optionNode *MemberNode) struct{} {
	var token Token

	token, err := p.expect(TokIden)
	if err.isPresent() {
		return p.propagateOption(optionNode, err)
	}
	optionNode.Iden = token.Str

	ord, err := p.parseTagWithToken(&token)
	if err.isPresent() {
		return p.propagateOption(optionNode, err)
	}
	optionNode.Begin = token.Begin
	optionNode.Tag = ord

	typeNode, err := p.parseType()
	if err.isPresent() {
		return p.propagateOption(optionNode, err)
	}
	optionNode.LeftType = typeNode

	firstToken, ok := p.eatWhile(TokSemicolon)
	if !ok {
		return p.propagateOption(optionNode, makeExpectErr(firstToken, TokSemicolon))
	}
	optionNode.End = firstToken.End

	return struct{}{}
}

func (p *Parser) parseEnum(enumNode *DefNode, name string, size uint64) struct{} {
	enumNode.Kind = EnumNodeKind
	enumNode.Iden = name
	enumNode.Size = size

	token, err := p.expect(TokEnum)
	if err.isPresent() {
		panic(fmt.Sprintf("assertion error: in enum: %s", err))
	}
	enumNode.Begin = token.Begin

	if _, err := p.expect(TokLBrace); err.isPresent() {
		return p.propagateDefNode(enumNode, err)
	}

	for {
		token := p.next()
		switch token.Kind {
		case TokTag:
			p.prev()
			var enumCase MemberNode
			p.parseCase(&enumCase)
			enumNode.Members = append(enumNode.Members, enumCase)
		case TokRBrace:
			enumNode.End = token.End
			return struct{}{}
		default:
			p.propagateDefNode(enumNode, makeExpectErr(token, TokCase, TokRBrace))
			if token.Kind == TokEof {
				return struct{}{}
			}
		}
	}
}

func (p *Parser) propagateCase(enumCase *MemberNode, parseErr ParseError) struct{} {
	return p.propagateMemberNode(CaseNodeKind, enumCase, parseErr)
}

func (p *Parser) parseCase(enumCase *MemberNode) struct{} {
	var token Token

	tag, err := p.parseTagWithToken(&token)
	if err.isPresent() {
		return p.propagateCase(enumCase, err)
	}
	enumCase.Tag = tag
	enumCase.Begin = token.Begin

	token, err = p.expect(TokIden)
	if err.isPresent() {
		return p.propagateCase(enumCase, err)
	}
	enumCase.Iden = token.Str

	firstToken, ok := p.eatWhile(TokSemicolon)
	if !ok {
		return p.propagateCase(enumCase, makeExpectErr(firstToken, TokSemicolon))
	}
	enumCase.End = firstToken.End

	return struct{}{}
}

func (p *Parser) parseArraySize() (uint64, ParseError) {
	token := p.next()
	switch token.Kind {
	case TokInteger:
		size := token.Int.Uint64()
		if _, err := p.expect(TokRBrack); err.isPresent() {
			return 0, err
		}
		return size, ParseError{}
	case TokRBrack:
		return 0, ParseError{}
	default:
		return 0, makeExpectErr(token, TokInteger, TokRBrack)
	}
}

func (p *Parser) parseTypeParams() ([]string, ParseError) {
	var typeParams []string

	if p.peek().Kind != TokLParen {
		return typeParams, ParseError{}
	}
	p.eat()

	for {
		if p.peek().Kind == TokRParen {
			break
		}
		token, err := p.expect(TokIden)
		if err.isPresent() {
			return nil, err
		}
		typeParams = append(typeParams, token.Str)

		if p.peek().Kind != TokComma {
			break
		}
		p.eat()
	}

	if _, err := p.expect(TokRParen); err.isPresent() {
		return nil, err
	}
	return typeParams, ParseError{}
}

func (p *Parser) parseTypeArgs(token *Token) ([]TypeNode, ParseError) {
	var typeArgs []TypeNode

	if p.peek().Kind != TokLParen {
		return typeArgs, ParseError{}
	}
	p.eat()

	for {
		if p.peek().Kind == TokRParen {
			break
		}
		typeNode, err := p.parseType()
		if err.isPresent() {
			return nil, err
		}
		typeArgs = append(typeArgs, typeNode)

		if p.peek().Kind != TokComma {
			break
		}
		p.eat()
	}

	if _, err := p.expect(TokRParen); err.isPresent() {
		*token = p.next()
		return nil, err
	}

	return typeArgs, ParseError{}
}

func (p *Parser) parseType() (TypeNode, ParseError) {
	// each element of the array is a nested array index
	var array []uint64
	var arrTokenBegin Token

	for {
		token := p.next()
		switch token.Kind {
		case TokLBrack:
			if arrTokenBegin.Kind == TokUnknown {
				// if begin token is unset, we know we're at the first array token
				arrTokenBegin = token
			}
			size, err := p.parseArraySize()
			if err.isPresent() {
				err.addKind(TypeNodeKind)
				// don't emit the error, caller will handle this
				return TypeNode{}, err
			}
			array = append(array, size)
		case TokIden:
			name := token.Str

			// select the beginning token depending on whether the type ref is an array or not
			var tokenBegin = token
			if arrTokenBegin.Kind != TokUnknown {
				tokenBegin = arrTokenBegin
			}
			tokenEnd := tokenBegin

			typeArgs, err := p.parseTypeArgs(&tokenEnd)
			if err.isPresent() {
				err.addKind(TypeNodeKind)
				// don't emit the error, caller will handle this
				return TypeNode{}, err
			}
			node := TypeNode{
				Iden:      name,
				Array:     array,
				TypeArgs:  typeArgs,
				Positions: Positions{Begin: tokenBegin.Begin, End: tokenEnd.End},
			}
			return node, ParseError{}
		default:
			return TypeNode{}, makeExpectErr(token, TokTypeRef)
		}
	}
}

func (p *Parser) parseTag() (uint64, ParseError) {
	var token Token
	return p.parseTagWithToken(&token)
}

// parseTagWithToken 'writes back' the token it reads to the caller for further processing
func (p *Parser) parseTagWithToken(token *Token) (uint64, ParseError) {
	t, err := p.expect(TokTag)
	if err.isPresent() {
		return 0, err
	}
	*token = t
	if len(token.Str) < 2 {
		panic("assertion error: a tag should have at least 2 characters")
	}
	tag := token.Int.Uint64()
	return tag, ParseError{}
}

func (p *Parser) parseService(svcNode *DefNode) struct{} {
	svcNode.Kind = ServiceNodeKind

	token, err := p.expect(TokService)
	if err.isPresent() {
		panic(fmt.Sprintf("assertion error: in service: %s", err))
	}

	token, err = p.expect(TokIden)
	if err.isPresent() {
		return p.propagateDefNode(svcNode, err)
	}
	svcNode.Iden = token.Str

	if _, err := p.expect(TokLBrace); err.isPresent() {
		return p.propagateDefNode(svcNode, err)
	}

	for {
		token = p.next()
		switch token.Kind {
		case TokRpc:
			p.prev()

			var rpcNode MemberNode
			p.parseRpc(&rpcNode)

			svcNode.Members = append(svcNode.Members, rpcNode)
		case TokMessage:
			var messageNode DefNode

			if err := p.parseMessage(&messageNode); err.isPresent() {
				p.propagateDefNode(svcNode, err)
				continue
			}
			svcNode.LocalDefs = append(svcNode.LocalDefs, messageNode)
		case TokRBrace:
			return struct{}{}
		default:
			p.propagateDefNode(svcNode, makeExpectErr(token, TokRpc, TokMessage, TokRBrace))
			if token.Kind == TokEof {
				return struct{}{}
			}
		}
	}
}

func (p *Parser) propagateRpc(rpcNode *MemberNode, parseErr ParseError) struct{} {
	return p.propagateMemberNode(RpcNodeKind, rpcNode, parseErr)
}

func (p *Parser) parseRpc(rpcNode *MemberNode) struct{} {
	token, err := p.expect(TokRpc)
	if err.isPresent() {
		panic(fmt.Sprintf("assertion error: in rpc: %s", err))
	}
	rpcNode.Begin = token.Begin

	tag, err := p.parseTag()
	if err.isPresent() {
		return p.propagateRpc(rpcNode, err)
	}
	rpcNode.Tag = tag

	if token, err = p.expect(TokIden); err.isPresent() {
		return p.propagateRpc(rpcNode, err)
	}
	rpcNode.Iden = token.Str

	if _, err = p.expect(TokLParen); err.isPresent() {
		return p.propagateRpc(rpcNode, err)
	}

	typeNode, err := p.parseType()
	if err.isPresent() {
		return p.propagateRpc(rpcNode, err)
	}
	rpcNode.LeftType = typeNode

	if err = p.expectChain(TokRParen, TokReturns, TokLParen); err.isPresent() {
		return p.propagateRpc(rpcNode, err)
	}

	typeNode, err = p.parseType()
	if err.isPresent() {
		return p.propagateRpc(rpcNode, err)
	}
	rpcNode.RightType = typeNode

	token, err = p.expect(TokRParen)
	if err.isPresent() {
		return p.propagateRpc(rpcNode, err)
	}
	rpcNode.End = token.End

	return struct{}{}
}
