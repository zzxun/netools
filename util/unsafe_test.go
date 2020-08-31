package util

import (
	"testing"
)

func Test_ByteString(t *testing.T) {
	var x = []byte("Hello World!")
	var y = Bytes2String(x)
	var z = string(x)

	if y != z {
		t.Fail()
	}

	var x2 = StringToBytes(y)
	if len(x2) != len(x) {
		t.Fail()
	}

	for i := range x {
		if x[i] != x2[i] {
			t.Fail()
		}
	}
}

func Benchmark_Normal(b *testing.B) {
	var x = []byte("Hello World!")
	for i := 0; i < b.N; i++ {
		_ = string(x)
	}
}

func Benchmark_ByteString(b *testing.B) {
	var x = []byte("Hello World!")
	for i := 0; i < b.N; i++ {
		_ = Bytes2String(x)
	}
}
