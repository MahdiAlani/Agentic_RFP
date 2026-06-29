package user

import "context"

type Service interface {
	GetUser(ctx context.Context, id int) (*User, error)
	CreateUser(ctx context.Context, email, name string) (*User, error)
	UpdateUser(ctx context.Context, id int, email, name string) (*User, error)
	DeleteUser(ctx context.Context, id int) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetUser(ctx context.Context, id int) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) CreateUser(ctx context.Context, email, name string) (*User, error) {
	return s.repo.Create(ctx, &User{Email: email, Name: name})
}

func (s *service) UpdateUser(ctx context.Context, id int, email, name string) (*User, error) {
	return s.repo.Update(ctx, &User{ID: id, Email: email, Name: name})
}

func (s *service) DeleteUser(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
