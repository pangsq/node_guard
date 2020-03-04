package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	timeoutSeconds = 30
)

type State string

const (
	Error       = "Error"
	Live        = "Live"
	Unknown     = "Unknown"
	Fatal       = "Fatal"
	Unitialized = "Unitialized"
)

type Daemon struct {
	config *DaemonConfig
}

func NewDaemon(config *DaemonConfig) *Daemon {
	daemon := &Daemon{
		config: config,
	}

	disable_checkers := config.getOrDefault("checkers", "disable", []string{}).([]string)
	if len(disable_checkers) > 0 {
		log.Println(fmt.Sprintf("Disable checkers: %s", strings.Join(disable_checkers, ",")))
		for _, disable_checker := range disable_checkers {
			delete(checkers, disable_checker)
		}
	}

	for name, checker := range checkers {
		err := checker.initialize(daemon.config)
		if err != nil {
			log.Fatalln(err.Error())
		}
		log.Println(fmt.Sprintf("Checker: %s\t begins to initialize", name))
		go checker.start()
	}
	return daemon
}

func (*Daemon) run() error {
	return nil
}

func (daemon *Daemon) states() map[string]interface{} {
	if len(checkers) == 0 {
		return map[string]interface{}{}
	}
	states := make(map[string]interface{})
	ch := make(chan Info)
	ok := make(chan struct{})
	for name, checker := range checkers {
		info := UnknownInfo(name)
		states[name] = info.toMap()
		go func(name string, checker Checker) {
			ch <- checker.info()
		}(name, checker)
	}

	// 检查所有checker是否已返回info
	go func() {
		count := 0
		for {
			select {
			case info := <-ch:
				count += 1
				states[info.name] = info.toMap()
				if count == len(checkers) {
					close(ok)
					break
				}
			}
		}
	}()

	// 超时机制
	select {
	case <-time.After(time.Second * timeoutSeconds):
		log.Println(fmt.Sprintf("Timeout while trying to get infos"))
	case <-ok:
	}

	return states
}
