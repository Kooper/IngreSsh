package server

import (
	"errors"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"

	"kuberstein.io/ingressh/internal/k8s"
	"kuberstein.io/ingressh/internal/types"
)

var (
	// ErrAuthorizationFailed is returned if the user is not authorized
	// to access the specified object.
	ErrAuthorizationFailed = errors.New("authorization failed")
)

// podSshConfig is a tuple binding together target pod and corresponding
// SSH config, as a result of the authorization.
type podSshConfig struct {
	pod    corev1.Pod
	config *types.SshConfig
}

// authz is an authorization engine.
type authz struct {
	authorizedConfigs []*types.SshConfig
	kube              k8s.Client
}

func GetAuthz(configs []*types.SshConfig, kube k8s.Client) authz {
	return authz{
		authorizedConfigs: configs,
		kube:              kube,
	}
}

func (a authz) getClusterNamespaces() (map[string]bool, error) {
	nss, err := a.kube.Namespaces()
	if err != nil {
		return map[string]bool{}, err
	}
	namespaces := map[string]bool{}
	for _, n := range nss {
		namespaces[n] = true
	}
	return namespaces, nil
}

// GetNamespaces returns the list of namespaces user is authorized to access.
//
// If hint is specified and the user is authorized to access the hinted
// namespace, the return slice contains only the specified namespace.
//
// If the hinted namespace is not authorized, the method returns "not
// authorized" error.
// If the hinted namespace doesn't exist, returns empty list
func (a authz) GetNamespaces(hintNs string) ([]string, error) {

	authorized := map[string]bool{}
	clusterNamespaces, err := a.getClusterNamespaces()
	if err != nil {
		return []string{}, err
	}

	for _, c := range a.authorizedConfigs {

		// Skip configuration entries that are not found in the cluster
		if _, ok := clusterNamespaces[c.Namespace]; !ok {
			continue
		}

		if hintNs == "" {
			authorized[c.Namespace] = true
		} else if hintNs == c.Namespace {
			// If we are interested only in a single namespace specified
			// with the hint, search no more.
			authorized[hintNs] = true
			break
		}
	}

	if len(authorized) > 0 {
		return maps.Keys(authorized), nil
	}

	if hintNs != "" {
		// Check either user has no access to the hinted namespace, or there is no
		// hinted namespace in the cluster at all.
		if _, ok := clusterNamespaces[hintNs]; ok {
			return []string{}, ErrAuthorizationFailed
		}
	}

	return []string{}, nil
}

// GetPods returns a list of pods from the specified namespace user is
// authorized to access.
//
// If hint is specified and the user is authorized to access the hinted
// pod, the return slice contains only the specified pod.
//
// If the hinted pod is not authorized, the method returns "not
// authorized" error.
// If the hinted pod doesn't exist, returns empty list
func (a authz) GetPods(namespace string, hintPod string) ([]podSshConfig, error) {

	clusterNamespaces, err := a.getClusterNamespaces()
	if err != nil {
		return []podSshConfig{}, err
	}

	relevantConfigs := []*types.SshConfig{}
	for _, c := range a.authorizedConfigs {
		if _, ok := clusterNamespaces[c.Namespace]; !ok {
			continue
		}
		if c.Namespace == namespace {
			relevantConfigs = append(relevantConfigs, c)
		}
	}

	result, err := a.listPods(relevantConfigs, hintPod, true)
	if err != nil {
		return []podSshConfig{}, err
	}
	if len(result) > 0 {
		return result, nil
	}

	if hintPod != "" {
		// Here the result is empty and hint isn't empty.
		// It means that either namespace doesn't contain such authorized pods,
		// or user is not authorized to access the hinted one.
		// How to check? Get hinted pod without the authorization selectors.
		// If empty result - this is the first situation, otherwise the second one.
		//
		// In fact, if there is no such pod - authorization error is retured
		// Is it OK???
		result, err = a.listPods(relevantConfigs, hintPod, false)
		if err != nil {
			return []podSshConfig{}, err
		}
		if len(result) > 0 {
			return []podSshConfig{}, ErrAuthorizationFailed
		}
	}

	return []podSshConfig{}, nil
}

func (a authz) listPods(configs []*types.SshConfig, hintPod string, useSelectors bool) ([]podSshConfig, error) {

	result := []podSshConfig{}

	// Functions to appends pods to the result set, checking for duplicates
	deduplicatePods := map[string]bool{}
	appendResult := func(pods []corev1.Pod, c *types.SshConfig) {
		for _, pod := range pods {
			if _, ok := deduplicatePods[pod.Name]; !ok {
				result = append(result, podSshConfig{pod: pod, config: c})
				deduplicatePods[pod.Name] = true
			}
		}
	}

	for _, c := range configs {
		if len(c.Selectors) == 0 || !useSelectors {
			// No sense to check the rest of configs, as a config without
			// the selector scans the whole namespace for pods
			pods, err := a.kube.Pods("", c.Namespace, hintPod)
			if err != nil {
				return []podSshConfig{}, err
			}
			appendResult(pods, c)
			break
		}

		for _, selector := range c.Selectors {
			pods, err := a.kube.Pods(selector, c.Namespace, hintPod)
			if err != nil {
				return []podSshConfig{}, err
			}
			appendResult(pods, c)
		}
	}
	return result, nil
}

// GetContainers returns a list of containers from the specified pod user is
// authorized to access.
//
// If hint is specified and the user is authorized to access the hinted
// container, the return slice contains only the specified container.
//
// If the hinted container is not authorized, the method returns "not
// authorized" error.
// If the hinted container doesn't exist, returns empty list.
func (a authz) GetContainers(pod corev1.Pod, restrictList []string, hintContainer string) ([]string, error) {

	result := []string{}

	for _, c := range pod.Spec.Containers {

		// If no hint specified we are ok with the container name from the
		// allowed configuration list.
		// If the hint has been specified we only accept the container with the
		// hinted name among the authorized set.

		if len(restrictList) == 0 {
			if hintContainer == "" {
				result = append(result, c.Name)
			} else if c.Name == hintContainer {
				return []string{c.Name}, nil
			}
		} else {
			for _, t := range restrictList {
				if c.Name == t {
					if hintContainer == "" {
						result = append(result, c.Name)
					} else if c.Name == hintContainer {
						return []string{c.Name}, nil
					}
				}
			}
		}
	}

	if len(result) > 0 {
		return result, nil
	}

	if hintContainer != "" {
		// At this point, the hint was specified and we are with the empty
		// result set. To distinguish "not authorized" and "no objects" situation
		// we'll see what was the result without hint applied.
		for _, c := range pod.Spec.Containers {
			if len(restrictList) == 0 {
				result = append(result, c.Name)
			} else {
				for _, t := range restrictList {
					if c.Name == t {
						result = append(result, c.Name)
					}
				}
			}
		}
		if len(result) > 0 {
			return []string{}, ErrAuthorizationFailed
		}
	}

	return []string{}, nil
}
