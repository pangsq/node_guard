package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/procfs"
	"golang.org/x/sys/unix"
)

func init() {
	registerChecker("os", NewOSChecker())
}

type OSChecker struct {
	name             string
	ticker           *time.Ticker
	mutex            sync.RWMutex
	stopCh           chan struct{}
	checkerState     State
	checkTime        time.Time
	checkInterval    time.Duration
	basicInfo        map[string]interface{}
	errors           map[string]interface{}
	procPath         string
	kernelParameters []string
	dbusAddress      string
	units            []string
}

func (c *OSChecker) initialize(daemonConfig *DaemonConfig) error {
	c.name = "os"
	c.checkerState = Unitialized
	c.basicInfo = make(map[string]interface{})
	c.checkInterval = daemonConfig.getOrDefault(c.name, "checkInterval", time.Second*60).(time.Duration)
	c.procPath = daemonConfig.proc_path
	c.kernelParameters = daemonConfig.getOrDefault(c.name, "kernel.parameters", []string{}).([]string)
	c.dbusAddress = daemonConfig.dbus_address
	c.units = daemonConfig.getOrDefault(c.name, "units", []string{}).([]string)
	return c.check()
}

func (c *OSChecker) state() (State, string) {
	return c.checkerState, ""
}

func (c *OSChecker) start() {
	c.ticker = time.NewTicker(c.checkInterval)
	for {
		select {
		case <-c.ticker.C:
			go c.check()
		case <-c.stopCh:
			return
		}
	}
}

func (c *OSChecker) stop() {
	close(c.stopCh)
}

func (c *OSChecker) check() error {
	basicInfo := make(map[string]interface{})
	errors := make(map[string]interface{})
	defer func() {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		c.basicInfo = basicInfo
		c.errors = errors
		c.checkTime = time.Now()
		c.checkerState = Live
	}()

	defer func() {
		if r := recover(); r != nil {
			log.Println(fmt.Sprintf("Error Catched: %s", r))
		}
	}()
	//get basic info
	basicInfo["hostname"], _ = os.Hostname()
	basicInfo["uname"] = getUname()

	stats, err := getStats(c.procPath)
	if err != nil {
		errors["stats"] = err
	} else {
		basicInfo["stats"] = stats
	}

	loads, err := getLoads(c.procPath)
	if err != nil {
		errors["loads"] = err
	} else {
		basicInfo["loads"] = loads
	}
	if len(c.units) > 0 {
		unitsStatus, err := getUnitsStatus(c.dbusAddress, c.units)
		if err != nil {
			errors["units"] = err
		} else {
			basicInfo["units"] = unitsStatus
		}
	}

	runtimeParameters, err := getKernelParameters(c.procPath, c.kernelParameters)
	if err != nil {
		errors["kernel.runtime.parameters"] = err
	} else {
		basicInfo["kernel.runtime.parameters"] = runtimeParameters
	}

	return nil
}

func (c *OSChecker) info() Info {
	defer c.mutex.RUnlock()
	c.mutex.RLock()

	return Info{
		name:      c.name,
		checkTime: c.checkTime,
		state:     c.checkerState,
		basic:     c.basicInfo,
		errors:    c.errors,
	}
}

func (c *OSChecker) newRouters() Routers {
	routers := make(Routers)
	return routers
}

func NewOSChecker() *OSChecker {
	return &OSChecker{}
}

func getUname() map[string]string {
	var uname unix.Utsname
	unix.Uname(&uname)
	return map[string]string{
		"sysname":    string(uname.Sysname[:bytes.IndexByte(uname.Sysname[:], 0)]),
		"release":    string(uname.Release[:bytes.IndexByte(uname.Release[:], 0)]),
		"version":    string(uname.Version[:bytes.IndexByte(uname.Version[:], 0)]),
		"machine":    string(uname.Machine[:bytes.IndexByte(uname.Machine[:], 0)]),
		"nodename":   string(uname.Nodename[:bytes.IndexByte(uname.Nodename[:], 0)]),
		"domainname": string(uname.Domainname[:bytes.IndexByte(uname.Domainname[:], 0)]),
	}
}

func getStats(procPath string) (map[string]interface{}, error) {
	fs, err := procfs.NewFS(procPath)
	if err != nil {
		return nil, err
	}
	stats, err := fs.NewStat()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"bootTime":         float64(stats.BootTime),
		"IRQTotal":         float64(stats.IRQTotal),
		"contextSwitches":  float64(stats.ContextSwitches),
		"processCreated":   float64(stats.ProcessCreated),
		"processesRunning": float64(stats.ProcessesRunning),
		"processesBlocked": float64(stats.ProcessesBlocked),
	}, nil
}

// Parse /proc loadavg and return 1m, 5m and 15m.
func getLoads(procPath string) (loads []float64, err error) {
	loads_data, err := ioutil.ReadFile(path.Join(procPath, "loadavg"))
	if err != nil {
		return nil, err
	}
	loads = make([]float64, 3)
	parts := strings.Fields(string(loads_data))
	if len(parts) < 3 {
		return nil, fmt.Errorf("unexpected content in %s", path.Join(procPath, "loadavg"))
	}
	for i, load := range parts[0:3] {
		loads[i], err = strconv.ParseFloat(load, 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse load '%s': %s", load, err)
		}
	}
	return loads, nil
}
