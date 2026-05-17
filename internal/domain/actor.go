package domain

type ActorType string

const (
	ActorUser   ActorType = "user"
	ActorService ActorType = "service"
	ActorSystem ActorType = "SYSTEM"
)

type Actor struct {
	ID   string
	Role string
	Auth string // "api_key" | "bearer"
}
