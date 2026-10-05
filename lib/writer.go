package lib

import (
	"io"
	"math/big"
)

type BitWriter struct {
	byteOffset int
	bitOffset  uint8
	w          io.Writer
}

func (w *BitWriter) WriteInt64(i64 int64, n int) {

}

func (w *BitWriter) WriteString(s string) {

}

func (w *BitWriter) WriteFloat32(f32 float32) {

}

func (w *BitWriter) WriteFloat64(f64 float64) {

}

func (w *BitWriter) WriteBigInt(bint big.Int, n int) {

}

func (w *BitWriter) WriteBool(b bool) {

}