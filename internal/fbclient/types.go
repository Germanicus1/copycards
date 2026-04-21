package fbclient

import "time"

// Organization profile from config
type OrgProfile struct {
	ID       string // org_id in config
	APIKey   string // resolved (env vars expanded)
	Endpoint string // discovered or overridden
}

// Flowboards API responses
type Board struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	Bins []Bin  `json:"bins"`
}

type Bin struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type TicketType struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type CustomField struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	Type int    `json:"type"` // 1=text, 2=decimal, 3=integer, 4=list
}

type User struct {
	ID    string `json:"_id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type UserGroup struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type Ticket struct {
	ID              string                 `json:"_id"`
	Name            string                 `json:"name"`
	BinID           string                 `json:"bin_id"`
	TicketTypeID    string                 `json:"ticketType_id"`
	Order           float64                `json:"order"`
	EnclosedID      string                 `json:"enclosed_id,omitempty"`
	AssignedIDs     []string               `json:"assigned_ids,omitempty"`
	WatchIDs        []string               `json:"watch_ids,omitempty"`
	Description     string                 `json:"description,omitempty"`
	CustomFields    map[string]interface{} `json:"customFields,omitempty"`
	Checklists      map[string]Checklist   `json:"checklists,omitempty"`
	UpdatedAt       time.Time              `json:"updatedAt"`
	PlannedStartDate string                `json:"plannedStartDate,omitempty"`
	DueDate         string                 `json:"dueDate,omitempty"`
}

type Checklist struct {
	Name  string                  `json:"name"`
	Order float64                 `json:"order"`
	Items map[string]ChecklistItem `json:"items,omitempty"`
}

type ChecklistItem struct {
	Name    string  `json:"name"`
	Order   float64 `json:"order"`
	Checked bool    `json:"checked"`
}

type Attachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Comment struct {
	ID        string    `json:"_id"`
	TicketID  string    `json:"ticket_id"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DirectoryResponse struct {
	RestURLPrefix string `json:"restUrlPrefix"`
}
