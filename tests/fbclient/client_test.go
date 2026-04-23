package fbclient

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"copycards/internal/fbclient"
)

func TestRetryOn500(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"_id":"board1","name":"Test"}]`))
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "test-key", 1)
	boards, err := client.ListBoards()

	if err != nil {
		t.Fatalf("ListBoards failed: %v", err)
	}
	if len(boards) != 1 || boards[0].Name != "Test" {
		t.Errorf("Unexpected result: %v", boards)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestNoRetryOn404(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "test-key", 1)
	_, err := client.ListBoards()

	if err == nil {
		t.Fatalf("Expected error for 404")
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt for 404, got %d", attempts)
	}
}

func TestRetryOn429(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"_id":"board1","name":"Test"}]`))
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "test-key", 1)
	boards, err := client.ListBoards()

	if err != nil {
		t.Fatalf("ListBoards failed: %v", err)
	}
	if len(boards) != 1 || boards[0].Name != "Test" {
		t.Errorf("Unexpected result: %v", boards)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestConcurrencyControl(t *testing.T) {
	concurrent := 0
	maxConcurrent := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		concurrent++
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"_id":"board1","name":"Test"}]`))
		concurrent--
	}))
	defer server.Close()

	// Create client with concurrency limit of 2
	client := fbclient.NewClient(server.URL, "test-key", 2)

	// Make 3 requests in parallel - should not exceed concurrency limit
	done := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			_, err := client.ListBoards()
			done <- err
		}()
	}

	// Wait for all requests
	for i := 0; i < 3; i++ {
		if err := <-done; err != nil {
			t.Fatalf("Request failed: %v", err)
		}
	}

	if maxConcurrent > 2 {
		t.Errorf("Expected max 2 concurrent requests, got %d", maxConcurrent)
	}
}

func TestGetBoardSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"_id":"board1","name":"My Board","bins":["bin1"]}`))
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "test-key", 1)
	board, err := client.GetBoard("board1")

	if err != nil {
		t.Fatalf("GetBoard failed: %v", err)
	}
	if board.ID != "board1" || board.Name != "My Board" {
		t.Errorf("Unexpected board: %+v", board)
	}
	if len(board.Bins) != 1 || board.Bins[0] != "bin1" {
		t.Errorf("Unexpected bins: %+v", board.Bins)
	}
}

func TestCreateTicketSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "test-key", 1)
	ticket := &fbclient.Ticket{
		ID:           "ticket1",
		Name:         "Test Ticket",
		BinID:        "bin1",
		TicketTypeID: "type1",
	}

	err := client.CreateTicket(ticket)
	if err != nil {
		t.Fatalf("CreateTicket failed: %v", err)
	}
}

func TestUpdateTicketSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "test-key", 1)
	updates := map[string]interface{}{
		"name": "Updated Name",
	}

	err := client.UpdateTicket("ticket1", updates)
	if err != nil {
		t.Fatalf("UpdateTicket failed: %v", err)
	}
}

func TestBearerTokenAuth(t *testing.T) {
	authHeaderValue := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaderValue = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"_id":"board1","name":"Test"}]`))
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "my-secret-key", 1)
	_, err := client.ListBoards()

	if err != nil {
		t.Fatalf("ListBoards failed: %v", err)
	}
	if authHeaderValue != "bearer my-secret-key" {
		t.Errorf("Expected auth header 'bearer my-secret-key', got '%s'", authHeaderValue)
	}
}
