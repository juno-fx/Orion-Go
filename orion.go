package oriongo

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// These are vars so we can override them for testing etc
var (
	namespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	tokenPath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// These custom type and function are here to facilitate a easier time setting up and testing
type k8sClientFuncType = func() (*kubernetes.Interface, error)

var k8sClientFunc = newK8sClient

// Client is a struct holding all runtime information for communication with orion services
type Client struct {
	k8sClient          kubernetes.Interface
	namespace          string
	serviceAccountName string
}

func Setup() (*Client, error) {

	k8sClient, err := k8sClientFunc()
	if err != nil {
		return nil, err
	}

	namespace, err := getNamespace()
	if err != nil {
		return nil, err
	}

	serviceAccountName, err := getServiceAccountName()
	if err != nil {
		return nil, err
	}

	orionClient := &Client{
		k8sClient:          k8sClient,
		namespace:          namespace,
		serviceAccountName: serviceAccountName,
	}

	return orionClient, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return nil, nil
}

func newK8sClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	k8sClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return k8sClientset, nil
}

func getNamespace() (string, error) {
	namespace, set := os.LookupEnv("NAMESPACE")
	if !set {
		// K8s mounts the namespace at this path inside every pod
		data, err := os.ReadFile(namespacePath)
		if err != nil {
			return "", fmt.Errorf("failed to read namespace: %w", err)
		}
		namespace = string(data)
	}
	return namespace, nil
}

func getServiceAccountName() (string, error) {
	tokenData, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{}

	token, _, err := jwt.NewParser().ParseUnverified(string(tokenData), claims)
	if err != nil {
		return "", err
	}

	subject, err := token.Claims.GetSubject()
	if err != nil {
		return "", err
	}

	if parts := strings.Split(subject, ":"); len(parts) >= 4 {
		return parts[3], nil
	}
	return "default", nil
}
