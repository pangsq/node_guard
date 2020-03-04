package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	registerChecker("kubernetes", NewKubernetesChecker())
}

type KubernetesChecker struct {
	name             string
	ticker           *time.Ticker
	mutex            sync.RWMutex
	stopCh           chan struct{}
	checkerState     State
	checkTime        time.Time
	checkInterval    time.Duration
	pingTimeout      time.Duration
	basicInfo        map[string]interface{}
	errors           map[string]interface{}
	procPath         string
	kubeletConfPath  string
	clientConfig     *rest.Config
	clientset        *kubernetes.Clientset
	clientConfigAuth map[string]interface{}
	localNode        *v1.Node
	dockerHost       string
	dockerAPIVersion string
	dockerClient     *client.Client
}

func (c *KubernetesChecker) initialize(daemonConfig *DaemonConfig) error {
	var err error
	c.name = "kubernetes"
	c.checkerState = Unitialized
	c.procPath = daemonConfig.proc_path
	c.checkInterval = daemonConfig.getOrDefault(c.name, "checkInterval", time.Second*120).(time.Duration)
	c.pingTimeout = daemonConfig.getOrDefault(c.name, "pingTimeout", time.Second*5).(time.Duration)
	c.kubeletConfPath = daemonConfig.getOrDefault(c.name, "kubelet.conf.path", path.Join(daemonConfig.mount_point, "/etc/kubernetes/kubelet.conf")).(string)
	c.dockerHost = daemonConfig.getOrDefault(c.name, "docker.host", "unix://"+path.Join(daemonConfig.mount_point, "/var/run/docker.sock")).(string)
	c.dockerAPIVersion = daemonConfig.getOrDefault(c.name, "dcoker.api.version", "1.22").(string)
	c.dockerClient, err = client.NewClient(c.dockerHost, c.dockerAPIVersion, nil, nil)
	if err != nil {
		return err
	}
	c.clientConfig, err = clientcmd.BuildConfigFromFlags("", c.kubeletConfPath)
	if err != nil {
		return err
	}
	c.clientset, err = kubernetes.NewForConfig(c.clientConfig)
	if err != nil {
		return err
	}
	c.clientConfigAuth = make(map[string]interface{})
	c.clientConfigAuth["host"] = c.clientConfig.Host
	if c.clientConfig.CertData != nil {
		block, _ := pem.Decode(c.clientConfig.CertData)
		if block == nil {
			return errors.New("Failed to parse client-certificate-data")
		}
		x509Cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return errors.New("Failed to parse client-certificate-data with error: " + err.Error())
		}
		c.clientConfigAuth["notBefore"] = x509Cert.NotBefore
		c.clientConfigAuth["notAfter"] = x509Cert.NotAfter
		c.clientConfigAuth["subject"] = x509Cert.Subject.String()
		// hostname, _ := os.Hostname()
		// if x509Cert.VerifyHostname(hostname) != nil {
		// 	// c.clientConfigAuth["verifyHostname"] = "not passed"
		// 	c.clientConfigAuth["verifyHostname"] = x509Cert.VerifyHostname(hostname)
		// }
	}

	return c.check()
}

func (c *KubernetesChecker) state() (State, string) {
	return c.checkerState, ""
}

func (c *KubernetesChecker) start() {
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

func (c *KubernetesChecker) stop() {
	close(c.stopCh)
}

func (c *KubernetesChecker) check() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	basicInfo := make(map[string]interface{})
	errors := make(map[string]interface{})
	localNode := &v1.Node{}
	defer func() {
		c.basicInfo = basicInfo
		c.errors = errors
		c.localNode = localNode
		c.checkTime = time.Now()
		c.checkerState = Live
	}()

	defer func() {
		if r := recover(); r != nil {
			log.Println(fmt.Sprintf("Error Catched: %s", r))
		}
	}()
	// get basic info
	var err error
	basicInfo["docker"], err = getDockerInfo(c.dockerClient)
	if err != nil {
		errors["docker"] = err.Error()
	}
	basicInfo["kubernetes"], localNode, err = getKubernetesInfo(c.clientset, c.pingTimeout)
	if err != nil {
		errors["kubernetes"] = err.Error()
	}
	basicInfo["clientConfigAuth"] = c.clientConfigAuth

	return nil
}

func (c *KubernetesChecker) newRouters() Routers {
	routers := make(Routers)
	routers["detail"] = func(w http.ResponseWriter, r *http.Request) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()
		details := map[string]interface{}{
			"localNode": c.localNode,
		}
		errors := make(map[string]interface{})
		containers, err := c.dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
		if err != nil {
			errors["docker.containers"] = err
		} else {
			details["docker.containers"] = containers
		}
		images, err := c.dockerClient.ImageList(context.Background(), types.ImageListOptions{})
		if err != nil {
			errors["docker.images"] = err
		} else {
			details["docker.images"] = images
		}
		info, err := c.dockerClient.Info(context.Background())
		if err != nil {
			errors["docker.info"] = err
		} else {
			details["docker.info"] = info
		}

		formatWrite(details, w, r)
	}
	return routers
}

