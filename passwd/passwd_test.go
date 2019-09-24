package passwd

import (
	"bytes"
	"testing"
)

var file = `
# for etcd
etcd_root_user = x
etcd_root_passwd=   xx#x
`

func TestReadPasswds(t *testing.T) {
	m := ReadPasswds(bytes.NewReader([]byte(file)))

	if len(m) != 2 {
		t.Errorf("file passwds len should be 4, but %d", len(m))
	}
	if c := m["etcd_root_user"]; c != "x" {
		t.Errorf("etcd_root_user should be %s, but %s", "x", c)
	}
	if c := m["etcd_root_passwd"]; c != "xx#x" {
		t.Errorf("etcd_root_passwd should be %s, but %s", "xx", c)
	}
}
