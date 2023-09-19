package server

import (
	"fmt"

	"github.com/gliderlabs/ssh"
	"kuberstein.io/ingressh/internal/types"
)

// Returns attach target and pod+configuration as a result of the
// automatic selection.
func automatic(sess ssh.Session, targetAuth authz, hint types.SshTarget) (
	types.SshTarget, podSshConfig, error,
) {

	var target types.SshTarget
	var targetPodConfig podSshConfig

	fmt.Fprintf(sess, "Hello %s, please wait while we are searching pods to set SSH connection to\n", sess.User())
	fmt.Fprintf(sess, "Note that at present you will connect to the first authorized pod\n")

	namespaces, err := targetAuth.GetNamespaces(hint.Namespace)
	if err != nil {
		return types.SshTarget{}, podSshConfig{}, err
	}
	if len(namespaces) == 0 {
		return types.SshTarget{}, podSshConfig{}, nil
	}

	target.Namespace = namespaces[0]
	podConfigs, err := targetAuth.GetPods(target.Namespace, hint.Pod)
	if err != nil {
		return target, podSshConfig{}, err
	}
	if len(podConfigs) == 0 {
		return target, podSshConfig{}, nil
	}

	targetPodConfig = podConfigs[0]
	target.Pod = targetPodConfig.pod.Name
	containers, err := targetAuth.GetContainers(targetPodConfig.pod, targetPodConfig.config.Containers, hint.Container)
	if err != nil {
		return target, targetPodConfig, err
	}
	if len(containers) == 0 {
		return target, targetPodConfig, nil
	}

	target.Container = containers[0]
	return target, targetPodConfig, nil
}
