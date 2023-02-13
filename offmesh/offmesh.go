package offmesh

import (
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
	"os"
)

const (
	ClusterConfigYamlPath = `/etc/offmesh/cluster-conf.yaml`

	CPUNode = "cpu_node"
	DPUNode = "dpu_node"
)

var offmeshCluster ClusterConfig
var read = false

type PUPair struct {
	CPUIp   string `yaml:"cpuNodeIP"`
	DPUIp   string `yaml:"dpuNodeIP"`
	CPUName string `yaml:"cpuNodeName"`
	DPUName string `yaml:"dpuNodeName"`
}

type PU struct {
	IP   string `yaml:"nodeIP"`
	Name string `yaml:"nodeName"`
}
type ClusterConfig struct {
	Pairs   []PUPair `yaml:"pairs"`
	Singles []PU     `yaml:"singles"`
}

type NodeInfo struct {
	IsSingleNode bool
	IsCPUNode    bool
	IsDPUNode    bool
	IsMyCPUNode  bool
	DPUIp        string
}

func ReadClusterConfigYaml(filePath string) ClusterConfig {
	if read {
		return offmeshCluster
	}
	var err error
	file, err := os.ReadFile(filePath)
	if err != nil {
		klog.Errorf("read cluster conf yaml error: %v", err)
	}
	err = yaml.Unmarshal(file, &offmeshCluster)
	if err != nil {
		klog.Errorf("unmarshal cluster conf yaml error: %v", err)
	}
	read = true
	return offmeshCluster
}

func GetPairNode(nodeName string, offmeshCluster ClusterConfig) PU {
	if NodeType(nodeName, offmeshCluster) == CPUNode {
		for _, pair := range offmeshCluster.Pairs {
			if pair.CPUName == nodeName {
				return PU{IP: pair.DPUIp, Name: pair.DPUName}
			}
		}
		return PU{}
	} else {
		for _, pair := range offmeshCluster.Pairs {
			if pair.DPUName == nodeName {
				return PU{IP: pair.CPUIp, Name: pair.CPUName}
			}
		}
		return PU{}
	}
}

func NodeType(NodeName string, offmeshCluster ClusterConfig) string {
	for _, pair := range offmeshCluster.Pairs {
		if pair.CPUName == NodeName {
			return CPUNode
		}
		if pair.DPUName == NodeName {
			return DPUNode
		}
	}
	return ""
}
