package oriongo

import (
	"context"
	"fmt"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) getToken(namespace, service string) (string, error) {
	serviceKey := fmt.Sprintf("%s::Service::%s", namespace, service)

	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	token := ""
	expiry := time.Time{}

	if item, ok := c.cache[serviceKey]; ok {
		expiry = item.exp
		token = item.token

		if time.Now().Before(expiry.Add(-time.Minute)) {
			return token, nil
		}
	}

	// Token is not in cache or has expired, need to make a new one
	tokenStatus, err := c.createToken(serviceKey)
	if err != nil {
		return "", err
	}

	c.cache[serviceKey] = cacheItem{
		exp:   tokenStatus.ExpirationTimestamp.Time,
		token: tokenStatus.Token,
	}

	return tokenStatus.Token, nil
}

func (c *Client) createToken(audience string) (*authenticationv1.TokenRequestStatus, error) {
	var expirationSeconds int64 = 600 // 10 minutes

	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         []string{audience},
			ExpirationSeconds: &expirationSeconds,
		},
	}

	response, err := c.k8sClient.CoreV1().
		ServiceAccounts(c.namespace).
		CreateToken(context.Background(), c.serviceAccountName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return &response.Status, nil
}
