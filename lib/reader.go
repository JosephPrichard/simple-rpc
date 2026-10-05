package lib

import (
	"io"
	"math/big"
	"strings"
)

type BitReader struct {
	byteOffset int
	bitOffset  uint8
	r          io.Reader
}

func (r *BitReader) ReadInt64(n int) int64 {
	var i64 int64

	return i64
}

func (r *BitReader) ReadBigInt(n int) big.Int {
	var bint big.Int

	return bint
}

func (r *BitReader) ReadFloat32() float32 {
	var f32 float32

	return f32
}

func (r *BitReader) ReadFloat64() float64 {
	var f64 float64

	return f64
}

func (r *BitReader) ReadString() string {
	var sb strings.Builder

	return sb.String()
}

func (r *BitReader) ReadBool() bool {
	var b bool

	return b
}
