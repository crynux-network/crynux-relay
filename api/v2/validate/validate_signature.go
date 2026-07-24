package validate

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum/crypto"
	log "github.com/sirupsen/logrus"
	"math"
	"strconv"
	"strings"
	"time"
)

func jsonRemarshal(bytes []byte) ([]byte, error) {
	var ifce interface{}
	err := json.Unmarshal(bytes, &ifce)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ifce)
}

func ValidateSignature(data interface{}, timestamp int64, signature string) (bool, string, error) {
	return validateSignatureAt(data, timestamp, signature, time.Now().Unix())
}

func validateSignatureAt(data interface{}, timestamp int64, signature string, current int64) (bool, string, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return false, "", err
	}

	dataBytes, err = jsonRemarshal(dataBytes)
	if err != nil {
		return false, "", err
	}
	log.Debugf("data to verify: %s", string(dataBytes))

	log.Debugln("signature to verify: " + signature)

	if len(signature) != 132 || !strings.HasPrefix(signature, "0x") {
		return false, "", errors.New("invalid signature format")
	}
	signatureBytes, err := hex.DecodeString(signature[2:])
	if err != nil {
		return false, "", err
	}

	if math.Abs(float64(current-timestamp)) > 60 {
		return false, "", nil
	}

	timeStr := strconv.FormatInt(timestamp, 10)

	timeByte := []byte(timeStr)

	signBytes := append(dataBytes, timeByte...)

	log.Debugln("sign string: " + string(signBytes))

	dataHash := crypto.Keccak256Hash(signBytes)

	sigPublicKeyECDSA, err := crypto.SigToPub(dataHash.Bytes(), signatureBytes)

	if err != nil {
		return false, "", err
	}

	address := crypto.PubkeyToAddress(*sigPublicKeyECDSA).Hex()

	log.Debugln("address from signature: " + address)

	signatureNoRecoverID := signatureBytes[:len(signatureBytes)-1] // remove recovery ID

	verified := crypto.VerifySignature(
		crypto.FromECDSAPub(sigPublicKeyECDSA),
		dataHash.Bytes(),
		signatureNoRecoverID)

	return verified, address, nil
}
