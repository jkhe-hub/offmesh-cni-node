package main

import (
	"offmesh-cni-node/offmesh"
	"github.com/dapr/dapr/utils"
	"github.com/eapache/queue"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	kubeClient          *kubernetes.Clientset
	offmeshCluster      offmesh.ClusterConfig
	NodeName            = os.Getenv("NODE_NAME")
	addNetworkQueue     = queue.New()
	addNetworkQueueLock = sync.Mutex{}
)

func main() {
	offmeshCluster = offmesh.ReadClusterConfigYaml(offmesh.ClusterConfigYamlPath)
	kubeClient = utils.GetKubeClient()
	informer := informers.NewSharedInformerFactoryWithOptions(kubeClient, 0).Core().V1().Pods().Informer()
	informer.AddEventHandler(EventHandler())
	stopper := make(chan struct{}, 2)
	go informer.Run(stopper)
	log.Println("watch pod started...")
	if offmesh.NodeType(NodeName, offmeshCluster) == offmesh.CPUNode {
		go AddNetworkRules(stopper, AddRedirectInWorkerPod)
	} else if offmesh.NodeType(NodeName, offmeshCluster) == offmesh.DPUNode {
		go AddNetworkRules(stopper, AddRedirectInDaprPod)
	}
	log.Println("add-network-rules started")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	stopper <- struct{}{}
	close(stopper)
	log.Println("watch pod stopped...")
}
