package routing

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/Jcd1230/socks-router/log"
)

// first match wins
type Map struct {
	Routes []Route
}

func (m Map) Match(network string, address AddressDetails) *Target {
	for _, route := range m.Routes {
		if target := route.Match(network, address); nil != target {
			return target
		}
	}
	return nil
}

func (m Map) MatchCommand(network string, address AddressDetails) string {
	for _, route := range m.Routes {
		if target := route.Match(network, address); nil != target {
			return route.Command()
		}
	}
	return ""
}

func (m Map) Dial(network, address string) (c net.Conn, err error) {
	if ad, err := ParseAddress(address); nil != err {
		return nil, err
	} else {
		var dial func(network, address string) (c net.Conn, err error)
		var desc string
		if target := m.Match(network, *ad); nil != target {
			desc = fmt.Sprintf("to %v over %v", address, target.Name)
			dial = target.Dialer.Dial
		} else {
			desc = fmt.Sprintf("directly to %v", address)
			dial = DirectTarget.Dialer.Dial
		}
		log.Access.Printf("connecting %v", desc)
		if conn, err := dial(network, address); nil == err {
			return conn, nil
		}
		// First try failed. Run command if present and try once more.
		if command := m.MatchCommand(network, *ad); "" != command {
			var commandline= strings.Fields(command)
			log.Info.Println("Running command: "+command)
			if out, err := exec.Command(commandline[0], commandline[1:]...).CombinedOutput(); nil != err {
				log.Error.Printf("Failed to run retry-command: %v: %v", command, err)
				return nil, err
			} else {
				log.Info.Printf("Command output:\n%s", out)
			}
			if conn,err := dial(network, address); nil == err {
				return conn, nil
			} else {
				log.Error.Printf("Failed to connect %v: %v", desc, err)
			}
		}
		return nil, err
	}
}

func ReadMap(r io.Reader) (*Map, error) {
	var m Map

	scanner := bufio.NewScanner(r)
	linenum := 0

	for scanner.Scan() {
		line := scanner.Text()
		linenum += 1
		if r, err := ParseRoute(line); nil != err {
			return nil, fmt.Errorf("Error in config line %v: %v", linenum, err)
		} else if nil != r {
			m.Routes = append(m.Routes, r)
		}
	}

	return &m, nil
}

func ReadMapFile(filename string) (*Map, error) {
	if f, err := os.Open(filename); nil != err {
		return nil, fmt.Errorf("Couldn't open file %q: %v", filename, err)
	} else {
		defer f.Close()
		return ReadMap(f)
	}
}
