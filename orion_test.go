package oriongo_test

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	oriongo "orion-go"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestNewClient(t *testing.T) {
	tokenFile, nsFile := setupTest(t)

	defer func() {
		require.NoError(t, os.Remove(tokenFile))
		require.NoError(t, os.Remove(nsFile))
	}()

	client, err := oriongo.Setup()
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestNewClientErrs(t *testing.T) {
	oriongo.K8sClientFunc = setupK8sTestClientFail
	client, err := oriongo.Setup()
	require.Error(t, err)
	require.Nil(t, client)

	oriongo.K8sClientFunc = setupK8sTestClient

	client, err = oriongo.Setup()
	require.Error(t, err)
	require.Nil(t, client)

	t.Setenv("NAMESPACE", "default")
	client, err = oriongo.Setup()
	require.Error(t, err)
	require.Nil(t, client)
}
func TestNewClientErrToken(t *testing.T) {
	oriongo.K8sClientFunc = setupK8sTestClient
	t.Setenv("NAMESPACE", "default")

	file, err := os.CreateTemp("./", "test_token")
	require.NoError(t, err)
	_, err = file.WriteString("I'm not a token!")
	require.NoError(t, err)
	oriongo.TokenPath = file.Name()

	defer func() {
		file.Close()
		require.NoError(t, os.Remove(file.Name()))
	}()

	client, err := oriongo.Setup()
	require.Error(t, err)
	require.Nil(t, client)
}

func TestNewClientTokenSubject(t *testing.T) {
	oriongo.K8sClientFunc = setupK8sTestClient
	t.Setenv("NAMESPACE", "default")

	privateKey, _ := generateTestRSAKey(t)
	kid := "test-kid-noaud"

	token := createTestJWT(t, privateKey, kid, map[string]interface{}{
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	file, err := os.CreateTemp("./", "test_token")
	require.NoError(t, err)
	_, err = file.WriteString(token)
	require.NoError(t, err)
	oriongo.TokenPath = file.Name()

	defer func() {
		file.Close()
		require.NoError(t, os.Remove(file.Name()))
	}()

	client, err := oriongo.Setup()
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestMakeRequest(t *testing.T) {
	tokenFile, nsFile := setupTest(t)

	defer func() {
		require.NoError(t, os.Remove(tokenFile))
		require.NoError(t, os.Remove(nsFile))
	}()

	client, err := oriongo.Setup()
	require.NoError(t, err)
	require.NotNil(t, client)

	req, err := http.NewRequest("GET", "http://genesis.argocd.kubernetes.local/some/api", nil)
	require.NoError(t, err)

	resp, err := client.Do(req, "default", "genesis")
	require.NoError(t, err)
	require.NotNil(t, resp)

}

func setupTest(t *testing.T) (string, string) {
	t.Helper()

	privateKey, _ := generateTestRSAKey(t)
	kid := "test-kid-noaud"

	token := createTestJWT(t, privateKey, kid, map[string]interface{}{
		"sub":                                    "system:serviceaccount:default:my-service",
		"kubernetes.io/serviceaccount/namespace": "default",
		"kubernetes.io/serviceaccount/service-account.name": "my-service",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	namespaceFile, err := os.CreateTemp("./", "namespace_file")
	require.NoError(t, err)
	_, err = namespaceFile.WriteString("default")
	require.NoError(t, err)

	file, err := os.CreateTemp("./", "test_token")
	require.NoError(t, err)
	_, err = file.WriteString(token)
	require.NoError(t, err)

	oriongo.TokenPath = file.Name()
	oriongo.NamespacePath = namespaceFile.Name()
	oriongo.K8sClientFunc = setupK8sTestClient

	defer func() {
		file.Close()
		namespaceFile.Close()
	}()
	return namespaceFile.Name(), file.Name()
}

func setupK8sTestClient() (kubernetes.Interface, error) {
	return testclient.NewClientset(), nil
}
func setupK8sTestClientFail() (kubernetes.Interface, error) {
	return nil, errors.New("test error")
}

func createTestJWT(t *testing.T, privateKey *rsa.PrivateKey, kid string, claims map[string]interface{}) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	token.Header["kid"] = kid

	signed, err := token.SignedString(privateKey)
	require.NoError(t, err, "failed to sign test token")

	return signed
}

func generateTestRSAKey(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate test RSA key")

	return privateKey, &privateKey.PublicKey
}
