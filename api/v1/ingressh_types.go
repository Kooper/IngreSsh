/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngreSshSpec defines the desired state of IngreSsh
// Ingress for ssh configures access to pods through SSH
// server running in the cluster. Users, authorized with their public keys,
// can establish SSH connection with the pods accordingly to the configured
// pods selectors.
// Ingress SSH resources are namespace-scoped.
type IngreSshSpec struct {

	// Session specifies the mechanism to use for the SSH session of this
	// ingress resource: exec in container (Exec) or ephemeral container (Debug)
	// Debug is the default.
	// +kubebuilder:validation:Enum=Debug;Exec
	// +optional
	Session string `json:"session,omitempty"`

	// Image for the ephemeral container. If not specified the default from the
	// server configuration is used. The option is relevant for the Debug
	// type sessions. For the Exec type sessions it has no effect.
	// +optional
	Image string `json:"image,omitempty"`

	// A command to execute as the login shell for the SSH session. This will
	// run in interactive mode when the user executes `ssh cluster` command.
	//
	// For the Debug session mode it sets entrypoint array for the docker
	// image of the ephermeral container. See the description of the
	// corresponding field in the ephemeral container spec
	// (https://github.com/kubernetes/api/blob/master/core/v1/types.go)
	// If not specified, an entrypoint of the docker image of the ephemeral
	// container will be used.
	//
	// For the Exec session mode functions like a login shell for the user.
	//
	// If the user specifies command as a part of the ssh connect string (f.e.
	// `ssh cluster ls -l`), the specified command will be used instead of the
	// login shell in the Exec session mode. For the Debug session mode an
	// ephemeral container will be started with the entrypoint defined in
	// this configuration, and then the specified command will be used in
	// scope of the SSH session.
	//
	// Please note that SSH does not set up terminal when running the command
	// specified via command line. If the user runs `ssh cluster /bin/bash`
	// there will be no normal terminal support. It is OK for non-interactive
	// commands like `ssh cluster ls -l`
	//
	// This means that although in theory you may not specify the command here,
	// in practice you would like to set it up to allow interactive sessions
	// in the Exec session mode.
	//
	// +optional
	Command []string `json:"command,omitempty"`

	// Arguments to the entrypoint.
	// The image's CMD is used if this is not provided.
	// See the description of corresponding field in the ephemeral container
	// spec (https://github.com/kubernetes/api/blob/master/core/v1/types.go)
	// +optional
	Args []string `json:"args,omitempty"`

	// Container's working directory to drop SSH session to.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// +optional
	WorkingDir string `json:"workingDir,omitempty"`

	// Selectors define target pods to authorize SSH session to.
	// If not specified, all pods could be accessed by the authorized user.
	// A user can specify one of the authorized pods as the login part
	// of SSH connection string, like `ssh pod-name@cluster /bin/bash`
	// As ingress SSH resources are namespace-scoped, selectors are matched
	// against pods in the resource's namespace.
	// +optional
	Selectors []string `json:"selectors,omitempty"`

	// If specified, containers define the list of container names to attach
	// SSH session to. The first container in the target pod, which matches one
	// of the container names in the list, will be attached. If the target pod
	// contains none of the specified container names session can not be
	// created.
	//
	// If not specified, all containers can be attached.
	//
	// A user can specify the container to attach as part of the login
	// part of the the SSH connection command, like
	// `ssh namespace:pod:container@cluster` where the namespace and pod parts
	// can be omitted: `ssh ::container@cluster`
	//
	// +optional
	Containers []string `json:"containers,omitempty"`

	// AuthorizedKeys is a set of public keys to authorize login
	// The keys are specified in the same format as lines in the
	// .ssh/authorized_keys file
	//
	// +kubebuilder:validation:MinItems=1
	AuthorizedKeys []string `json:"authorizedKeys"`
}

// IngreSshStatus defines the observed state of IngreSsh
type IngreSshStatus struct {
	// A list of pointers to currently running jobs.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// Information when was the last time the ssh session was opened.
	// +optional
	LastlogTime *metav1.Time `json:"lastlogTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=ingresshes

// IngreSsh is the Schema for the ingresshes API
type IngreSsh struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IngreSshSpec   `json:"spec,omitempty"`
	Status IngreSshStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IngreSshList contains a list of IngreSsh
type IngreSshList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IngreSsh `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IngreSsh{}, &IngreSshList{})
}
