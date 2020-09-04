package apiclient

import (
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }
type APIClientSuite struct {}
var _ = Suite(&APIClientSuite{})

func (a *APIClientSuite) TestX(c *C) {
	c.Check(true, Equals, true)
}
