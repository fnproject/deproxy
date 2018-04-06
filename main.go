package main

import (
	"github.com/elazarl/goproxy"
	"log"
	"net/http"
	"os"
	"fmt"
	"io"
	"github.com/pelletier/go-toml"
	"regexp"
	"errors"
	"net"
	"os/exec"
	"strings"
)

type Rewrite struct {
	Package string
	Source  string
}
type Config struct {
	Rewrite []Rewrite
}

func LoadHandler(rdr io.Reader) (*Config, error) {
	dec := toml.NewDecoder(rdr)
	config := new(Config)
	err := dec.Decode(config)
	if err != nil {
		return nil, err
	}

	for _, r := range config.Rewrite {

		if r.Package == "" {
			return nil, errors.New("Invalid package name")
		}

		parts := hostRegex.FindStringSubmatch(r.Package)
		if len(parts) < 2 {
			return nil, fmt.Errorf("Invalid package %s - no host ", r.Package)
		}
		if r.Source == "" {
			return nil, fmt.Errorf("invalid source name for package %s", r.Package)
		}
	}
	for _, r := range config.Rewrite {
		for _, r2 := range config.Rewrite {
			if r != r2 && strings.HasPrefix(r.Package, r2.Package) {
				return nil, fmt.Errorf("invalid invalid rewrite %s->%s  %s is a prefix  of another rewrite %s->%s", r2.Package, r2.Source, r2.Package, r.Package, r.Source)

			}

		}
	}

	return config, err
}

var hostRegex = regexp.MustCompile("^([^/]+)/")

func (r *Rewrite) Host() string {
	parts := hostRegex.FindStringSubmatch(r.Package)
	return parts[1]
}

var verbose = false

func lg(msg string, args ... interface{}) {
	if verbose {
		log.Printf("DeProxy: "+msg, args...)
	}
}
func main() {
	if os.Getenv("DEPROXY_VERBOSE") != "" {
		verbose = true
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = verbose
	configFile := "Deproxy.toml"
	f, err := os.Open(configFile)
	if err != nil {
		log.Fatalf("Failed to open config file %s, %s", configFile, err.Error())
		return
	}
	config, err := LoadHandler(f)
	if err != nil {
		log.Fatalf("Failed to read config file %s, %s", configFile, err.Error())
		return
	}

	hosts := make(map[string]bool)

	for _, ir := range config.Rewrite {
		r := ir

		// always block ssl on hosts being rewritten
		lg("Reqwriting %s -> %s", r.Package, r.Source)
		if !hosts[r.Host()] {
			hosts[r.Host()] = true
			proxy.OnRequest(goproxy.ReqHostIs(r.Host() + ":443")).HandleConnect(goproxy.AlwaysReject)
		}

		proxy.OnRequest(goproxy.UrlHasPrefix(r.Package)).DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {

			substitution := fmt.Sprintf("%s git %s", r.Package, r.Source)
			vanityHtml := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
<meta name="go-import" content="%s">
</head>
</html>
`, substitution)
			lg("Sending vanity HTML %s", vanityHtml)

			return req, goproxy.NewResponse(req, goproxy.ContentTypeHtml, http.StatusOK, vanityHtml)
		})
	}

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatal("failed to listen", err)
	}

	if len(os.Args) < 2 {
		log.Fatalf("Usage : deproxy <cmd args...>")
	}

	stderrR, stderrW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	go io.Copy(os.Stdout, stdoutR)
	go io.Copy(os.Stderr, stderrR)

	newEnv := make(map[string]string, len(os.Environ()))
	for _, env := range os.Environ() {
		s := strings.Split(env, "=")
		k := s[0]
		v := strings.Join(s[1:], "")
		newEnv[k] = v
	}
	proxyAddr := fmt.Sprintf("localhost:%d", l.Addr().(*net.TCPAddr).Port)

	newEnv["http_proxy"] = proxyAddr
	newEnv["https_proxy"] = proxyAddr
	newEnv["HTTP_PROXY"] = proxyAddr
	newEnv["HTTPS_PROXY"] = proxyAddr
	// everything in dep comes to me!
	delete(newEnv,"no_proxy")
	delete(newEnv,"NO_PROXY")



	newEnvSlice := make([]string, 0, len(newEnv))
	for k, v := range newEnv {
		newEnvSlice = append(newEnvSlice, k+"="+v)
	}
	args := os.Args[2:]

	cmdName := os.Args[1]
	realPath, err := exec.LookPath(cmdName)
	if err != nil {
		log.Fatalf("Failed to find %s in PATH", cmdName)
	}

	lg("Running command %s with args %v", cmdName, args)
	cmd := exec.Cmd{
		Stderr: stderrW,
		Stdout: stdoutW,
		Path:   realPath,
		Args:   os.Args[1:],
		Env:    newEnvSlice,
	}
	go func() {
		err = cmd.Run()

		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}()

	http.Serve(l, proxy)

}
