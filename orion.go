package oriongo

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// These are vars so we can override them for testing etc
var (
	NamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	TokenPath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// K8sClientFuncType is a function to facilitate a easier time setting up and testing
type K8sClientFuncType = func() (kubernetes.Interface, error)

// K8sClientFunc is a custom type to facilitate a easier time setting up and testing
var K8sClientFunc K8sClientFuncType = newK8sClient

// Client is a struct holding all runtime information for communication with orion services
type Client struct {
	k8sClient          kubernetes.Interface
	namespace          string
	serviceAccountName string
	cache              map[string]cacheItem
	cacheLock          sync.Mutex
}

// HTTPClient is the client used to make the http call, it is overridden in unit tests
var HTTPClient = &http.Client{}

type cacheItem struct {
	token string
	exp   time.Time
}

func Setup() (*Client, error) {
	k8sClient, err := K8sClientFunc()
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
		cache:              map[string]cacheItem{},
		cacheLock:          sync.Mutex{},
	}

	return orionClient, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	service, namespace, err := getNamespaceService(req.URL.String())
	if err != nil {
		return nil, err
	}

	token, err := c.getToken(namespace, service)
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-ORION-SERVICE-AUTH", token)

	return HTTPClient.Do(req)
}

func newK8sClient() (kubernetes.Interface, error) { // coverage-ignore
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
		data, err := os.ReadFile(NamespacePath)
		if err != nil {
			return "", fmt.Errorf("failed to read namespace: %w", err)
		}

		namespace = string(data)
	}

	return namespace, nil
}

func getServiceAccountName() (string, error) {
	tokenData, err := os.ReadFile(TokenPath)
	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{}

	token, _, err := jwt.NewParser().ParseUnverified(string(tokenData), claims)
	if err != nil {
		return "", err
	}

	subject, err := token.Claims.GetSubject()
	if err != nil { // coverage-ignore
		return "", err
	}

	if parts := strings.Split(subject, ":"); len(parts) >= 4 {
		return parts[3], nil
	}

	return "default", nil
}

func getNamespaceService(url string) (string, string, error) {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	splitURL := strings.Split(url, ".")

	// --------0---------1-----------2--3-------4-----------
	// http://{service}.{namespace}.svc.cluster.local:{port}
	if len(splitURL) < 4 {
		return "", "", errors.New("request url is malformed")
	}

	return splitURL[0], splitURL[1], nil
}
