package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/priahin-i/assistente_2/internal/domain"
	"github.com/priahin-i/assistente_2/internal/store"
)

type PersonTools struct {
	repo store.Repository
}

func NewPersonTools(repo store.Repository) *PersonTools {
	return &PersonTools{repo: repo}
}

type PersonUpsertInput struct {
	ID                  string `json:"id" jsonschema:"unique person slug"`
	Name                string `json:"name" jsonschema:"display name"`
	Relation            string `json:"relation" jsonschema:"developer, customer, assistant, colleague, or personal"`
	DefaultCheckCadence string `json:"default_check_cadence" jsonschema:"daily, 3days, weekly, or monthly"`
	Contact             string `json:"contact,omitempty"`
	Active              *bool  `json:"active,omitempty"`
	Notes               string `json:"notes,omitempty"`
}

type PersonIDInput struct {
	ID string `json:"id" jsonschema:"person id"`
}

type PersonResponse struct {
	Person PersonView `json:"person"`
}

type PersonListResponse struct {
	Items []PersonView `json:"items"`
	Count int          `json:"count"`
}

type PersonGetResponse struct {
	Person          PersonView    `json:"person"`
	ActiveWaiting   []WaitingView `json:"active_waiting"`
	ActiveWaitingCount int        `json:"active_waiting_count"`
}

type PersonView struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Relation            string `json:"relation"`
	DefaultCheckCadence string `json:"default_check_cadence"`
	Contact             string `json:"contact,omitempty"`
	Active              bool   `json:"active"`
	Notes               string `json:"notes,omitempty"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

func (t *PersonTools) Upsert(ctx context.Context, req *mcp.CallToolRequest, input PersonUpsertInput) (*mcp.CallToolResult, PersonResponse, error) {
	_ = req
	existing, err := t.repo.GetPerson(ctx, input.ID)
	isNew := err != nil
	if err != nil && err != domain.ErrNotFound {
		return nil, PersonResponse{}, err
	}

	person := domain.Person{
		ID:                  input.ID,
		Name:                input.Name,
		Relation:            domain.Relation(input.Relation),
		DefaultCheckCadence: domain.CheckCadence(input.DefaultCheckCadence),
		Contact:             input.Contact,
		Active:              true,
		Notes:               input.Notes,
	}
	if !isRelation(person.Relation) {
		return nil, PersonResponse{}, fmt.Errorf("%w: invalid relation", domain.ErrInvalidInput)
	}
	if !isCadence(person.DefaultCheckCadence) {
		return nil, PersonResponse{}, fmt.Errorf("%w: invalid default_check_cadence", domain.ErrInvalidInput)
	}
	if input.Active != nil {
		person.Active = *input.Active
	}
	if !isNew {
		person.CreatedAt = existing.CreatedAt
	}
	if err := t.repo.UpsertPerson(ctx, person); err != nil {
		return nil, PersonResponse{}, err
	}
	saved, err := t.repo.GetPerson(ctx, person.ID)
	if err != nil {
		return nil, PersonResponse{}, err
	}
	return nil, PersonResponse{Person: toPersonView(saved)}, nil
}

func (t *PersonTools) List(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, PersonListResponse, error) {
	_ = req
	people, err := t.repo.ListPeople(ctx)
	if err != nil {
		return nil, PersonListResponse{}, err
	}
	views := make([]PersonView, 0, len(people))
	for _, person := range people {
		views = append(views, toPersonView(person))
	}
	return nil, PersonListResponse{Items: views, Count: len(views)}, nil
}

func (t *PersonTools) Get(ctx context.Context, req *mcp.CallToolRequest, input PersonIDInput) (*mcp.CallToolResult, PersonGetResponse, error) {
	_ = req
	person, err := t.repo.GetPerson(ctx, input.ID)
	if err != nil {
		return nil, PersonGetResponse{}, err
	}
	status := domain.StatusActive
	waitingItems, err := t.repo.ListWaiting(ctx, domain.WaitingFilter{
		Status:      &status,
		Responsible: person.ID,
	})
	if err != nil {
		return nil, PersonGetResponse{}, err
	}
	views := make([]WaitingView, 0, len(waitingItems))
	for _, item := range waitingItems {
		views = append(views, toWaitingView(item))
	}
	return nil, PersonGetResponse{
		Person:             toPersonView(person),
		ActiveWaiting:      views,
		ActiveWaitingCount: len(views),
	}, nil
}

func toPersonView(person domain.Person) PersonView {
	return PersonView{
		ID:                  person.ID,
		Name:                person.Name,
		Relation:            string(person.Relation),
		DefaultCheckCadence: string(person.DefaultCheckCadence),
		Contact:             person.Contact,
		Active:              person.Active,
		Notes:               person.Notes,
		CreatedAt:           formatTimestamp(person.CreatedAt),
		UpdatedAt:           formatTimestamp(person.UpdatedAt),
	}
}

func isRelation(value domain.Relation) bool {
	switch value {
	case domain.RelationDeveloper, domain.RelationCustomer, domain.RelationAssistant, domain.RelationColleague, domain.RelationPersonal:
		return true
	default:
		return false
	}
}

func isCadence(value domain.CheckCadence) bool {
	switch value {
	case domain.CadenceDaily, domain.Cadence3Days, domain.CadenceWeekly, domain.CadenceMonthly:
		return true
	default:
		return false
	}
}
