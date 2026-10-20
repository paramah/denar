package rest

import (
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/user"
)

type usecaseUser = user.User

func domainID(s string) domain.ID { return domain.ID(s) }
