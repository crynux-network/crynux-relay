package v2

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wI2L/fizz"
)

func TestInitRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	fizzEngine := fizz.NewFromEngine(engine)

	InitRoutes(fizzEngine)

	if errs := fizzEngine.Errors(); len(errs) > 0 {
		t.Fatalf("expected no route initialization errors, got %v", errs)
	}
}
