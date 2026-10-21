package rest

import (
	"github.com/paramah/denar/internal/domain"
	"github.com/paramah/denar/internal/domain/user"
)

type usecaseUser = user.User

func domainID(s string) domain.ID { return domain.ID(s) }
