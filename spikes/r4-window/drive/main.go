// drive sends newline-delimited JSON to the app's MYGO_AUTOMATION socket.
// Usage:
//
//	go run . '{"id":1,"method":"windows"}'
//	go run . -f request.json
//	echo '{"id":1,"method":"windows"}' | go run .
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

func dial(sock string) (net.Conn, error) {
	network := "unix"
	addr := sock
	if a, ok := strings.CutPrefix(sock, "tcp:"); ok {
		network, addr = "tcp", a
	}
	return net.DialTimeout(network, addr, 5*time.Second)
}

func main() {
	sock := os.Getenv("MYGO_AUTOMATION")
	if sock == "" {
		sock = "tcp:127.0.0.1:18081"
	}
	var lines []string
	if len(os.Args) > 2 && os.Args[1] == "-f" {
		b, err := os.ReadFile(os.Args[2])
		if err != nil {
			panic(err)
		}
		lines = []string{strings.TrimSpace(string(b))}
	} else if len(os.Args) > 1 {
		lines = os.Args[1:]
	} else {
		sc := bufio.NewScanner(os.Stdin)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if s := strings.TrimSpace(sc.Text()); s != "" {
				lines = append(lines, s)
			}
		}
	}
	c, err := dial(sock)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial:", err)
		os.Exit(2)
	}
	defer c.Close()
	r := bufio.NewReaderSize(c, 1<<20)
	for i, l := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			m = map[string]any{"id": i + 1, "method": l}
		}
		if _, ok := m["id"]; !ok {
			m["id"] = i + 1
		}
		b, _ := json.Marshal(m)
		c.SetDeadline(time.Now().Add(120 * time.Second))
		if _, err := c.Write(append(b, '\n')); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(2)
		}
		resp, err := r.ReadBytes('\n')
		if err != nil && err != io.EOF {
			fmt.Fprintln(os.Stderr, "read:", err)
			os.Exit(2)
		}
		fmt.Println(string(resp))
	}
}
