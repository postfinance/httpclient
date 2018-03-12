# Example
This example covers the creation of a service for https://jsonplaceholder.typicode.com/ public test API with the following functions:

- Get an entry by ID
- List all entries
- Create a new entry
- Delete an entry by ID

## Step-by-step

### Install httpclient
```
go get -u github.com/postfinance/httpclient/cmd/httpclient-gen-go
```

### Define a service interface

```go
// PostService interface defines service methods
type PostService interface {
	Get(context.Context, int) (*Post, *http.Response, error)
	List(context.Context) ([]Post, *http.Response, error)
	Create(context.Context, *Post) (*Post, *http.Response, error)
	Delete(context.Context, int) (*http.Response, error)
}
```

### Implement the interface
See [jsonplaceholder.go](jsonplaceholder/jsonplaceholder.go)

### Generate the httpclient code
```
httpclient-gen-go -path ./jsonplaceholder -package jsonplaceholder -out ./jsonplaceholder/httpclient.go
```

### Run tests
```
cd jsonplaceholder
go test -v
```

### Usage
See [jsonplaceholder_test.go](jsonplaceholder/jsonplaceholder_test.go)
