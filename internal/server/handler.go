package server

import (
	"fmt"

	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"

	"kuberstein.io/ingressh/internal/k8s"
	"kuberstein.io/ingressh/internal/types"
)

// GetHandler returns SSH connection handler for the SSH server.
// The user is authorized at this moment, the list of authorized configurations
// is stored in the session context.
func GetHandler(kube *k8s.ClientImpl, conf *types.ServerConfig) func(sess ssh.Session) {

	return func(sess ssh.Session) {

		// User may hint the target route with login name of SSH session.
		hint := types.SshTarget{}
		hint.InitFromUsername(sess.User())

		targetAuth := GetAuthz(GetSshConfigsFromCtx(sess.Context()), kube)

		var target types.SshTarget
		var targetPodConfig podSshConfig
		var err error
		_, _, isPty := sess.Pty()

		// Interactive selection makes sense only when there is a terminal
		// and the user didn't specify all the components of the target
		// to connect to.
		if isPty && !hint.IsComplete() {
			target, targetPodConfig, err = interactive(sess, targetAuth, hint)
		} else {
			target, targetPodConfig, err = automatic(sess, targetAuth, hint)
		}
		if err != nil {
			fmt.Fprintf(sess, "Error: %s\n", err)
			sess.Exit(10)
			return
		}
		if !target.IsComplete() {
			fmt.Fprintf(sess, "No container selected\n")
			sess.Exit(13)
			return
		}

		targetConfig := targetPodConfig.config
		targetConfig.ApplyDefaults(*conf)
		pod := targetPodConfig.pod

		fmt.Fprintf(sess, "Pod has been found. Connecting your SSH session to %s/%s container %s...\n",
			pod.Namespace, pod.Name, target.Container)

		// Session attach options vary depending on the mode
		if targetConfig.Session == "Exec" {
			command := targetConfig.Command
			if len(sess.Command()) > 0 {
				command = sess.Command()
			}
			if len(command) == 0 {
				// In the Exec mode there is no default command to run like
				// in the Debug mode, where the docker image entry point
				// could be used.
				fmt.Fprintf(sess, "Command is not specified\n")
				sess.Exit(2)
				return
			}
			log.Infof("Executing %v in the container %s", command, target.Container)
			err := k8s.ExecInContainer(kube, &pod, target.Container, sess, command)
			if err != nil {
				log.Errorln(err)
				sess.Exit(3)
				return
			}
		} else {
			// debug session mode
			pod, accessContainerName, err := k8s.AttachAccessContainer(
				kube, &pod, target.Container, targetConfig)
			if err != nil {
				log.Errorln(err)
				sess.Exit(2)
				return
			}

			if len(sess.Command()) > 0 {
				// Execute command in the running debug container
				log.Infof("Executing %v in the ephemeral container %s", sess.Command(), accessContainerName)
				err = k8s.ExecInContainer(kube, pod, accessContainerName, sess, sess.Command())
				if err != nil {
					log.Errorln(err)
					sess.Exit(3)
					return
				}
			} else {
				// Attach terminal session to the running debug container
				log.Infof("Attaching SSH session into the container %s", accessContainerName)
				err = k8s.AttachSshSessionTerminal(kube, pod, accessContainerName, sess)
				if err != nil {
					log.Errorln(err)
					sess.Exit(3)
					return
				}
			}
		}

		sess.Exit(0)
	}
}
