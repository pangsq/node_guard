package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-systemd/dbus"
	raw_dbus "github.com/godbus/dbus"
	yaml "gopkg.in/yaml.v2"
)

var debugLogger *log.Logger
var infoLogger *log.Logger
var errorLogger *log.Logger

func initLogger(debug_enable bool) {
	if debug_enable {
		debugLogger = log.New(os.Stdout, "DEBUG\t", log.LstdFlags)
	}
	infoLogger = log.New(os.Stdout, "INFO\t", log.LstdFlags)
	errorLogger = log.New(os.Stdout, "ERROR\t", log.LstdFlags)
}

func debugln(args ...interface{}) {
	if debugLogger != nil {
		debugLogger.Printf(fmt.Sprintln(args))
	}
}

func infoln(args ...interface{}) {
	infoLogger.Printf(fmt.Sprintln(args))
}

func errorln(args ...interface{}) {
	errorLogger.Printf(fmt.Sprintln(args))
}

func ping(ips map[string]string, timeout time.Duration) map[string]bool {
	var wg sync.WaitGroup
	conditions := make(map[string]bool)
	ch := make(chan ([]interface{}))

	for name, ip := range ips {
		wg.Add(1)
		go func(name string, ip string, timeout time.Duration) {
			_, err := net.DialTimeout("ip4:icmp", ip, timeout)
			if err != nil {
				ch <- []interface{}{name, false}
			} else {
				ch <- []interface{}{name, true}
			}
		}(name, ip, timeout)
	}
	go func() {
	Loop:
		for {
			select {
			case cond, ok := <-ch:
				if !ok {
					break Loop
				}
				conditions[cond[0].(string)] = cond[1].(bool)
				wg.Done()
			}
		}
	}()
	wg.Wait()
	close(ch)
	return conditions
}

func getKernelParameters(procPath string, params []string) (map[string]interface{}, error) {
	runtime_parameters := make(map[string]interface{})
	for _, param := range params {
		data, err := ioutil.ReadFile(path.Join(procPath+"/sys", strings.Replace(param, ".", "/", -1)))
		if err != nil {
			return nil, err
		} else {
			runtime_parameters[param] = strings.TrimSpace(string(data))
		}
	}
	return runtime_parameters, nil
}

func formatWrite(data interface{}, w http.ResponseWriter, r *http.Request) {
	format_type := "yaml"
	if len(r.URL.Query().Get("format")) > 0 {
		format_type = string(r.URL.Query().Get("format"))
	}
	var output []byte
	var err error
	switch format_type {
	case "json":
		output, err = json.MarshalIndent(data, "", "  ")
	case "yaml":
		output, err = yaml.Marshal(data)
	}
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, err.Error())
	}
	fmt.Fprintf(w, string(output))
}

// 参考 github.com/prometheus/node_exporte/collector/systemd_linux.go
func getUnitsStatus(dbusAddress string, unitNames []string) (map[string]interface{}, error) {
	conn, err := dbus.NewConnection(func() (*raw_dbus.Conn, error) {
		raw_conn, err := raw_dbus.Dial(dbusAddress)
		// dbusAuthConnection(*dbus.Conn)
		// Only use EXTERNAL method, and hardcode the uid (not username)
		// to avoid a username lookup (which requires a dynamically linked
		// libc)
		methods := []raw_dbus.Auth{raw_dbus.AuthExternal(strconv.Itoa(os.Getuid()))}

		err = raw_conn.Auth(methods)
		if err != nil {
			raw_conn.Close()
			return nil, err
		}
		return raw_conn, nil
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't get dbus connection: %s", err)
	}
	defer conn.Close()
	units, err := conn.ListUnits()
	if err != nil {
		return nil, err
	}
	unitsStatus := make(map[string]interface{})
	for _, unitName := range unitNames {
		unitsStatus[unitName] = nil
	}
	for _, unit := range units {
		if _, ok := unitsStatus[unit.Name]; ok {
			unitStatus := map[string]interface{}{
				"description": unit.Description,
				"loadState":   unit.LoadState,
				"activeState": unit.ActiveState,
				"subState":    unit.SubState,
			}
			if strings.HasSuffix(unit.Name, ".service") {

				tasksCurrentCount, err := conn.GetUnitTypeProperty(unit.Name, "Service", "TasksCurrent")
				if err != nil {
					debugln("couldn't get unit '%s' TasksCurrent: %s", unit.Name, err)
				} else {
					val := tasksCurrentCount.Value.Value().(uint64)
					// Don't set if tasksCurrent if dbus reports MaxUint64.
					if val != math.MaxUint64 {
						unitStatus["tasksCurrent"] = &val
					}
				}

				tasksMaxCount, err := conn.GetUnitTypeProperty(unit.Name, "Service", "TasksMax")
				if err != nil {
					debugln("couldn't get unit '%s' TasksMax: %s", unit.Name, err)
				} else {
					val := tasksMaxCount.Value.Value().(uint64)
					// Don't set if tasksMax if dbus reports MaxUint64.
					if val != math.MaxUint64 {
						unitStatus["tasksMax"] = &val
					}
				}
			}
			if unit.ActiveState != "active" {
				unitStatus["startTimeUsec"] = 0
			} else {
				timestampValue, err := conn.GetUnitProperty(unit.Name, "ActiveEnterTimestamp")
				if err != nil {
					debugln("couldn't get unit '%s' StartTimeUsec: %s", unit.Name, err)
					continue
				}

				unitStatus["startTimeUsec"] = time.Unix(0, int64(timestampValue.Value.Value().(uint64))*1000)
			}
			unitsStatus[unit.Name] = unitStatus
		}
	}

	return unitsStatus, nil
}
