package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// LineClient wraps the two LINE platform calls this service needs:
// verifying a LIFF ID token (to learn who is submitting) and pushing a
// message back into that user's chat. It never touches the Messaging
// API's webhook/reply flow — this app only ever pushes, it never
// receives inbound events.
type LineClient struct {
	channelID   string
	accessToken string
	httpClient  *http.Client
}

func NewLineClient(channelID, accessToken string) *LineClient {
	return &LineClient{
		channelID:   channelID,
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

type verifyResponse struct {
	Sub              string `json:"sub"`
	Aud              string `json:"aud"`
	Exp              int64  `json:"exp"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// VerifyIDToken calls LINE's token verification endpoint
// (https://developers.line.biz/en/reference/line-login/#verify-id-token)
// and returns the LINE user ID (the "sub" claim) if the token is valid,
// unexpired, and issued for this channel. This is the only "auth" this
// app does: it proves the request came from inside a live LINE session,
// but does not gate who may open the LIFF app itself.
func (c *LineClient) VerifyIDToken(idToken string) (string, error) {
	form := url.Values{}
	form.Set("id_token", idToken)
	form.Set("client_id", c.channelID)

	req, err := http.NewRequest(http.MethodPost, "https://api.line.me/oauth2/v2.1/verify", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling verify endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var vr verifyResponse
	if err := json.Unmarshal(body, &vr); err != nil {
		return "", fmt.Errorf("decoding verify response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("id token rejected: %s (%s)", vr.Error, vr.ErrorDescription)
	}
	if vr.Aud != c.channelID {
		return "", fmt.Errorf("id token audience %q does not match configured channel %q", vr.Aud, c.channelID)
	}
	if vr.Exp < time.Now().Unix() {
		return "", fmt.Errorf("id token expired")
	}
	if vr.Sub == "" {
		return "", fmt.Errorf("id token missing subject")
	}
	return vr.Sub, nil
}

// --- Minimal Messaging API push-message types ---

type message map[string]any

func imageMessage(originalURL, previewURL string) message {
	return message{
		"type":               "image",
		"originalContentUrl": originalURL,
		"previewImageUrl":    previewURL,
	}
}

// flexAck builds a Flex Message "bubble" acknowledging a submitted
// complaint — this is the rich-text acknowledgement referenced in the
// brief. Every element here (box/text/separator) is a real Flex
// component; nothing in it needs a Mini App or custom web view.
func flexAck(ticketID, title, description string, photoCount int) message {
	if description == "" {
		description = "(no description provided)"
	}

	body := map[string]any{
		"type":   "box",
		"layout": "vertical",
		"contents": []map[string]any{
			{"type": "text", "text": "Complaint received", "weight": "bold", "size": "sm", "color": "#06C755"},
			{"type": "text", "text": title, "weight": "bold", "size": "lg", "wrap": true, "margin": "sm"},
			{"type": "text", "text": description, "size": "sm", "color": "#666666", "wrap": true, "margin": "sm"},
			{"type": "separator", "margin": "md"},
			{
				"type": "box", "layout": "baseline", "margin": "md",
				"contents": []map[string]any{
					{"type": "text", "text": "Ticket", "size": "xs", "color": "#999999", "flex": 2},
					{"type": "text", "text": ticketID, "size": "xs", "color": "#333333", "flex": 5},
				},
			},
			{
				"type": "box", "layout": "baseline", "margin": "sm",
				"contents": []map[string]any{
					{"type": "text", "text": "Photos", "size": "xs", "color": "#999999", "flex": 2},
					{"type": "text", "text": fmt.Sprintf("%d attached", photoCount), "size": "xs", "color": "#333333", "flex": 5},
				},
			},
		},
	}

	return message{
		"type":     "flex",
		"altText":  fmt.Sprintf("Complaint received: %s", title),
		"contents": map[string]any{"type": "bubble", "body": body},
	}
}

type pushRequest struct {
	To       string    `json:"to"`
	Messages []message `json:"messages"`
}

// Push sends messages to a single user, splitting into batches of 5
// (the Messaging API's per-call limit for the messages array).
func (c *LineClient) Push(userID string, messages []message) error {
	const batchSize = 5
	for i := 0; i < len(messages); i += batchSize {
		end := min(i+batchSize, len(messages))
		if err := c.pushBatch(userID, messages[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func (c *LineClient) pushBatch(userID string, messages []message) error {
	payload := pushRequest{To: userID, Messages: messages}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.line.me/v2/bot/message/push", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling push endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("push failed: %s: %s", resp.Status, string(respBody))
	}
	return nil
}
