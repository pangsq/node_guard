package main

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

var configsRecorded = make(map[string]map[string]interface{})

type DaemonConfig struct {
	separator     string
	debug_enable  bool
	mount_point   string
	proc_path     string
	sys_path      string
	rootfs_path   string
	dbus_address  string
	customConfigs map[string]map[string]interface{}
}

func (daemonConfig *DaemonConfig) toMap() map[string]interface{} {
	return map[string]interface{}{
		"separator":    daemonConfig.separator,
		"debug_enable": daemonConfig.debug_enable,
		"mount_point":  daemonConfig.mount_point,
		"proc_path":    daemonConfig.proc_path,
		"sys_path":     daemonConfig.sys_path,
		"rootfs_path":  daemonConfig.rootfs_path,
		"dbus_address": daemonConfig.dbus_address,
	}
}

func NewDaemonConfig() *DaemonConfig {
	return &DaemonConfig{
		separator:     ";",
		debug_enable:  false,
		mount_point:   "/",
		proc_path:     "/proc",
		sys_path:      "/sys",
		rootfs_path:   "/",
		dbus_address:  "unix:path=/run/systemd/private",
		customConfigs: make(map[string]map[string]interface{}),
	}
}

func (daemonConfig *DaemonConfig) setMountPoint(mount_point string) *DaemonConfig {
	daemonConfig.mount_point = mount_point
	daemonConfig.proc_path = path.Join(mount_point, "/proc")
	daemonConfig.sys_path = path.Join(mount_point, "/sys")
	daemonConfig.rootfs_path = mount_point
	daemonConfig.dbus_address = "unix:path=" + path.Join(mount_point, "/run/systemd/private")
	return daemonConfig
}

func (daemonConfig *DaemonConfig) getCustomConfig(checkerName string) map[string]interface{} {
	if config, ok := daemonConfig.customConfigs[checkerName]; ok {
		return config
	}
	return make(map[string]interface{})
}

func (daemonConfig *DaemonConfig) getOrDefault(checkerName string, key string, default_value interface{}) (value interface{}) {
	defer func() {
		if checkerConfigs, ok := configsRecorded[checkerName]; ok {
			checkerConfigs[key] = value
		} else {
			configsRecorded[checkerName] = map[string]interface{}{key: value}
		}
	}()
	//from env
	value_str := os.Getenv(fmt.Sprintf("%s_%s", strings.ToUpper(checkerName), strings.ToUpper(strings.Replace(key, ".", "_", -1))))
	if value_str != "" {
		value_array_string := strings.Split(value_str, daemonConfig.separator) // used when default_value is an array
		switch default_value.(type) {
		case string:
			return value_str
		case int:
			value_int, _ := strconv.Atoi(value_str)
			return value_int
		case float64:
			value_float, _ := strconv.ParseFloat(value_str, 64)
			return value_float
		case time.Duration:
			value_duration, _ := time.ParseDuration(value_str)
			return value_duration
		case []string:
			return value_array_string
		case []int:
			var value_array_int []int
			for _, item_string := range value_array_string {
				item_int, _ := strconv.Atoi(item_string)
				value_array_int = append(value_array_int, item_int)
			}
			return value_array_int
		case []float64:
			var value_array_float64 []float64
			for _, item_string := range value_array_string {
				item_float64, _ := strconv.ParseFloat(item_string, 64)
				value_array_float64 = append(value_array_float64, item_float64)
			}
			return value_array_float64
		// map is not supported
		// case map[string]string:
		// case map[string]int:
		// case map[string]float64:
		default:
			return value_str
		}
	}
	//from daemonConfig
	the_value, ok := daemonConfig.getCustomConfig(checkerName)[key]
	if !ok {
		//default
		return default_value
	}

	switch the_value.(type) {
	case string:
		switch default_value.(type) {
		case time.Duration:
			value_duration, _ := time.ParseDuration(the_value.(string))
			return value_duration
		default:
			return the_value
		}
	case int:
		switch default_value.(type) {
		case time.Duration:
			value_duration := time.Second * time.Duration(the_value.(int))
			return value_duration
		default:
			return the_value
		}
	case []interface{}:
		switch default_value.(type) {
		case []string:
			var value_array_string []string
			for _, item := range the_value.([]interface{}) {
				item_string, _ := item.(string)
				value_array_string = append(value_array_string, item_string)
			}
			return value_array_string
		case []int:
			var value_array_int []int
			for _, item := range the_value.([]interface{}) {
				item_int, _ := item.(int)
				value_array_int = append(value_array_int, item_int)
			}
			return value_array_int
		case []float64:
			var value_array_float64 []float64
			for _, item := range the_value.([]interface{}) {
				item_float64, _ := item.(float64)
				value_array_float64 = append(value_array_float64, item_float64)
			}
			return value_array_float64
		}
	// map is not supported
	// case map[string]string:
	// case map[string]int:
	// case map[string]float64:
	default:
		return the_value
	}

	return the_value
}
