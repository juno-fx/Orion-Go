# Orion-Go

Orion-Go is the client we use for Orion services to interact with each other from Go based applications.

It contains the authorization logic needed to interact with [`rhea`](https://juno-fx.github.io/Orion-Documentation/latest/rhea/intro/), our authN and authR backend for service-to-service communication.

## Usage

Simply create the http.Request you wish to perform and pass it to the client it will attach the auth token to the header and make the web request on your behalf returning the bare result to the caller

```go

// Create the client
client, err := oriongo.Setup()
if err != nil{
    // Handle the error
}

// Create the http request as usual
req, _ = http.NewRequest(http.MethodGet, "http://genesis.argocd.kubernetes.local/some/api:1234", nil)

// Pass it to the new client to make the request
resp, err := client.Do(req)

```