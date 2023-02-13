package offmesh_cni_node

import (
	"bytes"
	"context"
	"fmt"
	"github.com/dapr/dapr/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/remotecommand"
	"log"
	"os/exec"
	"time"
)

const (
	IPTABLES         = "iptables"
	OUTPUT_CHAIN     = "OUTPUT"
	PREROUTING_CHAIN = "PREROUTING"
	TABLE_NAT        = "nat"
	LOCALHOST        = "127.0.0.1"
	NSENTER          = "nsenter"
)

func AddNetworkRules(stopper chan struct{}, handler func(pod *corev1.Pod)) {
LOOP:
	for {
		select {
		case <-stopper:
			break LOOP
		default:
			if addNetworkQueue.Length() != 0 {
				addNetworkQueueLock.Lock()
				podObj := addNetworkQueue.Remove()
				addNetworkQueueLock.Unlock()
				pod := podObj.(*corev1.Pod)
				handler(pod)
			}
			time.Sleep(time.Second * 1)
		}
	}
}

func ExecuteInContainer(podName string, podNamespace string, cmd []string) (string, string, error) {
	req := kubeClient.CoreV1().RESTClient().Post().Resource("pods").Name(podName).
		Namespace(podNamespace).SubResource("exec")

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return "", "", fmt.Errorf("error adding to scheme: %v", err)
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}, parameterCodec)

	exec_, err := remotecommand.NewSPDYExecutor(utils.GetConfig(), "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error while creating Executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec_.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return "", "", fmt.Errorf("error in Stream: %v", err)
	}

	return stdout.String(), stderr.String(), nil
}

// iptables -A OUTPUT -t nat -p tcp --dst 127.0.0.1 --dport daprHttpPort -j DNAT --to-destination daprIP:daprHttpPort
// iptables -A OUTPUT -t nat -p tcp --dst 127.0.0.1 --dport daprGRPCPort -j DNAT --to-destination daprIP:daprGRPCPort
// iptables -A PREROUTING -t nat -p tcp --src daprIP -j SNAT --to-source 127.0.0.1

func AddNetworkRulesToPod(pod *corev1.Pod, daprIP string) {
	daprHttpPort := ""
	daprGRPCPort := ""
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "DAPR_HTTP_PORT"  {
			daprHttpPort = env.Value
		} else if env.Name == "DAPR_GRPC_PORT" {
			daprGRPCPort = env.Value
		}
	}
	cmds := [][]string{
		{
			"sh",
			"-c",
			IPTABLES,
			"-A", OUTPUT_CHAIN,
			"-t", TABLE_NAT,
			"-p", "tcp",
			"--dst", LOCALHOST,
			"--dport", daprHttpPort,
			"-j", "DNAT",
			"--to-destination", fmt.Sprintf("%s:%s", daprIP, daprHttpPort),
		},
		{
			"sh",
			"-c",
			IPTABLES,
			"-A", OUTPUT_CHAIN,
			"-t", TABLE_NAT,
			"-p", "tcp",
			"--dst", LOCALHOST,
			"--dport", daprGRPCPort,
			"-j", "DNAT",
			"--to-destination", fmt.Sprintf("%s:%s", daprIP, daprGRPCPort),
		},
		{
			"sh",
			"-c",
			IPTABLES,
			"-A", PREROUTING_CHAIN,
			"-t", TABLE_NAT,
			"-p", "tcp",
			"--src", daprIP,
			"-j", "SNAT",
			"--to-source", LOCALHOST,
		},
	}
	for _, cmd := range cmds {
		stdout, stderr, err := ExecuteInContainer(pod.ObjectMeta.Name, pod.ObjectMeta.Namespace, cmd)
		if err != nil {
			log.Println("[AddNetworkRulesToPod] kubectl exec error: ", err)
			return
		}
		log.Printf("[AddNetworkRulesToPod] kubectl exec stdout: %v, stderr: %v", stdout, stderr)
	}
}

func GetNetworkRulesInDaprPod(workerPodIp string, applicationPort string) [][]string {
	// iptables -A OUTPUT -t nat -p tcp --dst 127.0.0.1 --dport applicationPort -j DNAT --to-destination workerPodIp:applicationPort
	// iptables -A PREROUTING -t nat -p tcp --src workerPodIp -j SNAT --to-source 127.0.0.1
	return [][]string{
		{
			"sh",
			"-c",
			IPTABLES,
			"-A", OUTPUT_CHAIN,
			"-t", TABLE_NAT,
			"-p", "tcp",
			"--dst", LOCALHOST,
			"--dport", applicationPort,
			"-j", "DNAT",
			"--to-destination", fmt.Sprintf("%s:%s", workerPodIp, applicationPort),
		},
		{
			"sh",
			"-c",
			IPTABLES,
			"-A", PREROUTING_CHAIN,
			"-t", TABLE_NAT,
			"-p", "tcp",
			"--src", workerPodIp,
			"-j", "SNAT",
			"--to-source", LOCALHOST,
		},
	}
}

