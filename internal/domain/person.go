package domain

import "time"

type Relation string

const (
	RelationDeveloper Relation = "developer"
	RelationCustomer  Relation = "customer"
	RelationAssistant Relation = "assistant"
	RelationColleague Relation = "colleague"
	RelationPersonal  Relation = "personal"
)

type CheckCadence string

const (
	CadenceDaily  CheckCadence = "daily"
	Cadence3Days  CheckCadence = "3days"
	CadenceWeekly CheckCadence = "weekly"
	CadenceMonthly CheckCadence = "monthly"
)

type Person struct {
	ID                  string
	Name                string
	Relation            Relation
	DefaultCheckCadence CheckCadence
	Contact             string
	Active              bool
	Notes               string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
