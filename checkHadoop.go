package main

import (
	"io/ioutil"
	"path"
	"sync"
	"time"
)

func init() {
	registerChecker("hadoop", NewHadoopChecker())
}

type HadoopChecker struct {
	name          string
	ticker        *time.Ticker
	mutex         sync.RWMutex
	stopCh        chan struct{}
	checkerState  State
	checkTime     time.Time
	checkInterval time.Duration
	basicInfo     map[string]interface{}
	errors        map[string]interface{}
	procPath      string
	krb5Path      string
}

func (c *HadoopChecker) initialize(daemonConfig *DaemonConfig) error {
	c.name = "hadoop"
	c.checkerState = Unitialized
	c.basicInfo = make(map[string]interface{})
	c.checkInterval = daemonConfig.getOrDefault(c.name, "checkInterval", time.Second*60).(time.Duration)
	c.procPath = daemonConfig.proc_path
	c.krb5Path = daemonConfig.getOrDefault(c.name, "etc.krb5.conf.path", path.Join(daemonConfig.mount_point, "/etc/krb5.conf")).(string)
	return c.check()
}

func (c *HadoopChecker) state() (State, string) {
	return c.checkerState, ""
}

func (c *HadoopChecker) start() {
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

func (c *HadoopChecker) stop() {
	close(c.stopCh)
}

func (c *HadoopChecker) check() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	basicInfo := make(map[string]interface{})
	errors := make(map[string]interface{})
	defer func() {
		c.basicInfo = basicInfo
		c.errors = errors
		c.checkTime = time.Now()
		c.checkerState = Live
	}()
	krb5Content, err := getKrb5(c.krb5Path)
	if err != nil {
		errors["krb5"] = err
	} else {
		basicInfo["krb5"] = krb5Content
	}
	return nil
}

func (c *HadoopChecker) info() Info {
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

func (c *HadoopChecker) newRouters() Routers {
	routers := make(Routers)
	return routers
}

func NewHadoopChecker() *HadoopChecker {
	return &HadoopChecker{}
}

func getKrb5(krb5Path string) (string, error) {
	data, err := ioutil.ReadFile(krb5Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
