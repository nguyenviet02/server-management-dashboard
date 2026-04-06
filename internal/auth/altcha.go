package auth

import (
	"encoding/base64"
	"encoding/json"
	"time"

	altcha "github.com/altcha-org/altcha-lib-go"
)

// GenerateAltchaChallenge creates a new ALTCHA PoW challenge
func GenerateAltchaChallenge(hmacKey string) (altcha.Challenge, error) {
	expires := time.Now().Add(120 * time.Second)
	return altcha.CreateChallenge(altcha.ChallengeOptions{
		HMACKey:   hmacKey,
		MaxNumber: 50000,
		Expires:   &expires,
	})
}

// VerifyAltchaSolution verifies a base64-encoded ALTCHA payload
func VerifyAltchaSolution(payload string, hmacKey string) (bool, error) {
	// Decode base64 payload from widget
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return false, err
	}

	// Parse JSON into map
	var data altcha.Payload
	if err := json.Unmarshal(decoded, &data); err != nil {
		return false, err
	}

	return altcha.VerifySolution(data, hmacKey, true)
}
