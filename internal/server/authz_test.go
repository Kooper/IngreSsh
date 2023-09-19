package server

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	ingssh "kuberstein.io/ingressh/api/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kuberstein.io/ingressh/internal/types"
)

// Mocking only operations with namespaces for TestNamespace* tests
type clientNamespacesMock struct {
	namespaces []string
	err        error
}

func (c clientNamespacesMock) Namespaces() ([]string, error) {
	return c.namespaces, c.err
}
func (c clientNamespacesMock) Pods(selector string, namespace string, hint string) ([]corev1.Pod, error) {
	return []corev1.Pod{}, nil
}

func TestNamespaceAccess(t *testing.T) {

	authorizedConfigs := []*types.SshConfig{
		{Namespace: "authorized-ns1"},
		{Namespace: "authorized-ns2"},
		{Namespace: "broken-config-ns1"},
	}
	kube := clientNamespacesMock{
		namespaces: []string{
			"authorized-ns1",
			"authorized-ns2",
			"non-authorized-ns1",
		},
	}
	a := authz{
		authorizedConfigs: authorizedConfigs,
		kube:              kube,
	}

	tests := []struct {
		input  string
		result []string
		err    error
	}{
		{input: "", result: []string{"authorized-ns1", "authorized-ns2"}, err: nil},
		{input: "authorized-ns1", result: []string{"authorized-ns1"}, err: nil},
		{input: "non-existing-ns1", result: []string{}, err: nil},
		{input: "broken-config-ns1", result: []string{}, err: nil},
		{input: "non-authorized-ns1", result: []string{}, err: ErrAuthorizationFailed},
	}

	for _, tc := range tests {
		namespaces, err := a.GetNamespaces(tc.input)
		sort.Strings(namespaces)
		if !reflect.DeepEqual(namespaces, tc.result) {
			t.Errorf("Did not return authorized-namespace. Returned %v instead", namespaces)
		}
		if err != tc.err {
			t.Errorf("Unexpected error when returning authorized namespace: %v", err)
		}
	}
}

func TestNamepsaceEmptyResultWhenError(t *testing.T) {
	authorizedConfigs := []*types.SshConfig{
		{Namespace: "authorized-ns1"},
	}
	kube := clientNamespacesMock{
		namespaces: []string{"authorized-ns1", "non-authorized-ns1"},
		err:        errors.New("Client request error"),
	}
	a := authz{authorizedConfigs: authorizedConfigs, kube: kube}

	namespaces, err := a.GetNamespaces("authorized-ns1")
	if len(namespaces) > 0 || err == nil {
		t.Errorf("Didn't return error result: %v %v", namespaces, err)
	}
}

// Mocking operations with pods for TestPod* tests
type clientPodMock struct {
	namespaces []string
	pods       []struct {
		pod      corev1.Pod
		selector string
	}
	err error
}

func (c clientPodMock) Namespaces() ([]string, error) {
	return c.namespaces, c.err
}
func (c clientPodMock) Pods(selector string, namespace string, hint string) ([]corev1.Pod, error) {
	r := []corev1.Pod{}
	for _, p := range c.pods {
		if selector != "" && p.selector != selector {
			continue
		}
		if hint != "" && p.pod.Name != hint {
			continue
		}
		r = append(r, p.pod)
	}
	return r, nil
}

func TestPodAccess(t *testing.T) {

	// Only pods with the "app=name" selector authorized
	configNs1 := types.SshConfig{
		IngreSshSpec: ingssh.IngreSshSpec{Selectors: []string{"app=name"}},
		Namespace:    "authorized-ns1",
	}
	authorizedConfigs := []*types.SshConfig{&configNs1}

	// Two authorized pods and one pod which is not authorized
	kube := clientPodMock{
		pods: []struct {
			pod      corev1.Pod
			selector string
		}{
			{
				pod:      corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "name1"}},
				selector: "app=name",
			},
			{
				pod:      corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "name2"}},
				selector: "app=name",
			},
			{
				pod:      corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "non-authorized"}},
				selector: "some-other-selector",
			},
		},
		namespaces: []string{"authorized-ns1"},
	}

	a := authz{
		authorizedConfigs: authorizedConfigs,
		kube:              kube,
	}

	// Get all the pods if no hint specified. Both results should point to the
	// same config object
	configs, err := a.GetPods("authorized-ns1", "")
	if err != nil {
		t.Errorf("Unexpected error when getting pod: %v", err)
	}
	if len(configs) != 2 {
		t.Error("Should be 2 pods authorized")
	}
	if !reflect.DeepEqual(*(configs[0].config), configNs1) {
		t.Errorf("Didn't find the authorized pod. Returned %v instead of %v", *(configs[0].config), configNs1)
	}
	if !reflect.DeepEqual(*(configs[1].config), configNs1) {
		t.Errorf("Didn't find the authorized pod. Returned %v instead of %v", *(configs[1].config), configNs1)
	}

	// Get the exact pod if hint specified
	configs, err = a.GetPods("authorized-ns1", "name1")
	if err != nil {
		t.Errorf("Unexpected error when getting pod: %v", err)
	}
	if len(configs) != 1 {
		t.Error("Should be 1 pod authorized")
	}
	if !reflect.DeepEqual(*(configs[0].config), configNs1) {
		t.Errorf("Didn't find the authorized pod. Returned %v instead of %v", *(configs[0].config), configNs1)
	}

	// Get empty set when hint non-existing pod
	configs, err = a.GetPods("authorized-ns1", "no-such-pod-name")
	if err != nil {
		t.Errorf("Unexpected error when getting pod: %v", err)
	}
	if len(configs) > 0 {
		t.Errorf("Must return empty result, returned %v instead", configs)
	}

	// Get authorization error if hint non-authorized pod
	configs, err = a.GetPods("authorized-ns1", "non-authorized")
	if err != ErrAuthorizationFailed {
		t.Errorf("Expected error when getting non-authorized pod, got: %v", err)
	}
	if len(configs) > 0 {
		t.Errorf("Must return empty result, returned %v instead", configs)
	}
}
