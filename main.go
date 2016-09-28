package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime"
	"sync"

	"github.com/Unknwon/goconfig"
	//"github.com/fvbock/endless"
)

const (
	LOG_PREFIX string = "[reverseproxy]"

	CERT_FILE = "/home/kyf/cert/molu.crt"
	KEY_FILE  = "/home/kyf/cert/molu.key"
)

var (
	config_path string
	log_path    string

	my_logger *log.Logger

	exitCaller *ExitCaller
)

type ExitCaller struct {
	defer_handlers []func()
	mutex          sync.Mutex
}

func (ec *ExitCaller) Add(fn func()) {
	ec.mutex.Lock()
	defer ec.mutex.Unlock()
	ec.defer_handlers = append(ec.defer_handlers, fn)
}

func init() {
	flag.StringVar(&config_path, "config_path", "/etc/reverseproxy/conf.d/default.ini", "reverse proxy config file")
	flag.StringVar(&log_path, "log_path", "/var/log/reverseproxy/reverseproxy.log", "reverse proxy run log file")
	exitCaller = &ExitCaller{defer_handlers: make([]func(), 0)}
}

func defer_fn() {
	for _, fn := range exitCaller.defer_handlers {
		fn()
	}
}

func handle_err(e error) {
	if _, file, line, ok := runtime.Caller(1); ok {
		if my_logger != nil {
			my_logger.Printf("Occur error %v in %s line number [%d]", e, file, line)
		} else {
			fmt.Printf("Occur error %v in %s line number [%d]\n", e, file, line)
		}
	}

	os.Exit(1)
}

func init_logger(log_path string) (*log.Logger, error) {
	fp, err := os.OpenFile(log_path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	exitCaller.Add(func() { fp.Close() })
	logger := log.New(fp, LOG_PREFIX, log.LstdFlags)

	return logger, nil
}

func newMultiHostReverseProxy(hm map[string]string) *httputil.ReverseProxy {
	director := func(r *http.Request) {
		r.URL.Scheme = "http"
		r.URL.Host = hm[r.Host]
		r.URL.RawQuery = r.RequestURI
	}

	return &httputil.ReverseProxy{Director: director}
}

func main() {
	defer defer_fn()
	flag.Parse()

	my_logger, err := init_logger(log_path)
	if err != nil {
		handle_err(err)
	}
	conf, err := goconfig.LoadConfigFile(config_path)
	if err != nil {
		handle_err(err)
	}

	hm := make(map[string]string)
	for _, node := range conf.GetSectionList() {
		hm[node] = conf.MustValue(node, "host")
	}

	reverse_proxy := newMultiHostReverseProxy(hm)
	reverse_proxy.ErrorLog = my_logger

	var exit chan error
	go func() {
		exit <- http.ListenAndServeTLS(":443", CERT_FILE, KEY_FILE, reverse_proxy)
	}()

	e := <-exit
	my_logger.Printf("service exit, err is %v", e)
}
