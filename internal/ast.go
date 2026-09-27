package internal

import (
	"fmt"
	"math/big"
	"slices"
)

type NodeKind int

const (
	NoNodeKind NodeKind = iota
	PropertyNodeKind
	ImportNodeKind
	MessageNodeKind
	StructNodeKind
	EnumNodeKind
	UnionNodeKind
	CaseNodeKind
	FieldNodeKind
	OptionNodeKind
	ServiceNodeKind
	RpcNodeKind
	TypeNodeKind
	ValueNodeKind
)

func (kind NodeKind) String() string {
	switch kind {
	case NoNodeKind:
		return "unknown"
	case PropertyNodeKind:
		return "property"
	case ImportNodeKind:
		return "import"
	case MessageNodeKind:
		return "message"
	case StructNodeKind:
		return "struct"
	case EnumNodeKind:
		return "enum"
	case UnionNodeKind:
		return "union"
	case FieldNodeKind:
		return "struct field"
	case CaseNodeKind:
		return "enum case"
	case OptionNodeKind:
		return "union option"
	case ServiceNodeKind:
		return "service"
	case RpcNodeKind:
		return "rpc"
	case TypeNodeKind:
		return "type"
	case ValueNodeKind:
		return "value"
	default:
		panic(fmt.Sprintf("assertion error: string func: unknown NodeKind: %d", kind))
	}
}

type Modifier int

const (
	Optional Modifier = iota
	Required
	Deprecated
)

func (m Modifier) String() string {
	var modStr string
	switch m {
	case Required:
		modStr = "required"
	case Optional:
		modStr = "optional"
	case Deprecated:
		modStr = "deprecated"
	}
	return modStr
}

type Positions struct {
	Begin int
	End   int
}

func (r *Positions) Offset() string {
	if r.Begin == r.End {
		return fmt.Sprintf("%d:", r.Begin)
	}

	return fmt.Sprintf("%d:%d:", r.Begin, r.End)
}

func (r *Positions) ClearPositions() {
	r.End = 0
	r.Begin = 0
}

var DefNodeKinds = []NodeKind{StructNodeKind, UnionNodeKind, EnumNodeKind, ServiceNodeKind}

func (kind NodeKind) isTypeDef() bool {
	return slices.Contains(DefNodeKinds, kind)
}

func (kind NodeKind) MemberKind() NodeKind {
	switch kind {
	case StructNodeKind:
		return FieldNodeKind
	case UnionNodeKind:
		return OptionNodeKind
	case EnumNodeKind:
		return CaseNodeKind
	case ServiceNodeKind:
		return RpcNodeKind
	default:
		return NoNodeKind
	}
}

type DefNode struct {
	// L1 (parse)
	Positions
	Kind       NodeKind
	Poisoned   bool
	Iden       string       // Every node kind has an identifier
	StrValue   string       // Property/Import value
	Members    []MemberNode // Elements of Structs/Unions/Enums/Services
	TypeParams []string     // Parameterization for Structs/Unions/Enums/Services
	LocalDefs  []DefNode    // Recursively definitions for Structs/Unions/Enums/Services
	Size       uint64       // Enum/Union sizes

	// L2 (transform)
	DefStack *TypeDefStack
}

type MemberNode struct {
	// L1 (parse)
	Positions
	Poisoned     bool
	Iden         string
	Tag          uint64    // Tag used for data node kinds (Field/Option/Enum)
	Modifier     Modifier  // Prefix value for Fields only
	LeftType     TypeNode  // Argument type for Rpc, Primary type for Option and Field
	RightType    TypeNode  // Return type for Rpc
	DefaultValue ValueNode // Literal value for Field
}

type TypeInstanceKind int

const (
	NoInstanceKind TypeInstanceKind = iota
	StringInstanceKind
	IntInstanceKind
	Float64InstanceKind
)

type ValueNode struct {
	// L1 (parse)
	Positions
	Poisoned bool
	Kind     TypeInstanceKind
	Str      string
	Int      big.Int
	Float64  float64
}

type TypeNode struct {
	// L1 (parse)
	Positions
	Iden     string
	TypeArgs []TypeNode
	Array    []uint64

	// L2 (transform)
	TypeValue Type
}
