package oriongo_test

import (
	"context"
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

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestNewClient(t *testing.T) {
	origFn := oriongo.K8sClientFunc
	origToken := oriongo.TokenPath
	origNs := oriongo.NamespacePath

	t.Cleanup(func() {
		oriongo.K8sClientFunc = origFn
		oriongo.TokenPath = origToken
		oriongo.NamespacePath = origNs
	})
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
	origFn := oriongo.K8sClientFunc
	origToken := oriongo.TokenPath
	origNs := oriongo.NamespacePath

	t.Cleanup(func() {
		oriongo.K8sClientFunc = origFn
		oriongo.TokenPath = origToken
		oriongo.NamespacePath = origNs
	})

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
	origFn := oriongo.K8sClientFunc
	origToken := oriongo.TokenPath
	origNs := oriongo.NamespacePath

	t.Cleanup(func() {
		oriongo.K8sClientFunc = origFn
		oriongo.TokenPath = origToken
		oriongo.NamespacePath = origNs
	})

	oriongo.K8sClientFunc = setupK8sTestClient

	t.Setenv("NAMESPACE", "default")

	file, err := os.CreateTemp("./", "test_token")
	require.NoError(t, err)
	_, err = file.WriteString("I'm not a token!")
	require.NoError(t, err)

	oriongo.TokenPath = file.Name()

	defer func() {
		require.NoError(t, file.Close())
		require.NoError(t, os.Remove(file.Name()))
	}()

	client, err := oriongo.Setup()
	require.Error(t, err)
	require.Nil(t, client)
}

func TestNewClientTokenSubject(t *testing.T) {
	origFn := oriongo.K8sClientFunc
	origToken := oriongo.TokenPath
	origNs := oriongo.NamespacePath

	t.Cleanup(func() {
		oriongo.K8sClientFunc = origFn
		oriongo.TokenPath = origToken
		oriongo.NamespacePath = origNs
	})

	oriongo.K8sClientFunc = setupK8sTestClient

	t.Setenv("NAMESPACE", "default")

	privateKey := generateTestRSAKey(t)
	kid := "test-kid-noaud"

	token := createTestJWT(t, privateKey, kid, map[string]any{
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	file, err := os.CreateTemp("./", "test_token")
	require.NoError(t, err)
	_, err = file.WriteString(token)
	require.NoError(t, err)

	oriongo.TokenPath = file.Name()

	defer func() {
		require.NoError(t, file.Close())
		require.NoError(t, os.Remove(file.Name()))
	}()

	client, err := oriongo.Setup()
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestMakeRequest(t *testing.T) {
	origFn := oriongo.K8sClientFunc
	origToken := oriongo.TokenPath
	origNs := oriongo.NamespacePath

	t.Cleanup(func() {
		oriongo.K8sClientFunc = origFn
		oriongo.TokenPath = origToken
		oriongo.NamespacePath = origNs
	})

	oriongo.HTTPClient = &http.Client{Transport: &mockTransport{}}

	t.Cleanup(func() { oriongo.HTTPClient = &http.Client{} })
	tokenFile, nsFile := setupTest(t)

	defer func() {
		require.NoError(t, os.Remove(tokenFile))
		require.NoError(t, os.Remove(nsFile))
	}()

	client, err := oriongo.Setup()
	require.NoError(t, err)
	require.NotNil(t, client)

	req, err := http.NewRequest(http.MethodGet, "http://genesis.argocd.kubernetes.local/some/api:1234", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "fake-jwt-token", req.Header.Get("X-ORION-SERVICE-AUTH"))

	// Call it again to hit the cache
	req, err = http.NewRequest(http.MethodGet, "http://genesis.argocd.kubernetes.local/some/api:1234", nil)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "fake-jwt-token", req.Header.Get("X-ORION-SERVICE-AUTH"))
}
func TestBadURL(t *testing.T) {
	origFn := oriongo.K8sClientFunc
	origToken := oriongo.TokenPath
	origNs := oriongo.NamespacePath

	t.Cleanup(func() {
		oriongo.K8sClientFunc = origFn
		oriongo.TokenPath = origToken
		oriongo.NamespacePath = origNs
	})

	oriongo.HTTPClient = &http.Client{Transport: &mockTransport{}}

	t.Cleanup(func() { oriongo.HTTPClient = &http.Client{} })
	tokenFile, nsFile := setupTest(t)

	defer func() {
		require.NoError(t, os.Remove(tokenFile))
		require.NoError(t, os.Remove(nsFile))
	}()

	client, err := oriongo.Setup()
	require.NoError(t, err)
	require.NotNil(t, client)

	req, err := http.NewRequest(http.MethodGet, "http://genesis.argocd", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.Error(t, err)

	defer func() {
		require.NoError(t, resp.Body.Close())
	}()
}

func setupTest(t *testing.T) (string, string) {
	t.Helper()

	origFn := oriongo.K8sClientFunc
	origToken := oriongo.TokenPath
	origNs := oriongo.NamespacePath

	t.Cleanup(func() {
		oriongo.K8sClientFunc = origFn
		oriongo.TokenPath = origToken
		oriongo.NamespacePath = origNs
	})
	privateKey := generateTestRSAKey(t)
	kid := "test-kid-noaud"

	token := createTestJWT(t, privateKey, kid, map[string]any{
		"sub":                                    "system:serviceaccount:default:rhea",
		"kubernetes.io/serviceaccount/namespace": "default",
		"kubernetes.io/serviceaccount/service-account.name": "rhea",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	file, err := os.CreateTemp("./", "test_token")
	require.NoError(t, err)
	_, err = file.WriteString(token)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	namespaceFile, err := os.CreateTemp("./", "namespace_file")
	require.NoError(t, err)
	_, err = namespaceFile.WriteString("default")
	require.NoError(t, err)
	require.NoError(t, namespaceFile.Close())

	oriongo.TokenPath = file.Name()
	oriongo.NamespacePath = namespaceFile.Name()
	oriongo.K8sClientFunc = setupK8sTestClient

	return file.Name(), namespaceFile.Name()
}

func setupK8sTestClient() (kubernetes.Interface, error) {
	fakeK8s := testclient.NewSimpleClientset()

	fakeK8s.PrependReactor("create", "serviceaccounts/token",
		func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, &authenticationv1.TokenRequest{
				Status: authenticationv1.TokenRequestStatus{ //nolint:gosec
					Token:               "fake-jwt-token",
					ExpirationTimestamp: metav1.NewTime(time.Now().Add(10 * time.Minute)),
				},
			}, nil
		},
	)

	for _, serviceAccount := range []struct{ name, ns string }{
		{"genesis", "argocd"},
		{"rhea", "default"},
	} {
		_, err := fakeK8s.CoreV1().ServiceAccounts(serviceAccount.ns).Create(
			context.Background(),
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: serviceAccount.name, Namespace: serviceAccount.ns},
			},
			metav1.CreateOptions{},
		)
		if err != nil {
			return nil, err
		}
	}

	return fakeK8s, nil
}
func setupK8sTestClientFail() (kubernetes.Interface, error) {
	return nil, errors.New("test error")
}

func createTestJWT(t *testing.T, privateKey *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	token.Header["kid"] = kid

	signed, err := token.SignedString(privateKey)
	require.NoError(t, err, "failed to sign test token")

	return signed
}

func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate test RSA key")

	return privateKey
}

// mockTransport intercepts HTTP calls so tests don't hit the network
type mockTransport struct{}

func (m *mockTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}
