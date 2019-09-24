package passwd

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"strings"
)

// ReadPassFile read conf file, format as xxx=xxx
func ReadPassFile(file string) (map[string]string, error) {
	f, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return ReadPasswds(bytes.NewReader(f)), nil
}

// ReadPasswds from Reader
func ReadPasswds(r io.Reader) map[string]string {
	pds := make(map[string]string)
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		lls := strings.Split(strings.TrimSpace(line), "=")
		if len(lls) < 2 {
			continue
		}
		// only take the first '=', as 'a=b=c#x' => map ['a', 'b=c#2']
		pds[strings.TrimSpace(lls[0])] = strings.TrimSpace(strings.Join(lls[1:], "="))
	}
	return pds
}
