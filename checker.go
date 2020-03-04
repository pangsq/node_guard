package main

import (
	"time"
)

type Checker interface {
	initialize(daemonConfig *DaemonConfig) error
	state() (State, string)
	start()
	stop()
	check() error
	info() Info
	newRouters() Routers
}

var checkers = make(map[string]Checker)

func registerChecker(name string, checker Checker) {
	checkers[name] = checker
}

type Info struct {
	name      string
	checkTime time.Time
	state     State
	basic     map[string]interface{}
	detail    map[string]interface{}
	errors    map[string]interface{}
}

func (info *Info) toMap() map[string]interface{} {
	infoMap := make(map[string]interface{})
	infoMap["name"] = info.name
	infoMap["time"] = info.checkTime
	infoMap["state"] = info.state
	if info.detail != nil {
		infoMap["detail"] = info.detail
	}
	if info.basic != nil {
		infoMap["basic"] = info.basic
	}
	if info.errors != nil {
		infoMap["errors"] = info.errors
	}
	return infoMap
}

func NewInfo() {

}

func UnknownInfo(name string) *Info {
	return &Info{
		name:  name,
		state: Unknown,
	}
}
