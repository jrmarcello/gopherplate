// Semgrep test fixture for handlers.yml — consumed by `semgrep --test .semgrep/`.
// Marker comments:
//   ruleid: <rule-id>   → the next line MUST match the rule
//   ok:     <rule-id>   → the next line MUST NOT match the rule
//
// Do not import; this file is text-only input for semgrep.

//go:build semgrep_fixture

package semgrep_fixture_handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	// ruleid: gopherplate-handler-no-domain-errors-import
	usererrors "github.com/jrmarcello/gopherplate/internal/domain/user/errors"
)

// Keep the compiler happy by referencing the forbidden import's symbol.
var _ = usererrors.ErrUserNotFound

// These stubs let the ruleid/ok markers anchor to lines that look like
// real handler code. The actual semgrep rule only checks static patterns;
// runtime correctness is irrelevant here.

type httpginStub struct{}

func (httpginStub) SendSuccess(c *gin.Context, status int, data any)           {}
func (httpginStub) SendError(c *gin.Context, status int, msg string)           {}
func (httpginStub) SendSuccessWithMeta(c *gin.Context, s int, d any, m, l any) {}

var httpgin httpginStub

func handlerOK(c *gin.Context) {
	// ok: gopherplate-no-direct-gin-json
	httpgin.SendSuccess(c, http.StatusOK, gin.H{"name": "alice"})
	// ok: gopherplate-no-direct-gin-json
	httpgin.SendError(c, http.StatusBadRequest, "invalid")
}

func handlerBad(c *gin.Context) {
	// ruleid: gopherplate-no-direct-gin-json
	c.JSON(http.StatusOK, gin.H{"name": "leaky"})
	// ruleid: gopherplate-no-direct-gin-json
	c.String(http.StatusOK, "raw")
	// ruleid: gopherplate-no-direct-gin-json
	c.IndentedJSON(http.StatusOK, gin.H{"x": 1})
	// ruleid: gopherplate-no-direct-gin-json
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"err": "oops"})
}
