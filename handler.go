package offmesh_cni_node

import (
	"github.com/jkhe-hub/offmesh-cni-node/offmesh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"github.com/dapr/dapr/pkg/injector/sidecar"
	"log"
)

func IsWorkerPod(pod *corev1.Pod) bool {
	return sidecar.Annotations(pod.Annotations).GetBoolOrDefault("dapr.io/enabled", false)
}
func IsDaprPod(pod *corev1.Pod) bool {
	return sidecar.Annotations(pod.Annotations).GetBoolOrDefault("offmesh/is-sidecar", false)
}

func DPUNodeEventHandler() *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)
			if !IsDaprPod(newPod) || NodeName != newPod.Spec.NodeName {
				return
			}
			allReady := true
			for _, ctrStatus := range oldPod.Status.ContainerStatuses {
				allReady = allReady && ctrStatus.Ready
			}
			if allReady {
				log.Printf("%s oldPod all ready", newPod.ObjectMeta.Name)
				return
			}
			allReady = true
			for _, ctrStatus := range newPod.Status.ContainerStatuses {
				allReady = allReady && ctrStatus.Ready
			}
			if allReady {
				log.Printf("%s add to queue", newPod.ObjectMeta.Name)
				addNetworkQueueLock.Lock()
				addNetworkQueue.Add(newPod)
				addNetworkQueueLock.Unlock()
			}
		},
	}
}

func CPUNodeEventHandler() *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			log.Printf("[OnAdd] pod name: %s, NodeName:%s, dapr.io/enabled: %s, offmesh/is-sidecar:%s \n", pod.ObjectMeta.Name, pod.Spec.NodeName, sidecar.Annotations(pod.Annotations)["dapr.io/enabled"], sidecar.Annotations(pod.Annotations)["offmesh/is-sidecar"])
			if pod.Spec.NodeName == NodeName && IsWorkerPod(pod) {
				log.Println("[HandlePodAdd] handling dapr worker pod, name: ", pod.ObjectMeta.Name)
				StartSidecarPod(pod)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)
			if !IsWorkerPod(newPod) || newPod.Spec.NodeName != NodeName {
				return
			}
			log.Printf("[OnUpdate] pod name: %s \n", oldPod.ObjectMeta.Name)
			if oldPod.Spec.NodeName == "" {
				log.Printf("[OnUpdate] HandlePodAdd \n")
				StartSidecarPod(newPod)
				return
			}
			allReady := true
			for _, ctrStatus := range oldPod.Status.ContainerStatuses {
				allReady = allReady && ctrStatus.Ready
			}
			if allReady {
				log.Printf("%s oldPod all ready", newPod.ObjectMeta.Name)
				return
			}
			allReady = true
			for _, ctrStatus := range newPod.Status.ContainerStatuses {
				allReady = allReady && ctrStatus.Ready
			}
			if allReady {
				log.Printf("%s add to queue", newPod.ObjectMeta.Name)
				addNetworkQueueLock.Lock()
				addNetworkQueue.Add(newPod)
				addNetworkQueueLock.Unlock()
			}

		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			log.Println("[OnDelete] pod name: ", pod.ObjectMeta.Name)
			if pod.Spec.NodeName == NodeName && IsWorkerPod(pod) {
				DeleteSidecarPod(pod)
			}
		},
	}
}

func EventHandler() *cache.ResourceEventHandlerFuncs {
	if offmesh.NodeType(NodeName, offmeshCluster) == offmesh.CPUNode {
		return CPUNodeEventHandler()
	} else if offmesh.NodeType(NodeName, offmeshCluster) == offmesh.DPUNode {
		return DPUNodeEventHandler()
	}
	return nil
}
