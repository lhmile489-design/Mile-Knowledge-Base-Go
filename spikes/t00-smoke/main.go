// T00 smoke checks: mygo.Secrets against a throwaway service name and
// netproxy against loopback. Prints one JSON object; exit 1 if a check fails.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
)

type result struct {
	Secrets  secretsResult  `json:"secrets"`
	Netproxy netproxyResult `json:"netproxy"`
	OK       bool           `json:"ok"`
}

type secretsResult struct {
	Service     string `json:"service"`
	Set         string `json:"set"`
	Get         string `json:"get"`
	Oversize    string `json:"oversize"`
	AfterDelete string `json:"afterDelete"`
}

type netproxyResult struct {
	Loopback    string `json:"loopback"`
	Localhost   string `json:"localhost"`
	PACAndSOCKS string `json:"pacAndSocks"`
}

func main() {
	r := result{}
	ok := true

	name := make([]byte, 8)
	if _, err := rand.Read(name); err != nil {
		fail(err)
	}
	service := "mkb-spike-" + hex.EncodeToString(name)
	module := mygo.Secrets.Service(service)
	r.Secrets = secretsResult{Service: service}

	if err := module.Set("probe", []byte("hello")); err != nil {
		r.Secrets.Set = "error: " + err.Error()
		ok = false
	} else {
		r.Secrets.Set = "ok"
	}
	if value, err := module.Get("probe"); err != nil {
		r.Secrets.Get = "error: " + err.Error()
		ok = false
	} else {
		r.Secrets.Get = fmt.Sprintf("ok len=%d", len(value))
	}
	if err := module.Set("big", []byte(strings.Repeat("x", mygo.MaxSecretSize+1))); err != nil {
		if errors.Is(err, errors.ErrUnsupported) {
			r.Secrets.Oversize = "unsupported"
		} else {
			r.Secrets.Oversize = "rejected: " + err.Error()
		}
	} else {
		r.Secrets.Oversize = "ACCEPTED (unexpected)"
		ok = false
	}
	if err := module.Delete("probe"); err != nil {
		r.Secrets.AfterDelete = "delete error: " + err.Error()
		ok = false
	} else if _, err := module.Get("probe"); err != nil {
		r.Secrets.AfterDelete = "gone: " + err.Error()
	} else {
		r.Secrets.AfterDelete = "STILL PRESENT (unexpected)"
		ok = false
	}
	_ = module.Delete("big")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = io.WriteString(w, "pong")
	}))
	defer server.Close()
	client := &http.Client{Transport: netproxy.Transport(), Timeout: 5 * time.Second}
	probe := func(url string) string {
		resp, err := client.Get(url)
		if err != nil {
			ok = false
			return "error: " + err.Error()
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return fmt.Sprintf("ok %d %s", resp.StatusCode, string(body))
	}
	r.Netproxy.Loopback = probe(server.URL) // 127.0.0.1
	port := server.URL[strings.LastIndex(server.URL, ":"):]
	r.Netproxy.Localhost = probe("http://localhost" + port)
	r.Netproxy.PACAndSOCKS = "not exercised: no PAC or SOCKS proxy configured on this machine; behavior per netproxy docs and tech design 2.1"

	r.OK = ok
	out, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(out))
	if !ok {
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
