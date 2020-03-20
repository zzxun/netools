package util

import "unsafe"

// Bytes2String convert b to string without copy.
func Bytes2String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToBytes convert string to b without copy.
func StringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}
