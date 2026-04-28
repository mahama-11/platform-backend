package productscope

import (
	"strings"

	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

type Scope struct {
	ProductCode string
	IncludeAll  bool
}

func Resolve(c *gin.Context) (Scope, bool) {
	productCode := strings.TrimSpace(c.Query("product_code"))
	includeAll := strings.EqualFold(strings.TrimSpace(c.Query("include_all_products")), "true")

	if productCode != "" && includeAll {
		response.JSONError(c, response.CodeInvalidParameter, "product_code and include_all_products cannot be used together")
		return Scope{}, false
	}
	if productCode == "" && !includeAll {
		response.JSONError(c, response.CodeMissingParameter, "product_code is required unless include_all_products=true")
		return Scope{}, false
	}

	return Scope{
		ProductCode: productCode,
		IncludeAll:  includeAll,
	}, true
}
