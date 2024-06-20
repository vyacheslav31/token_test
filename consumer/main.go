package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
)

type TokenResponse struct {
	Token string `json:"token"`
}

type SalesforceRequest struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}

type SalesforceResponse struct {
	ResponseMessage string `json:"message"`
}

func main() {
	http.HandleFunc("/restapi/do", MakeRequest)
	log.Printf("New Server started on: http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

var (
	tokenMutex sync.RWMutex
	authToken  string
)

func MakeRequest(w http.ResponseWriter, r *http.Request) {
	tokenMutex.RLock()
	localToken := authToken
	tokenMutex.RUnlock()

	salesforceRequest := SalesforceRequest{
		Token:   localToken,
		Message: "Your message here",
	}

	jsonReq, err := json.Marshal(salesforceRequest)
	if err != nil {
		log.Fatalf("Failed to marshal SalesforceRequest: %v", err)
	}

	resp, err := http.Post("http://localhost:8081/salesforce/do", "application/json", bytes.NewBuffer(jsonReq))
	if err != nil {
		log.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		tokenMutex.Lock()
		if authToken == localToken { // Double-checking
			authToken = GetAuthToken()
		}
		tokenMutex.Unlock()

		// Retry with new token
		MakeRequest(w, r)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	log.Printf("Response: %s", body)
}

func GetAuthToken() string {
	resp, err := http.Get("http://localhost:8081/token/new")
	if err != nil {
		log.Fatalf("Failed to get auth token: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
	}

	var tokenResponse TokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		log.Fatalf("Failed to unmarshal token response: %v", err)
	}

	return tokenResponse.Token
}
