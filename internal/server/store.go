package server

type Store struct{}

func NewStorage() *Store {
	return &Store{}
}
