package middleware

import (
	"crynux_relay/api/tools"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
)

type authTestSigningInput struct {
	Address string `json:"address"`
}

func TestJWTOrSignatureAuthMiddlewareAllowsJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initialJWTManager := tools.DefaultJWTManager
	tools.DefaultJWTManager = tools.NewJWTManager("test-secret", time.Hour)
	t.Cleanup(func() {
		tools.DefaultJWTManager = initialJWTManager
	})

	address := "0x123"
	token, _, err := tools.GenerateToken(address)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	c, recorder := newAuthTestContext("/v2/resource/0x123")
	c.Request.Header.Set("Authorization", "Bearer "+token)

	JWTOrSignatureAuthMiddleware(authTestInputBuilder)(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := GetUserAddress(c); got != address {
		t.Fatalf("expected authorized address %s, got %s", address, got)
	}
}

func TestJWTOrSignatureAuthMiddlewareAllowsSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	privateKeyHex := "0440cb8b2962699e5ce6835170ba86a085d67477e5581e398674a59feb8e7b9c"
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	timestamp := time.Now().Unix()
	signature := signAuthTestInput(t, privateKeyHex, authTestSigningInput{Address: address}, timestamp)
	c, recorder := newAuthTestContext(
		"/v2/resource/" + address +
			"?timestamp=" + strconv.FormatInt(timestamp, 10) +
			"&signature=" + signature,
	)
	c.Params = gin.Params{{Key: "address", Value: address}}

	JWTOrSignatureAuthMiddleware(authTestInputBuilder)(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := GetUserAddress(c); got != address {
		t.Fatalf("expected authorized address %s, got %s", address, got)
	}
}

func TestJWTOrSignatureAuthMiddlewareRejectsMissingCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, recorder := newAuthTestContext("/v2/resource/0x123")

	JWTOrSignatureAuthMiddleware(authTestInputBuilder)(c)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
	if !c.IsAborted() {
		t.Fatal("expected request to be aborted")
	}
}

func newAuthTestContext(rawURL string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, rawURL, nil)
	return c, recorder
}

func authTestInputBuilder(c *gin.Context) interface{} {
	return authTestSigningInput{Address: c.Param("address")}
}

func signAuthTestInput(t *testing.T, privateKeyHex string, input authTestSigningInput, timestamp int64) string {
	t.Helper()
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	dataBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	signBytes := append(dataBytes, []byte(strconv.FormatInt(timestamp, 10))...)
	signature, err := crypto.Sign(crypto.Keccak256Hash(signBytes).Bytes(), privateKey)
	if err != nil {
		t.Fatalf("failed to sign input: %v", err)
	}
	return hexutil.Encode(signature)
}
