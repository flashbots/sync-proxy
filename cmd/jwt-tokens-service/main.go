package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt"
)

var (
	listenAddr = flag.String("addr", "localhost:1337", "listen address")
	configFile = flag.String("config", "config.json", "path to the config file")
	clientID   = flag.String("client-id", "", "CL client id, optional")
)

func main() {
	flag.Parse()

	f, err := os.Open(*configFile)
	if err != nil {
		log.Fatalf("failed to open config file: %v", err)
	}
	defer f.Close()

	// host name => hex jwt secret
	var secrets map[string]string
	if err := json.NewDecoder(f).Decode(&secrets); err != nil {
		log.Fatalf("failed to read config file: %v", err)
	}

	http.HandleFunc("/tokens/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("requested tokens from %s", r.RemoteAddr)

		for host, secret := range secrets {
			token, err := generateJWT(secret)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to generate token for %s: %v", host, err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Authorization-"+host, "Bearer "+token)
		}

		w.WriteHeader(http.StatusOK)
	})

	log.Printf("Starting server on %s", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func generateJWT(secretHex string) (string, error) {
	secret, err := hex.DecodeString(secretHex)
	if err != nil {
		return "", fmt.Errorf("invalid hex secret: %v", err)
	}

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)

	claims["iat"] = jwt.TimeFunc().Unix()
	if *clientID != "" {
		claims["id"] = *clientID
	}

	return token.SignedString(secret)
}
