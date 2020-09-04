package apiclient

import (
	"fmt"
	"net/http"
)

type Authenticator interface {
	AuthenticateRequest(req *http.Request) error
}

type QueryArgAuth struct {
	arg string
	key string
}

func NewQueryArgAuth(arg, key string) *QueryArgAuth {
	return &QueryArgAuth{arg: arg, key: key}
}

func (a *QueryArgAuth) AuthenticateRequest(req *http.Request) error {
	u := req.URL
	v := u.Query()
	v.Set(a.arg, a.key)
	u.RawQuery = v.Encode()
	return nil
}

type BearerAuth string

func NewBearerAuth(token string) *BearerAuth {
	a := BearerAuth(token)
	return &a
}

func (a *BearerAuth) AuthenticateRequest(req *http.Request) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(*a)))
	return nil
}

type BasicAuth struct {
	user string
	pwd string
}

func NewBasicAuth(user, pwd string) *BasicAuth {
	return &BasicAuth{user: user, pwd: pwd}
}

func (a *BasicAuth) AuthenticateRequest(req *http.Request) error {
	req.SetBasicAuth(a.user, a.pwd)
	return nil
}
