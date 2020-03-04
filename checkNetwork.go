package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

func init() {
	registerChecker("network", NewNetworkChecker())
}

type NetworkChecker struct {
	name             string
	ticker           *time.Ticker
	mutex            sync.RWMutex
	stopCh           chan struct{}
	checkInterval    time.Duration
	checkerState     State
	checkTime        time.Time
	basicInfo        map[string]interface{}
	errors           map[string]interface{}
	details          map[string]interface{}
	procPath         string
	hostsPath        string
	resolvConfPath   string
	concernedHosts   []string
	kernelParameters []string
	statusFilePath   string
	netRoutePath     string
}

func (c *NetworkChecker) initialize(daemonConfig *DaemonConfig) error {
	c.name = "network"
	c.checkerState = Unitialized
	c.procPath = daemonConfig.proc_path
	c.checkInterval = daemonConfig.getOrDefault(c.name, "checkInterval", time.Second*60).(time.Duration)
	c.hostsPath = daemonConfig.getOrDefault(c.name, "etc.hosts.path", path.Join(daemonConfig.mount_point, "/etc/hosts")).(string)
	c.concernedHosts = daemonConfig.getOrDefault(c.name, "etc.hosts.concerned", []string{}).([]string)
	c.resolvConfPath = daemonConfig.getOrDefault(c.name, "etc.resolv.conf.path", path.Join(daemonConfig.mount_point, "/etc/resolv.conf")).(string)
	c.kernelParameters = daemonConfig.getOrDefault(c.name, "kernel.parameters", []string{}).([]string)
	c.statusFilePath = daemonConfig.getOrDefault(c.name, "status.file.path", path.Join(daemonConfig.sys_path, "class/net")).(string)
	c.netRoutePath = daemonConfig.getOrDefault(c.name, "net.route.path", path.Join(daemonConfig.proc_path, "net/route")).(string)
	return c.check()
}

func (c *NetworkChecker) state() (State, string) {
	return c.checkerState, ""
}

func (c *NetworkChecker) start() {
	c.ticker = time.NewTicker(time.Second * 5)
	for {
		select {
		case <-c.ticker.C:
			go c.check()
		case <-c.stopCh:
			return
		}
	}
}

func (c *NetworkChecker) stop() {
	close(c.stopCh)
}

func (c *NetworkChecker) check() error {
	defer c.mutex.Unlock()
	c.mutex.Lock()
	basicInfo := make(map[string]interface{})
	errors := make(map[string]interface{})
	details := make(map[string]interface{})
	defer func() {
		c.basicInfo = basicInfo
		c.errors = errors
		c.details = details
		c.checkTime = time.Now()
		c.checkerState = Live
	}()

	defer func() {
		if r := recover(); r != nil {
			log.Println(fmt.Sprintf("Error Catched: %s", r))
		}
	}()

	hostsContent, hostsConcerned, err := getHosts(c.hostsPath, c.concernedHosts)
	if err != nil {
		errors["hosts"] = err
	} else {
		details["hosts"] = hostsContent
		basicInfo["hosts.concerned"] = hostsConcerned
	}

	resolvConfContent, resolvConfs, err := getResolv(c.resolvConfPath)
	if err != nil {
		errors["resolv.conf"] = err
	} else {
		details["resolv.conf"] = resolvConfContent
		basicInfo["resolv.conf"] = resolvConfs
	}

	runtimeParameters, err := getKernelParameters(c.procPath, c.kernelParameters)
	if err != nil {
		errors["kernel.runtime.parameters"] = err
	} else {
		basicInfo["kernel.runtime.parameters"] = runtimeParameters
	}

	bondingStates, err := readBondingStats(c.statusFilePath)
	if err != nil {
		errors["bonding.stats"] = err
	} else {
		basicInfo["bonding.stats"] = bondingStates
	}

	intfs, err := readIntfs(c.netRoutePath)
	if err != nil {
		errors["net"] = err
	} else {
		basicInfo["net"] = intfs
	}

	return nil
}