func NewKubernetesChecker() *KubernetesChecker {
	return &KubernetesChecker{}
}

func getDockerInfo(cli *client.Client) (map[string]interface{}, error) {

	info, err := cli.Info(context.Background())
	if err != nil {
		return nil, err
	}

	dockerInfo := map[string]interface{}{
		"containers":          info.Containers,
		"containers.running":  info.ContainersRunning,
		"containers.paused":   info.ContainersPaused,
		"containers.stopped":  info.ContainersStopped,
		"images":              info.Images,
		"driver":              info.Driver,
		"cgroup.driver":       info.CgroupDriver,
		"logging.driver":      info.LoggingDriver,
		"ipv4.forwarding":     info.IPv4Forwarding,
		"bridge.nf.iptables":  info.BridgeNfIptables,
		"bridge.nf.ip6tables": info.BridgeNfIP6tables,
		"docker.root.dir":     info.DockerRootDir,
		"server.version":      info.ServerVersion,
	}
	return dockerInfo, nil
}

func getKubernetesInfo(clientset *kubernetes.Clientset, pingTimeout time.Duration) (map[string]interface{}, *v1.Node, error) {
	kubernetesInfo := make(map[string]interface{})

	// namespaces
	namespaces, err := clientset.Core().Namespaces().List(metav1.ListOptions{})
	namespaceNames := []string{}
	for _, namespace := range namespaces.Items {
		namespaceNames = append(namespaceNames, namespace.Name)
	}
	if err != nil {
		return nil, nil, err
	}
	kubernetesInfo["namespaces"] = namespaceNames

	// pods
	nodePods := make(map[string][]v1.Pod)
	for _, namespace := range namespaces.Items {
		pods, err := clientset.Core().Pods(namespace.Name).List(metav1.ListOptions{})
		if err != nil {
			return kubernetesInfo, nil, err
		}
		for _, pod := range pods.Items {
			nodeName := pod.Spec.NodeName
			if nodeName != "" {
				thePods, ok := nodePods[nodeName]
				if ok {
					nodePods[nodeName] = append(thePods, pod)
				} else {
					nodePods[nodeName] = []v1.Pod{pod}
				}
			}
		}
	}

	nodeList, err := clientset.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return kubernetesInfo, nil, err
	}

	nodes := make(map[string]interface{})
	ips := make(map[string]string)
	for _, node := range nodeList.Items {
		flannelIP := strings.Split(node.Spec.PodCIDR, "/")[0]
		ips[node.Name] = flannelIP
	}
	flannelIPConditions := ping(ips, pingTimeout)
	for _, node := range nodeList.Items {
		cond, ok := flannelIPConditions[node.Name]
		if !ok {
			cond = false
		}

		nodes[node.Name] = map[string]interface{}{
			"spec.podCIDR": node.Spec.PodCIDR,
			"flannel.ping": cond,
		}

		thePods, ok := nodePods[node.Name]
		if ok {
			podIPs := make(map[string]string)
			for _, pod := range thePods {
				podIP := pod.Status.PodIP
				if podIP != "" {
					podIPs[pod.Name] = podIP
				}
			}
			podIPConditions := ping(podIPs, pingTimeout)
			pingSuccessPods := []string{}
			pingFailPods := []string{}
			for name, cond := range podIPConditions {
				if cond {
					pingSuccessPods = append(pingSuccessPods, name)
				} else {
					pingFailPods = append(pingFailPods, name)
				}
			}
			if len(pingSuccessPods) > 0 {
				nodes[node.Name].(map[string]interface{})["pods.ping.success.num"] = len(pingSuccessPods)
			}
			if len(pingFailPods) > 0 {
				nodes[node.Name].(map[string]interface{})["pods.ping.fail"] = pingFailPods
			}
		}
	}

	kubernetesInfo["nodes"] = nodes
	hostname, err := os.Hostname()
	if err != nil {
		return kubernetesInfo, nil, err
	}
	localNode, err := clientset.Core().Nodes().Get(hostname, metav1.GetOptions{})
	if err != nil {
		return kubernetesInfo, nil, err
	}
	local := map[string]interface{}{
		"status.capacity.memory":                 localNode.Status.Capacity.Memory().String(),
		"status.capacity.cpu":                    localNode.Status.Capacity.Cpu().String(),
		"status.capacity.pods":                   localNode.Status.Capacity.Pods().String(),
		"status.allocatable.memory":              localNode.Status.Allocatable.Memory().String(),
		"status.allocatable.cpu":                 localNode.Status.Capacity.Cpu().String(),
		"status.allocatable.pods":                localNode.Status.Capacity.Pods().String(),
		"status.daemonEndpoints.kubeletEndpoint": localNode.Status.DaemonEndpoints.KubeletEndpoint,
	}
	kubernetesInfo["local"] = local
	return kubernetesInfo, localNode, nil
}

func (c *KubernetesChecker) info() Info {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return Info{
		name:      c.name,
		checkTime: c.checkTime,
		state:     c.checkerState,
		basic:     c.basicInfo,
		errors:    c.errors,
	}
}
