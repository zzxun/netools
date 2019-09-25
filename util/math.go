package util

import (
	"encoding/binary"
	"hash/fnv"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// CIDRRange a cidr to long start ip and end.
func CIDRRange(cidr string) (start uint32, end uint32, err error) {
	// 192.168.0.1/16
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0, 0, nil
	}

	ip := binary.BigEndian.Uint32(ipNet.IP)
	rest := binary.BigEndian.Uint32(ipNet.Mask) ^ 0xffffffff
	return ip + 1, ip + rest - 1, nil
}

// LongIPv4 convert string to uint32
func LongIPv4(ip string) uint32 {
	ip = strings.Split(ip, ":")[0]
	longIP := net.ParseIP(ip)
	if longIP == nil {
		return 0
	}
	return binary.BigEndian.Uint32(longIP[12:])
}

// Hash calculate string hashcode
func Hash(qname string) uint32 {
	h := fnv.New32()
	for i := range qname {
		c := qname[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		h.Write([]byte{c})
	}

	return h.Sum32()
}

// ReverseQname a qname
func ReverseQname(s string) string {
	qname := dns.SplitDomainName(s)
	// reverse
	for i, j := 0, len(qname)-1; i < j; i, j = i+1, j-1 {
		qname[i], qname[j] = qname[j], qname[i]
	}

	return "." + strings.Join(qname, ".")
}

// ReverseQnames reverse names and make to fqdn
func ReverseQnames(qname []string) string {
	qn := make([]string, len(qname))
	for i := 0; i < len(qname); i++ {
		qn[i] = qname[len(qname)-1-i]
	}

	return strings.Join(qn, ".") + "."
}

// RReverseQname a ReverseQname
func RReverseQname(s string) string {
	qname := dns.SplitDomainName(s)
	// reverse
	for i, j := 0, len(qname)-1; i < j; i, j = i+1, j-1 {
		qname[i], qname[j] = qname[j], qname[i]
	}

	return strings.Join(qname, ".")
}

// From https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-golang
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

// RandStringBytesMaskImprSrc return a random []byte with length n
func RandStringBytesMaskImprSrc(n int) []byte {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return b
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

// Contains return true if element in array
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Shuffle string slice
func Shuffle(src []string) []string {
	dest := make([]string, len(src))
	perm := rand.Perm(len(src))
	for i, v := range perm {
		dest[v] = src[i]
	}
	return dest
}
