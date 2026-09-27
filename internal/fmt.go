package internal

import (
	"fmt"
	"strings"
)

type FmtAstConfig struct {
	ShouldPrintLines bool
}

var DefaultFmtAstConfig = FmtAstConfig{ShouldPrintLines: false}

func FmtAst(nodes []DefNode) string {
	return FmtAstWithConfig(nodes, &DefaultFmtAstConfig)
}

func FmtAstWithConfig(nodes []DefNode, config *FmtAstConfig) string {
	if config == nil {
		config = &DefaultFmtAstConfig
	}

	state := &FmtAstState{
		sb:         strings.Builder{},
		lineNumber: 1,
	}
	if !config.ShouldPrintLines {
		state.lineNumber = -1
	}

	addLine(state)
	FmtDefList(state, nodes, 0)

	return state.sb.String()
}

type FmtAstState struct {
	sb         strings.Builder
	lineNumber int
}

func fmtIndents(state *FmtAstState, depth int) {
	for range depth {
		state.sb.WriteString("\t")
	}
}

func addLine(state *FmtAstState) {
	if state.lineNumber < 0 {
		state.sb.WriteString("\n")
	} else {
		prefix := "\n"
		if state.lineNumber == 1 {
			prefix = ""
		}
		// note(Joseph): if the number gets over 4 digits, this will look weird.
		fmt.Fprintf(&state.sb, "%s%4d ", prefix, state.lineNumber)
		state.lineNumber++
	}
}

func FmtDefList(state *FmtAstState, nodes []DefNode, depth int) {
	for _, node := range nodes {
		if depth != 0 && node.Kind.isTypeDef() {
			addLine(state)
		}
		switch node.Kind {
		case ImportNodeKind:
			fmt.Fprintf(&state.sb, "import \"%s\"", node.StrValue)
			addLine(state)
		case PropertyNodeKind:
			fmt.Fprintf(&state.sb, "%s \"%s\"", node.Iden, node.StrValue)
			addLine(state)
		case StructNodeKind:
			fmtIndents(state, depth)
			fmt.Fprintf(&state.sb, "message %s struct ", node.Iden)
			FmtTypeParams(state, node.TypeParams)
			state.sb.WriteString("{")
			addLine(state)
			FmtMemberList(state, node.Kind.MemberKind(), node.Members, depth+1)
			FmtDefList(state, node.LocalDefs, depth+1)
			fmtIndents(state, depth)
			state.sb.WriteString("}")
			addLine(state)
		case UnionNodeKind:
			fmtIndents(state, depth)
			fmt.Fprintf(&state.sb, "message %s union ", node.Iden)
			FmtTypeParams(state, node.TypeParams)
			state.sb.WriteString("{")
			addLine(state)
			FmtMemberList(state, node.Kind.MemberKind(), node.Members, depth+1)
			FmtDefList(state, node.LocalDefs, depth+1)
			fmtIndents(state, depth)
			state.sb.WriteString("}")
			addLine(state)
		case EnumNodeKind:
			fmtIndents(state, depth)
			fmt.Fprintf(&state.sb, "message %s enum ", node.Iden)
			FmtTypeParams(state, node.TypeParams)
			state.sb.WriteString("{")
			addLine(state)
			FmtMemberList(state, node.Kind.MemberKind(), node.Members, depth+1)
			fmtIndents(state, depth)
			state.sb.WriteString("}")
			addLine(state)
		case ServiceNodeKind:
			fmtIndents(state, depth)
			fmt.Fprintf(&state.sb, "service %s {", node.Iden)
			addLine(state)
			FmtMemberList(state, node.Kind.MemberKind(), node.Members, depth+1)
			FmtDefList(state, node.LocalDefs, depth+1)
			fmtIndents(state, depth)
			state.sb.WriteString("}")
			addLine(state)
		default:
		}
	}
}

func FmtTypeParams(state *FmtAstState, typeParams []string) {
	for i, param := range typeParams {
		if i == 0 {
			state.sb.WriteString("(")
		}
		fmt.Fprintf(&state.sb, "%s", param)
		if i == len(typeParams)-1 {
			state.sb.WriteString(") ")
		} else {
			state.sb.WriteString(", ")
		}
	}
}

func FmtType(state *FmtAstState, node TypeNode) {
	for _, size := range node.Array {
		if size != 0 {
			fmt.Fprintf(&state.sb, "[%d]", size)
		} else {
			state.sb.WriteString("[]")
		}
	}
	state.sb.WriteString(node.Iden)
	FmtTypeArgs(state, node.TypeArgs)
}

func FmtTypeArgs(state *FmtAstState, typeArgs []TypeNode) {
	for i, arg := range typeArgs {
		if i == 0 {
			state.sb.WriteString("(")
		}
		FmtType(state, arg)
		if i == len(typeArgs)-1 {
			state.sb.WriteString(")")
		} else {
			state.sb.WriteString(", ")
		}
	}
}

func FmtMemberList(state *FmtAstState, kind NodeKind, nodes []MemberNode, depth int) {
	for _, node := range nodes {
		switch kind {
		case FieldNodeKind:
			fmtIndents(state, depth)
			fmt.Fprintf(&state.sb, "%s %s @%d ", node.Modifier, node.Iden, node.Tag)
			FmtType(state, node.LeftType)
			state.sb.WriteString(";")
			addLine(state)
		case CaseNodeKind:
			fmtIndents(state, depth)
			fmt.Fprintf(&state.sb, "@%d %s;", node.Tag, node.Iden)
			addLine(state)
		case OptionNodeKind:
			fmtIndents(state, depth)
			fmt.Fprintf(&state.sb, "%s @%d ", node.Iden, node.Tag)
			FmtType(state, node.LeftType)
			state.sb.WriteString(";")
			addLine(state)
		case RpcNodeKind:
			fmtIndents(state, depth)
			fmt.Fprintf(&state.sb, "rpc @%d %s(", node.Tag, node.Iden)
			FmtType(state, node.LeftType)
			state.sb.WriteString(") returns (")
			FmtType(state, node.RightType)
			state.sb.WriteString(")")
			addLine(state)
		default:
		}
	}
}
