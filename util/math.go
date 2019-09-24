package util

import (
	"math/rand"
)

// Shuffle string slice
func Shuffle(src []string) []string {
	dest := make([]string, len(src))
	perm := rand.Perm(len(src))
	for i, v := range perm {
		dest[v] = src[i]
	}
	return dest
}

// NextPow2 find small x for 2^x > a.
func NextPow2(a int) int {
	var rval = 1
	// rval<<=1 Is A Prettier Way Of Writing rval*=2;
	for rval < a {
		rval <<= 1
	}
	return rval
}
