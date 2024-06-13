package main

import (
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

var currentToken string

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"

var tokenMutex sync.RWMutex

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
	currentToken = generateRandomString(32)

	http.HandleFunc("/token/new", GetToken)
	http.HandleFunc("/salesforce/do", DoSalesforceStuff)
	log.Println("Starting server on :8081")
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ticker.C:
				tokenMutex.Lock()
				currentToken = generateRandomString(32)
				log.Printf("Updated token: %s\n", currentToken)
				tokenMutex.Unlock()
			}
		}
	}()
	http.ListenAndServe(":8081", nil)
}

func GetToken(w http.ResponseWriter, r *http.Request) {
	tokenMutex.RLock()
	resp := TokenResponse{Token: currentToken}
	tokenMutex.RUnlock()
	respBytes, err := json.Marshal(resp)
	log.Printf("New token: %s\n", currentToken)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error marshalling response"))
		return
	}
	w.Write(respBytes)
}

func DoSalesforceStuff(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error reading request"))
		return
	}

	var req SalesforceRequest
	err = json.Unmarshal(data, &req)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error unmarshalling request"))
		return
	}

	if req.Token != currentToken {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Token expired"))
		return
	}

	resp := SalesforceResponse{
		ResponseMessage: "Hello, " + req.Message,
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error marshalling response"))
		return
	}

	log.Printf("Responding with: %s\n", string(respBytes))

	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

func generateRandomString(length int) string {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}
