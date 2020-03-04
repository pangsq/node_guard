package main

import (
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
)

type Routers map[string]func(w http.ResponseWriter, r *http.Request)

type Server struct {
	daemon      *Daemon
	listen_port int
	router      *mux.Router
}

func NewServer(daemon *Daemon, listen_port int) *Server {
	server := &Server{
		daemon:      daemon,
		listen_port: listen_port,
	}
	server.router = server.createMux()
	return server
}

func (s *Server) createMux() *mux.Router {
	r := mux.NewRouter()
	statesInfoSetup(r, s.daemon)
	configsSetup(r, s.daemon)
	checkerRoutersSetup(r)
	profilerSetup(r)
	return r
}

func configsSetup(r *mux.Router, daemon *Daemon) {
	r.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		configs := map[string]interface{}{
			"daemon":  daemon.config.toMap(),
			"checker": configsRecorded,
		}
		formatWrite(configs, w, r)
	})
	infoln(fmt.Sprintf("Setup on /configs"))
}

func statesInfoSetup(r *mux.Router, daemon *Daemon) {
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		formatWrite(daemon.states(), w, r)
	})
	infoln(fmt.Sprintf("Setup on /"))
}

func checkerRoutersSetup(r *mux.Router) {
	for name, checker := range checkers {
		config_set := false
		for path, _func := range checker.newRouters() {
			router_path := fmt.Sprintf("/%s/%s", name, path)
			r.HandleFunc(router_path, _func)
			infoln(fmt.Sprintf("Setup on %s", router_path))
			if router_path == "config" {
				config_set = true
			}
		}
		if !config_set {
			config_path := fmt.Sprintf("/%s/config", name)
			r.HandleFunc(config_path, func(w http.ResponseWriter, r *http.Request) {
				config, _ := configsRecorded[name]
				formatWrite(config, w, r)
			})
			infoln(fmt.Sprintf("Setup on %s", config_path))
		}

	}
}

func profilerSetup(r *mux.Router) {
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	r.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	r.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	r.HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
}

func (s *Server) run() error {
	return http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", s.listen_port), s.router)
}
