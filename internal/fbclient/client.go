package fbclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	endpoint    string
	apiKey      string
	httpClient  *http.Client
	concurrency int
	semaphore   chan struct{}
}

// NewClient creates a new Flowboards API client
func NewClient(endpoint, apiKey string, concurrency int) *Client {
	return &Client{
		endpoint:    endpoint,
		apiKey:      apiKey,
		concurrency: concurrency,
		semaphore:   make(chan struct{}, concurrency),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// GetBoard fetches board details
func (c *Client) GetBoard(boardID string) (*Board, error) {
	var result Board
	if err := c.get(fmt.Sprintf("/boards/%s", boardID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListBoards fetches all boards
func (c *Client) ListBoards() ([]Board, error) {
	var results []Board
	if err := c.get("/boards", &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetTicket fetches a single ticket
func (c *Client) GetTicket(ticketID string) (*Ticket, error) {
	var result Ticket
	if err := c.get(fmt.Sprintf("/tickets/%s", ticketID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTicketsByBin fetches tickets in a bin
func (c *Client) ListTicketsByBin(binID string) ([]Ticket, error) {
	var results []Ticket
	if err := c.getWithParams(fmt.Sprintf("/tickets?bin_id=%s", binID), &results); err != nil {
		return nil, err
	}
	return results, nil
}

// ListTicketsByParent fetches child tickets
func (c *Client) ListTicketsByParent(parentID string) ([]Ticket, error) {
	var results []Ticket
	if err := c.getWithParams(fmt.Sprintf("/tickets?parent_id=%s", parentID), &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetTicketType fetches a ticket type by ID
func (c *Client) GetTicketType(typeID string) (*TicketType, error) {
	var result TicketType
	if err := c.get(fmt.Sprintf("/ticket-types/%s", typeID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTicketTypes fetches all ticket types
func (c *Client) ListTicketTypes() ([]TicketType, error) {
	var results []TicketType
	if err := c.get("/ticket-types", &results); err != nil {
		return nil, err
	}
	return results, nil
}

// ListCustomFields fetches all custom fields
func (c *Client) ListCustomFields() ([]CustomField, error) {
	var results []CustomField
	if err := c.get("/custom-fields", &results); err != nil {
		return nil, err
	}
	return results, nil
}

// ListUsers fetches all users
func (c *Client) ListUsers() ([]User, error) {
	var results []User
	if err := c.get("/users", &results); err != nil {
		return nil, err
	}
	return results, nil
}

// CreateTicket creates a new ticket
func (c *Client) CreateTicket(ticket *Ticket) error {
	newID := ticket.ID
	body, _ := json.Marshal(ticket)
	return c.post(fmt.Sprintf("/tickets/%s", newID), bytes.NewReader(body), nil)
}

// UpdateTicket updates a ticket using $partial syntax
func (c *Client) UpdateTicket(ticketID string, updates map[string]interface{}) error {
	payload := map[string]interface{}{
		"$partial": updates,
	}
	body, _ := json.Marshal(payload)
	return c.put(fmt.Sprintf("/tickets/%s", ticketID), bytes.NewReader(body), nil)
}

// AddTicketParent links a ticket to a parent
func (c *Client) AddTicketParent(ticketIDs []string, parentID string) error {
	idStr := ""
	for i, id := range ticketIDs {
		if i > 0 {
			idStr += ","
		}
		idStr += id
	}
	return c.put(fmt.Sprintf("/tickets/addParent?ids=%s&parent_id=%s", idStr, parentID), nil, nil)
}

// CreateComment creates a comment on a ticket
func (c *Client) CreateComment(commentID string, ticketID string, body string) error {
	payload := map[string]interface{}{
		"comment":   body,
		"ticket_id": ticketID,
	}
	data, _ := json.Marshal(payload)
	return c.post(fmt.Sprintf("/ticket-comments/%s", commentID), bytes.NewReader(data), nil)
}

// ListComments fetches comments for a ticket
func (c *Client) ListComments(ticketID string) ([]Comment, error) {
	var results []Comment
	if err := c.getWithParams(fmt.Sprintf("/ticket-comments?ticket_id=%s", ticketID), &results); err != nil {
		return nil, err
	}
	return results, nil
}

// UploadAttachment uploads a file to a ticket
func (c *Client) UploadAttachment(ticketID, filename string, data []byte) (*Attachment, error) {
	var result Attachment
	reader := bytes.NewReader(data)
	url := fmt.Sprintf("%s/tickets/%s/attachments?name=%s", c.endpoint, ticketID, filename)

	req, _ := http.NewRequest("POST", url, reader)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	c.acquire()
	defer c.release()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload attachment: HTTP %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAttachment downloads an attachment
func (c *Client) GetAttachment(ticketID, attachmentID string) ([]byte, error) {
	url := fmt.Sprintf("%s/tickets/%s/attachments/%s", c.endpoint, ticketID, attachmentID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))

	c.acquire()
	defer c.release()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download attachment: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// GetIDs allocates new IDs from the /ids endpoint
func (c *Client) GetIDs(count int) ([]string, error) {
	if count < 1 || count > 100 {
		count = 4
	}
	url := fmt.Sprintf("%s/ids?max-results=%d", c.endpoint, count)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ids []string
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("decode IDs: %w", err)
	}

	return ids, nil
}

// --- Private methods ---

func (c *Client) get(path string, out interface{}) error {
	url := c.endpoint + path
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) getWithParams(path string, out interface{}) error {
	// For now, same as get; pagination added later if needed
	return c.get(path, out)
}

func (c *Client) post(path string, body io.Reader, out interface{}) error {
	url := c.endpoint + path
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, string(bodyBytes))
	}

	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) put(path string, body io.Reader, out interface{}) error {
	url := c.endpoint + path
	req, _ := http.NewRequest("PUT", url, body)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT %s: HTTP %d: %s", path, resp.StatusCode, string(bodyBytes))
	}

	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	maxRetries := 6
	baseDelay := 500 * time.Millisecond
	maxDelay := 30 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request: %w", err)
		}

		// Success
		if resp.StatusCode < 500 && resp.StatusCode != 429 {
			return resp, nil
		}

		// Retryable error: 5xx or 429
		resp.Body.Close()

		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("max retries exceeded, last status: %d", resp.StatusCode)
		}

		// Exponential backoff with jitter
		delay := baseDelay * time.Duration(1<<uint(attempt))
		if delay > maxDelay {
			delay = maxDelay
		}
		jitter := time.Duration(rand.Intn(int(delay / 10)))
		time.Sleep(delay + jitter)
	}

	return nil, fmt.Errorf("unexpected retry loop exit")
}

func (c *Client) acquire() {
	c.semaphore <- struct{}{}
}

func (c *Client) release() {
	<-c.semaphore
}
