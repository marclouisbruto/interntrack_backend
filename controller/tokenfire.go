package controller

import (
	"encoding/json"
	"fmt"
	"intern_template_v1/model"
	"net/http"
)

func SaveFCMToken(w http.ResponseWriter, r *http.Request) {
	var req model.TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Store `req.Token` in your database linked to `req.InternID`
	fmt.Printf("Received token for intern %s: %s\n", req.InternID, req.Token)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Token saved successfully"))
}
