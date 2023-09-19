package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type Client interface {
	Namespaces() ([]string, error)
	Pods(selector string, namespace string, hint string) ([]corev1.Pod, error)
}

type ClientImpl struct {
	ctx    context.Context
	client kubernetes.Interface
	cfg    *rest.Config
}

// TODO examples are using the following code to load configuration:
// kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
//
//	clientcmd.NewDefaultClientConfigLoadingRules(),
//	&clientcmd.ConfigOverrides{},
//
// )
// restCfg, err := kubeCfg.ClientConfig()
//
//	if err != nil {
//		return "", "", err
//	}
func (c *ClientImpl) Init(cfg *rest.Config) error {
	c.cfg = cfg
	client, err := kubernetes.NewForConfig(c.cfg)
	if err != nil {
		return err
	}

	c.ctx = context.Background()

	// Make a test API call
	_, err = client.CoreV1().Namespaces().List(c.ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	c.client = client
	return nil
}

func (c *ClientImpl) V1() v1.CoreV1Interface {
	return c.client.CoreV1()
}

// Pods finds target pods using the selector and namespace. If hint is not
// empty, only the pod with the hinted name will be returned, if authorized
// by the selector.
//
// Returns slice of pod objects or error if there is internal error.
//
// If selector is empty string - no selector filtering is applied.
func (c *ClientImpl) Pods(selector string, namespace string, hint string) (
	[]corev1.Pod, error,
) {

	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	if hint != "" {
		listOptions.FieldSelector = fmt.Sprintf("metadata.name=%s", hint)
	}

	pods, err := c.V1().Pods(namespace).List(c.ctx, listOptions)
	if err != nil {
		return []corev1.Pod{}, err
	}

	return pods.Items, nil
}

// Returns the list of namespaces in the cluster.
func (c *ClientImpl) Namespaces() ([]string, error) {
	nss, err := c.V1().Namespaces().List(c.ctx, metav1.ListOptions{})
	if err != nil {
		return []string{}, err
	}
	result := make([]string, 0, len(nss.Items))
	for _, ns := range nss.Items {
		result = append(result, ns.Name)
	}
	return result, nil
}

func (c *ClientImpl) Ctx() context.Context {
	return c.ctx
}