func (c *NetworkChecker) info() Info {
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

func (c *NetworkChecker) newRouters() Routers {
	routers := make(Routers)
	return routers
}

func NewNetworkChecker() *NetworkChecker {
	return &NetworkChecker{}
}

func getHosts(hostsPath string, concernedHosts []string) (string, map[string]string, error) {
	hosts_data, err := ioutil.ReadFile(hostsPath)
	if err != nil {
		return "", nil, err
	}
	hosts_str := string(hosts_data)
	hosts := make(map[string]string)
	for _, line := range strings.Split(hosts_str, "\n") {
		segs := strings.Fields(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if len(segs) >= 2 {
			ip := segs[0]
			for _, hostname := range segs[1:] {
				hosts[hostname] = ip
			}
		}
	}
	hostsConcerned := make(map[string]string)
	for _, hostname := range concernedHosts {
		if ip, ok := hosts[hostname]; ok {
			hostsConcerned[hostname] = ip
		} else {
			hostsConcerned[hostname] = ""
		}
	}
	return hosts_str, hostsConcerned, nil
}

func getResolv(resolvConfPath string) (string, map[string]interface{}, error) {
	resolv_data, err := ioutil.ReadFile(resolvConfPath)
	if err != nil {
		return "", nil, err
	}
	resolv_str := string(resolv_data)
	nameservers := []string{}
	searchs := []string{}
	for _, line := range strings.Split(resolv_str, "\n") {
		segs := strings.Fields(line)
		if len(segs) < 2 {
			continue
		}
		switch segs[0] {
		case "nameserver":
			nameservers = append(nameservers, segs[1])
		case "search":
			for _, search := range segs[1:] {
				searchs = append(searchs, search)
			}
		}
	}

	return resolv_str, map[string]interface{}{"nameservers": nameservers, "searchs": searchs}, nil
}

// github.com/prometheus/node_exporter/collector/bonding_linux.go
func readBondingStats(root string) (status map[string]map[string]interface{}, err error) {
	status = map[string]map[string]interface{}{}
	masters, err := ioutil.ReadFile(path.Join(root, "bonding_masters"))
	if err != nil {
		return nil, err
	}
	for _, master := range strings.Fields(string(masters)) {
		slaves, err := ioutil.ReadFile(path.Join(root, master, "bonding", "slaves"))
		if err != nil {
			return nil, err
		}
		sstat := map[string]interface{}{}
		for _, slave := range strings.Fields(string(slaves)) {
			state, err := ioutil.ReadFile(path.Join(root, master, fmt.Sprintf("lower_%s", slave), "operstate"))
			if err != nil {
				return nil, err
			}
			speed, err := ioutil.ReadFile(path.Join(root, master, fmt.Sprintf("lower_%s", slave), "speed"))
			if err != nil {
				return nil, err
			}
			sstat[slave] = map[string]string{
				"state": strings.TrimSpace(string(state)),
				"speed": strings.TrimSpace(string(speed)),
			}
		}
		status[master] = sstat
	}
	return status, err
}

func readIntfs(netRoutePath string) (map[string]interface{}, error) {
	file, err := os.Open(netRoutePath)
	if err != nil {
		return nil, fmt.Errorf("can not open " + netRoutePath + " : " + err.Error())
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	infInfos := map[string]interface{}{}
	gateways := map[string]net.IP{}
	for scanner.Scan() {
		columns := strings.Fields(scanner.Text())

		if len(columns) < 11 {
			return nil, errors.New("unexpected route format")
		}

		if columns[0] != "Iface" {
			iface := columns[0]
			if columns[2] != "00000000" {
				gateway, err := parseHexIP(columns[2])
				if err != nil {
					return nil, err
				}
				gateways[iface] = gateway
			}
		}
	}
	intfs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, intf := range intfs {
		if strings.HasPrefix(intf.Name, "veth") {
			continue
		}
		addrs, err := intf.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		addresses := []string{}
		for _, addr := range addrs {
			addresses = append(addresses, addr.String())
		}
		infInfo := map[string]interface{}{
			"addresses": addresses,
		}
		if gateway, ok := gateways[intf.Name]; ok {
			infInfo["gateway"] = gateway
		}
		infInfos[intf.Name] = infInfo
	}
	return infInfos, nil
}

func parseHexIP(hexIP string) (net.IP, error) {
	bytes, err := hex.DecodeString(hexIP)
	if err != nil {
		return nil, fmt.Errorf("can not parse string to bytes: " + err.Error())
	}
	if len(bytes) != 4 {
		return nil, fmt.Errorf("hexIP has %d bytes", len(bytes))
	}
	return net.IP{bytes[3], bytes[2], bytes[1], bytes[0]}, nil
}

func parseHexMask(hexMask string) (net.IPMask, error) {
	bytes, err := hex.DecodeString(hexMask)
	if err != nil {
		return nil, fmt.Errorf("can not parse string to bytes: " + err.Error())
	}
	if len(bytes) != 4 {
		return nil, fmt.Errorf("hexMask has %d bytes", len(bytes))
	}
	return net.IPv4Mask(bytes[3], bytes[2], bytes[1], bytes[0]), nil
}
