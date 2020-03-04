package main

import (
	"flag"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"
)

var config_path string
var mount_point string
var debug_enable bool
var listen_port int

func main() {

	flag.StringVar(&config_path, "c", "", "the config file path")
	flag.StringVar(&mount_point, "m", "", "mount point")
	flag.BoolVar(&debug_enable, "d", false, "debug mode")
	flag.IntVar(&listen_port, "p", 8080, "listen port")
	flag.Parse()
	initLogger(debug_enable)
	daemonConfig := NewDaemonConfig()
	daemonConfig.debug_enable = debug_enable
	if mount_point != "" {
		daemonConfig.setMountPoint(mount_point)
	}
	if config_path != "" {
		yaml_data, err := ioutil.ReadFile(config_path)
		if err != nil {
			panic(err)
		}
		err = yaml.Unmarshal([]byte(yaml_data), &daemonConfig.customConfigs)
		if err != nil {
			panic(err)
		}
	}
	daemon := NewDaemon(daemonConfig)
	go daemon.run()
	server := NewServer(daemon, listen_port)
	if err := server.run(); err != nil {
		errorln(err.Error())
		os.Exit(-1)
	}
}
