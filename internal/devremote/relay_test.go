package devremote

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRelayPreservesRequestsAndAuthentication(t *testing.T) {
	replies, replyWriter := io.Pipe()
	requests, requestWriter := io.Pipe()
	defer replies.Close()
	defer replyWriter.Close()
	defer requests.Close()
	defer requestWriter.Close()
	finished := make(chan error, 1)
	go func() { finished <- serveRelay(context.Background(), replies, requestWriter) }()
	decoder := json.NewDecoder(requests)
	var ready relayMessage
	if err := decoder.Decode(&ready); err != nil {
		t.Fatal(err)
	}
	writer := &relayWriter{encoder: json.NewEncoder(replyWriter)}
	go func() {
		for {
			var m relayMessage
			if decoder.Decode(&m) != nil {
				return
			}
			status := http.StatusOK
			if m.Authorization != "Bearer session-key" {
				status = http.StatusForbidden
			}
			writer.send(relayMessage{ID: m.ID, Status: status})
			writer.send(relayMessage{ID: m.ID, Data: m.Data})
			writer.send(relayMessage{ID: m.ID, Done: true})
		}
	}()
	for _, key := range []string{"Bearer session-key", "wrong"} {
		key := key
		t.Run(key, func(t *testing.T) {
			// Requests stay on a loopback endpoint and carry the capability unchanged.
			req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/invoke", ready.Port), strings.NewReader("request body"))
			req.Header.Set("Authorization", key)
			client := &http.Client{Transport: &http.Transport{Proxy: nil}}
			defer client.CloseIdleConnections()
			response, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(response.Body)
			response.Body.Close()
			want := http.StatusOK
			if key == "wrong" {
				want = http.StatusForbidden
			}
			if response.StatusCode != want || string(data) != "request body" {
				t.Fatalf("response %d %q", response.StatusCode, data)
			}
		})
	}
	replyWriter.Close()
	<-finished
}

func TestCanceledRelayRequestCannotStallOtherReplies(t *testing.T) {
	replies := make(chan relayMessage, 1)
	replies <- relayMessage{ID: 1}
	canceled := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		if err := deliverRelayReply(context.Background(), relayPending{replies: replies, done: canceled}, relayMessage{ID: 1}); err != nil {
			finished <- err
			return
		}
		next := make(chan relayMessage, 1)
		err := deliverRelayReply(context.Background(), relayPending{replies: next}, relayMessage{ID: 2})
		if err == nil && (<-next).ID != 2 {
			t.Error("lost another request's reply")
		}
		finished <- err
	}()
	close(canceled)
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("abandoned request blocked the shared relay")
	}
}
