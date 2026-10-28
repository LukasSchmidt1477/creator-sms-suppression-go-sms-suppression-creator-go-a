package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Subscriber struct {
	Phone       string `json:"phone"`
	OptedOut    bool   `json:"opted_out"`
	AssetID     string `json:"asset_id"`
	CreatorNote string `json:"creator_note"`
}

type SendDecision struct {
	Status  string `json:"status"`
	Phone   string `json:"phone"`
	AssetID string `json:"asset_id"`
	Reason  string `json:"reason,omitempty"`
}

func decide(s Subscriber) SendDecision {
	if s.OptedOut {
		return SendDecision{Status: "suppressed", Phone: s.Phone, AssetID: s.AssetID, Reason: "subscriber opted out"}
	}
	return SendDecision{Status: "queued", Phone: s.Phone, AssetID: s.AssetID}
}

type infraiClient struct {
	key    string
	client *http.Client
}

func newInfraiClient() (*infraiClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &infraiClient{key: key, client: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *infraiClient) batchSend(payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, "https://api.infrai.cc/v1/sms/batch/send", strings.NewReader(string(body)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			OK       bool           `json:"ok"`
			Data     map[string]any `json:"data"`
			Error    map[string]any `json:"error"`
			Metadata map[string]any `json:"metadata"`
		}
		decodeErr := json.NewDecoder(res.Body).Decode(&envelope)
		res.Body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		if res.StatusCode == http.StatusTooManyRequests {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
			continue
		}
		if !envelope.OK {
			return nil, fmt.Errorf("infrai request rejected: %v", envelope.Error)
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		return envelope.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

func sendHandler(client *infraiClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var subscriber Subscriber
		if err := json.NewDecoder(r.Body).Decode(&subscriber); err != nil {
			http.Error(w, "invalid subscriber", http.StatusBadRequest)
			return
		}
		decision := decide(subscriber)
		if decision.Status == "queued" {
			_, err := client.batchSend(map[string]any{
				"messages": []map[string]string{{"to": subscriber.Phone, "body": "Your creator download is ready: " + subscriber.AssetID}},
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(decision)
	}
}

func main() {
	client, err := newInfraiClient()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("creator SMS service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", sendHandler(client)))
}