func GetNetworkRulesInWorkerPod(daprPodIP string, daprHttpPort string, daprGRPCPort string) [][]string {
	//iptables -A OUTPUT -t nat -p tcp --dst 127.0.0.1 --dport daprHttpPort -j DNAT --to-destination daprIP:daprHttpPort
	//iptables -A OUTPUT -t nat -p tcp --dst 127.0.0.1 --dport daprGRPCPort -j DNAT --to-destination daprIP:daprGRPCPort
	//iptables -A PREROUTING -t nat -p tcp --src daprIP -j SNAT --to-source 127.0.0.1
	return [][]string{
		{
			"sh",
			"-c",
			IPTABLES,
			"-A", OUTPUT_CHAIN,
			"-t", TABLE_NAT,
			"-p", "tcp",
			"--dst", LOCALHOST,
			"--dport", daprHttpPort,
			"-j", "DNAT",
			"--to-destination", fmt.Sprintf("%s:%s", daprPodIP, daprHttpPort),
		},
		{
			"sh",
			"-c",
			IPTABLES,
			"-A", OUTPUT_CHAIN,
			"-t", TABLE_NAT,
			"-p", "tcp",
			"--dst", LOCALHOST,
			"--dport", daprGRPCPort,
			"-j", "DNAT",
			"--to-destination", fmt.Sprintf("%s:%s", daprPodIP, daprGRPCPort),
		},
		{
			"sh",
			"-c",
			IPTABLES,
			"-A", PREROUTING_CHAIN,
			"-t", TABLE_NAT,
			"-p", "tcp",
			"--src", daprPodIP,
			"-j", "SNAT",
			"--to-source", LOCALHOST,
		},
	}
}

func ExecUsingNSEnter(pid string, cmd []string) error {
	args := append([]string{"-u", "-n", "-p", "-t", pid, "--"}, cmd...)
	command := exec.Command(NSENTER, args...)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command.Stdout = stdout
	command.Stderr = stderr
	log.Println("[ExecUsingNSEnter] cmd to run ", command.String())
	err := command.Run()
	if len(stdout.String()) != 0 {
		log.Printf("Command %v output: %v\n", command.String(), stdout.String())
	}
	if len(stderr.Bytes()) != 0 {
		log.Printf("Command %v error: %v\n", command.String(), stderr.String())
	}
	return err
}

func AddRedirectInDaprPod(pod *corev1.Pod) {
	workerPodIp := ""
	appPort := ""
	for {
		workerPod, _ := kubeClient.CoreV1().Pods(pod.ObjectMeta.Namespace).Get(context.Background(), GetWorkerPodName(pod), metav1.GetOptions{})
		if workerPod.Status.PodIP != "" {
			workerPodIp = workerPod.Status.PodIP
			appPort = pod.Annotations["dapr.io/app-port"]
			break
		}
	}
	ctrPID, err := GetContainerPIDByID(pod.Status.ContainerStatuses[0].ContainerID)
	if err != nil {
		log.Println("AddRedirectInDaprPod get ctr error", err)
	}
	for _, cmd := range GetNetworkRulesInDaprPod(workerPodIp, appPort) {
		err = ExecUsingNSEnter(ctrPID, cmd)
		if err != nil {
			log.Println("AddRedirectInDaprPod error", err)
		}
	}
}

func AddRedirectInWorkerPod(pod *corev1.Pod) {
	daprHttpPort := ""
	daprGRPCPort := ""
	daprPodIP := ""
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "DAPR_HTTP_PORT" {
			daprHttpPort = env.Value
		} else if env.Name == "DAPR_GRPC_PORT" {
			daprGRPCPort = env.Value
		}
	}
	for {
		daprPod, _ := kubeClient.CoreV1().Pods(pod.ObjectMeta.Namespace).Get(context.Background(), GetSidecarPodName(pod.ObjectMeta.Name), metav1.GetOptions{})
		if daprPod.Status.PodIP != "" {
			daprPodIP = daprPod.Status.PodIP
			break
		}
		time.Sleep(time.Second)
	}
	ctrPID, err := GetContainerPIDByID(pod.Status.ContainerStatuses[0].ContainerID)
	if err != nil {
		log.Println("AddRedirectInWorkerPod get ctr error", err)
	}
	for _, cmd := range GetNetworkRulesInWorkerPod(daprPodIP, daprHttpPort, daprGRPCPort) {
		err = ExecUsingNSEnter(ctrPID, cmd)
		if err != nil {
			log.Println("AddRedirectInWorkerPod error", err)
		}
	}

}
