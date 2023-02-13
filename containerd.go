package main

import (
	"k8s.io/kubernetes/pkg/kubelet/cri/remote"
	"time"
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
	status, err := service.ContainerStatus(id, true)
	if err != nil {
		return "", err
	}
	return status.Info[PID], nil
}
