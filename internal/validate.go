package internal

import (
	"slices"
	"unicode"
)

func Validate(nodes []DefNode) []ValidateErr {
	validator := &Validator{}

	validator.transformDefList(nodes, nil)
	validator.validateDefList(nodes)

	return validator.errs
}

type Validator struct {
	errs []ValidateErr
}

func (v *Validator) emit(err ValidateErr) {
	v.errs = append(v.errs, err)
}

type TypeParamStack struct {
	prev *TypeParamStack
	m    map[string]*TypeNode
}

func (t *TypeParamStack) insert(iden string, node *TypeNode) ValidateErr {
	if _, exists := t.m[iden]; exists {
		return makeRedefErr(TypeNodeKind, node.Positions, iden)
	}
	t.m[iden] = node
	return ValidateErr{}
}

func (t *TypeParamStack) resolve(iden string) *TypeNode {
	table := t
	for table != nil {
		node, ok := table.m[iden]
		if ok {
			return node
		}
		table = table.prev
	}
	return nil
}

type TypeDefStack struct {
	prev *TypeDefStack
	m    map[string]*DefNode
}

func makeTypeDefStack(prev *TypeDefStack) *TypeDefStack {
	return &TypeDefStack{m: make(map[string]*DefNode), prev: prev}
}

func (t *TypeDefStack) insert(iden string, node *DefNode) ValidateErr {
	if _, exists := t.m[iden]; exists {
		return makeRedefErr(node.Kind, node.Positions, iden)
	}
	t.m[iden] = node
	return ValidateErr{}
}

func (t *TypeDefStack) resolve(iden string) *DefNode {
	table := t
	for table != nil {
		node, ok := table.m[iden]
		if ok {
			return node
		}
		table = table.prev
	}
	return nil
}

func resolveIden(node *DefNode, iden string) *DefNode {
	// resolving the identifier relative to the definition stack and then local definitions
	resolvedNode := node.DefStack.resolve(iden)
	if resolvedNode != nil {
		return resolvedNode
	}
	for i := range node.LocalDefs {
		localDef := &node.LocalDefs[i]
		if localDef.Iden == iden {
			return localDef
		}
	}
	return nil
}

func (v *Validator) transformDefList(nodes []DefNode, prevStack *TypeDefStack) {
	stack := makeTypeDefStack(prevStack)

	for i := range nodes {
		node := &nodes[i]
		if !node.Kind.isTypeDef() {
			continue
		}

		if err := stack.insert(node.Iden, node); err.isPresent() {
			v.emit(err)
		}
		node.DefStack = stack

		kind := node.Kind.MemberKind()
		for i := range node.Members {
			node := &node.Members[i]
			switch kind {
			case FieldNodeKind, OptionNodeKind:
				v.transformType(&node.LeftType)
			case RpcNodeKind:
				v.transformType(&node.LeftType)
				v.transformType(&node.RightType)
			default:
			}
		}
		slices.SortFunc(node.Members, func(n1, n2 MemberNode) int { return int(n1.Tag - n2.Tag) })

		v.transformDefList(node.LocalDefs, stack)
	}
}

func (v *Validator) transformType(node *TypeNode) {
	node.TypeValue = makeTypeValue(node.Iden)
	for i := range node.TypeArgs {
		v.transformType(&node.TypeArgs[i])
	}
}

func (v *Validator) validateDefList(nodes []DefNode) {
	for i := range nodes {
		defNode := &nodes[i]
		memberKind := defNode.Kind.MemberKind()
		if !defNode.Kind.isTypeDef() {
			continue
		}

		if !isMessageNameValid(defNode.Iden) {
			v.emit(makeNameErr(defNode.Kind, defNode.Positions, defNode.Iden))
		}

		// invariant: def.Members is sorted by tag
		expTag := uint64(1)
		for _, memberNode := range defNode.Members {
			tag := memberNode.Tag
			if tag != expTag {
				v.emit(makeTagErr(memberKind, memberNode.Positions, expTag, tag))
				break
			}
			expTag++
		}
		for i, mem := range defNode.Members {
			rightIden := mem.Iden
			// note(Joseph): def.Members is expected to be small, so brute force search is acceptable
			for j := i - 1; j >= 0; j-- {
				leftIden := defNode.Members[j].Iden
				if rightIden == leftIden {
					v.emit(makeRedefErr(memberKind, mem.Positions, rightIden))
				}
			}
		}

		if defNode.Kind == EnumNodeKind {
			continue
		}

		for _, mem := range defNode.Members {
			switch memberKind {
			case FieldNodeKind, OptionNodeKind:
				v.validateType(memberKind, mem.LeftType, defNode)
			case RpcNodeKind:
				v.validateType(memberKind, mem.LeftType, defNode)
				v.validateType(memberKind, mem.RightType, defNode)
			}
		}

		v.validateDefList(defNode.LocalDefs)
	}
}

func (v *Validator) validateType(kind NodeKind, node TypeNode, parent *DefNode) {
	if node.TypeValue.Primitive {
		return
	}
	iden := node.TypeValue.Native()
	def := resolveIden(parent, iden)
	if def == nil {
		v.emit(makeUndefErr(kind, node.Positions, iden))
		return
	}
	if len(node.TypeArgs) != len(def.TypeParams) {
		v.emit(makeTypeArgErr(kind, node.Positions, def.TypeParams, node.TypeArgs))
		return
	}
	for _, arg := range node.TypeArgs {
		v.validateType(kind, arg, parent)
	}
}

func isMessageNameValid(name string) bool {
	for i, c := range name {
		if i == 0 && unicode.IsLower(c) {
			return false
		}
		if !unicode.IsLetter(c) && !unicode.IsNumber(c) {
			return false
		}
	}
	return true
}
