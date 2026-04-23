package fbclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrCloudFrontBlocked is returned when a request is rejected by CloudFront's
// WAF after all retries have been exhausted. Callers can match this with
// errors.Is to apply fallbacks (e.g. split the payload across two requests).
var ErrCloudFrontBlocked = errors.New("CloudFront blocked request")

type Client struct {
	endpoint    string
	apiKey      string
	httpClient  *http.Client
	concurrency int
	semaphore   chan struct{}
}

// userAgentTransport injects a copycards User-Agent into every outgoing
// request. The default Go client sends "Go-http-client/1.1", which many
// CloudFront / AWS WAF managed rule sets treat as suspicious.
type userAgentTransport struct {
	base http.RoundTripper
}

const defaultUserAgent = "copycards/1.0"

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUserAgent)
	}
	return t.base.RoundTrip(req)
}

// NewClient creates a new Flowboards API client
func NewClient(endpoint, apiKey string, concurrency int) *Client {
	return &Client{
		endpoint:    endpoint,
		apiKey:      apiKey,
		concurrency: concurrency,
		semaphore:   make(chan struct{}, concurrency),
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &userAgentTransport{base: http.DefaultTransport},
		},
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

// GetBin fetches bin details
func (c *Client) GetBin(binID string) (*Bin, error) {
	var result Bin
	if err := c.get(fmt.Sprintf("/bins/%s", binID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListBins fetches all bins in the organization (with pagination support)
func (c *Client) ListBins() ([]Bin, error) {
	return c.getAllBins()
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
	return c.post(fmt.Sprintf("/tickets/%s", newID), body, nil)
}

// UpdateTicket updates a ticket using $partial syntax
func (c *Client) UpdateTicket(ticketID string, updates map[string]interface{}) error {
	payload := map[string]interface{}{
		"$partial": updates,
	}
	body, _ := json.Marshal(payload)
	return c.put(fmt.Sprintf("/tickets/%s", ticketID), body, nil)
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
	return c.post(fmt.Sprintf("/ticket-comments/%s", commentID), data, nil)
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

// getAllBins fetches all bins across all pages, handling pageToken pagination
func (c *Client) getAllBins() ([]Bin, error) {
	var allBins []Bin
	pageToken := ""

	for {
		url := c.endpoint + "/bins?max-results=500"
		if pageToken != "" {
			url += "&page-token=" + pageToken
		}

		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))

		c.acquire()
		resp, err := c.doWithRetry(req)
		c.release()

		if err != nil {
			return nil, err
		}

		// Decode this page
		var pageBins []Bin
		if err := json.NewDecoder(resp.Body).Decode(&pageBins); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode bins: %w", err)
		}
		resp.Body.Close()

		// Append to results
		allBins = append(allBins, pageBins...)

		// Check for next page. Server sends the token as "page-token".
		pageToken = resp.Header.Get("page-token")
		if pageToken == "" {
			break
		}
	}

	return allBins, nil
}

func (c *Client) getWithParams(path string, out interface{}) error {
	// For now, same as get; pagination added later if needed
	return c.get(path, out)
}

func (c *Client) post(path string, body []byte, out interface{}) error {
	url := c.endpoint + path
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, _ := http.NewRequest("POST", url, reqBody)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		dumpFailedBody("POST", path, body, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		dumpFailedBody("POST", path, body, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)))
		return fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, string(bodyBytes))
	}

	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) put(path string, body []byte, out interface{}) error {
	url := c.endpoint + path
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, _ := http.NewRequest("PUT", url, reqBody)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		dumpFailedBody("PUT", path, body, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		dumpFailedBody("PUT", path, body, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes)))
		return fmt.Errorf("PUT %s: HTTP %d: %s", path, resp.StatusCode, string(bodyBytes))
	}

	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// dumpFailedBody writes a rejected request body + error to
// ~/.copycard/failed-posts/ so the payload is available for inspection.
// Best-effort: any I/O error during dumping is silently ignored.
func dumpFailedBody(method, path string, body []byte, failure error) {
	if body == nil {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".copycard", "failed-posts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	// Derive a safe suffix from the path (last segment).
	suffix := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		suffix = path[idx+1:]
	}
	if idx := strings.IndexAny(suffix, "?&"); idx >= 0 {
		suffix = suffix[:idx]
	}
	if suffix == "" {
		suffix = "request"
	}
	ts := time.Now().Format("20060102-150405.000000000")
	base := filepath.Join(dir, fmt.Sprintf("%s-%s-%s", ts, method, suffix))
	_ = os.WriteFile(base+".json", body, 0o644)
	_ = os.WriteFile(base+".error.txt", []byte(failure.Error()+"\n"), 0o644)
}

func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	maxRetries := 6
	baseDelay := 500 * time.Millisecond
	maxDelay := 30 * time.Second

	var lastCloudFront bool

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Rewind the request body for retry attempts. http.NewRequest sets
		// GetBody automatically when the body is a *bytes.Reader, *bytes.Buffer,
		// or *strings.Reader. Without this, POST/PUT retries send an empty body.
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("rewind request body: %w", err)
			}
			req.Body = body
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request: %w", err)
		}

		retry, cloudFront := classifyResponse(resp)
		lastCloudFront = cloudFront
		if !retry {
			return resp, nil
		}

		resp.Body.Close()

		if attempt == maxRetries-1 {
			if lastCloudFront {
				return nil, fmt.Errorf("%w: max retries exceeded", ErrCloudFrontBlocked)
			}
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

// classifyResponse decides whether a response warrants a retry and whether
// the cause is CloudFront WAF. CloudFront (in front of Flowboards) returns
// HTML 403s for rate-limited or blocked requests; real Flowboards 403s come
// back as small JSON. On the non-retry path we restore a fresh body reader so
// the caller can still inspect the error.
func classifyResponse(resp *http.Response) (retry, cloudFront bool) {
	switch {
	case resp.StatusCode >= 500:
		return true, false
	case resp.StatusCode == 429:
		return true, false
	case resp.StatusCode == 403:
		body, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(body))
		cf := bytes.Contains(body, []byte("CloudFront")) ||
			bytes.Contains(body, []byte("Request blocked"))
		return cf, cf
	}
	return false, false
}

func (c *Client) acquire() {
	c.semaphore <- struct{}{}
}

func (c *Client) release() {
	<-c.semaphore
}
