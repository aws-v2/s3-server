package domain

type ActorType string

const (
	ActorUser   ActorType = "user"
	ActorService ActorType = "service"
)

type Actor struct {
	ID   string
	Type ActorType
	Auth string // "api_key" | "bearer"
}
