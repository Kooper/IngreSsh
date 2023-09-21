package k8s

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"kuberstein.io/ingressh/internal/types"
)

// AttachAccessContainer attaches an ephemeral container to the pod, into the
// targetContainer Linux namespace.
//
// After the attachment it waits the container to be in Running state, then
// returns an updated pod and newly added container name.
//
// If the container is already attached and running - do nothing.
// If the container is already attached but completed - attaches a new one
func AttachAccessContainer(
	kube *ClientImpl,
	pod *v1.Pod,
	targetContainer string,
	config *types.SshConfig,
) (*v1.Pod, string, error) {

	const attachNameTmpl = "ssh-access-"

	// Check if there is already debug container in a good state which could be reused.

	// Sanity check as I'm not sure if it works this way but rely on this in the
	// reasoning of containers' status.
	if len(pod.Status.EphemeralContainerStatuses) != len(pod.Spec.EphemeralContainers) {
		log.Errorf("Can't detect ephemeral containers status: status and spec slices are different")
		return nil, "", fmt.Errorf("failed to detect container status")
	}

	// Find a running access container and the names used.
	usedIndexValues := []int{}
	for i := 0; i < len(pod.Status.EphemeralContainerStatuses); i++ {
		container := pod.Spec.EphemeralContainers[i]
		if !strings.HasPrefix(container.Name, attachNameTmpl) {
			continue
		}

		// Save the used name index for the future reference
		usedIndex, err := strconv.Atoi(container.Name[len(attachNameTmpl):])
		if err != nil {
			log.Warnf("Skip attach container name without proper numeric index: %s", container.Name)
		} else {
			usedIndexValues = append(usedIndexValues, usedIndex)
		}

		if container.TargetContainerName != targetContainer {
			continue
		}

		status := pod.Status.EphemeralContainerStatuses[i]
		if status.State.Running != nil {
			return pod, container.Name, nil
		}
	}

	// No running ephemeral containers could be reused, gonna create a new one.
	maxUsed := 0
	for _, v := range usedIndexValues {
		if v > maxUsed {
			maxUsed = v
		}
	}

	containerName := fmt.Sprintf("%s%d", attachNameTmpl, maxUsed+1)

	// Ephemeral container always starts with the command from the
	// configuration spec, not from the user's input.
	command := config.Command
	ephemeralContainer := getEphemeralContainerSpec(command, config, containerName, targetContainer)

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, *ephemeralContainer)
	pod, err := kube.V1().Pods(pod.Namespace).UpdateEphemeralContainers(kube.ctx, pod.Name, pod, metav1.UpdateOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("could not add ephemeral container: %w", err)
	}

	// Wait for the container to be in the running state
	err = waitAccessContainer(kube, pod, containerName)
	if err != nil {
		return nil, "", err
	}

	return pod, containerName, nil
}

// getEphemeralContainerSpec prepares resource spec for the new ephemeral
// container. It merges server default configuration, attach container
// configuration from the route and the options entered by the user via
// command-line of SSH command.
func getEphemeralContainerSpec(
	command []string,
	config *types.SshConfig,
	containerName string,
	targetContainer string,
) *corev1.EphemeralContainer {

	args := config.Args
	workdir := config.WorkingDir

	// trueValue := true
	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:  containerName,
			Image: config.Image,
			TTY:   true,
			Stdin: true,
			// Not specifying a security context should in theory run debug
			// pod with the context of the pod been attached. So skip the
			// security context for now.
			// SecurityContext: &corev1.SecurityContext{
			// 	Privileged:               &trueValue,
			// 	AllowPrivilegeEscalation: &trueValue,
			// },
			Command:    command,
			Args:       args,
			WorkingDir: workdir,
		},
		TargetContainerName: targetContainer,
	}

	return &ephemeralContainer
}

// waitAccessContainer waits until the ephemeral container containerName
// is in the Running state.
// Returns error if container is terminated or could not be found.
func waitAccessContainer(kube *ClientImpl, pod *v1.Pod, containerName string) error {

	watcher, err := kube.V1().Pods(pod.Namespace).Watch(kube.ctx, metav1.SingleObject(pod.ObjectMeta))
	if err != nil {
		return err
	}
	defer watcher.Stop()

	log.Infof("Watching the pod %s to wait attach container %s ready...", pod.Name, containerName)

	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Modified:
			pod = event.Object.(*corev1.Pod)

			containerFound := false
			for i, containerStatus := range pod.Status.EphemeralContainerStatuses {
				log.Debugf("Iterating over ephemeral container %s", pod.Spec.EphemeralContainers[i].Name)
				if containerName != pod.Spec.EphemeralContainers[i].Name {
					continue
				}
				containerFound = true
				if containerStatus.State.Running != nil {
					return nil
				}
				if containerStatus.State.Terminated != nil {
					return fmt.Errorf("pod %s has the attach container %s terminated", pod.Name, containerName)
				}
			}
			if !containerFound {
				return fmt.Errorf("pod %s does not have ephemeral container %s", pod.Name, containerName)
			}

		default:
			return fmt.Errorf("unexpected pod %s event type: %v", pod.Name, event)
		}
	}

	return nil
}

// ExecInContainer setups SSH session to run a shell in the container
//
// Terminal quirks:
//   - if the user specifies command via ssh - no terminal is allocated. The mode
//     is supposed for one-shot commands?
//   - Login shell command could be specified in Exec mode resource. In this case
//     all session streams should work. If the user specified the command
//     on the command line - then no terminal set. This means Exec mode resource
//     have to specify the command. What about Debug mode resource?
func ExecInContainer(kube *ClientImpl, pod *v1.Pod, containerName string, sess ssh.Session, command []string) error {

	request := kube.V1().RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(kube.cfg, "POST", request.URL())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(kube.Ctx())
	defer cancel()
	tty := TerminalSession{}
	tty.Init(sess, ctx)

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             sess,
		Stdout:            sess,
		Stderr:            sess,
		TerminalSizeQueue: &tty,
		Tty:               true,
	})

	if err != nil {
		return fmt.Errorf("%w failed executing command on %v/%v container %s",
			err, pod.Namespace, pod.Name, containerName)
	}
	return nil
}

// AttachSshSessionTerminal setups SSH session to run a shell in the container
func AttachSshSessionTerminal(kube *ClientImpl, pod *v1.Pod, containerName string, sess ssh.Session) error {

	request := kube.V1().RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("attach").
		VersionedParams(&v1.PodAttachOptions{
			Container: containerName,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(kube.cfg, "POST", request.URL())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(kube.Ctx())
	defer cancel()
	tty := TerminalSession{}
	tty.Init(sess, ctx)

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             sess,
		Stdout:            sess,
		Stderr:            sess,
		TerminalSizeQueue: &tty,
		Tty:               true,
	})

	if err != nil {
		return fmt.Errorf("%w failed executing shell on %v/%v container %s",
			err, pod.Namespace, pod.Name, containerName)
	}

	return nil
}
