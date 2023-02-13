package offmesh_cni_node

import (
	"context"
	"time"
	"k8s.io/kubernetes/pkg/kubelet/cri/remote"
	"log"
)

const (
	RootMountPath        = "/host/root"
	ContainerdSocketPath = "/run/containerd/containerd.sock"
	Unix                 = "unix://"
	defaultTimeout       = 2 * time.Second
	PID                  = "pid"
)

func GetContainerPIDByID(id string) (string, error) {
	service, err := remote.NewRemoteRuntimeService(Unix+RootMountPath+ContainerdSocketPath, defaultTimeout, nil)
	if err != nil {
		return "", err
	}
	status, err := service.ContainerStatus(context.TODO(), id, true)
	if err != nil {
		return "", err
	}
	log.Printf("GetContainerPIDByID ctr info: %v", status.Info)
	return status.Info[PID], nil
}
