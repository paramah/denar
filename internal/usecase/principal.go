package usecase

import (
	"git.eiac.dev/tools/denar/internal/domain"
	"git.eiac.dev/tools/denar/internal/domain/bot"
	"git.eiac.dev/tools/denar/internal/domain/user"
)

// Principal is who a use case acts for: the signed-in user, or the bot a
// token belongs to. Exactly one of the two is set. A principal is not tied
// to a household: a person acts in each household they are a member of,
// a bot only in its own. access.household gives the scope of one
// household.
type Principal struct {
	User user.User
	Bot  bot.Bot
}

// IsBot reports whether the principal is a bot.
func (p Principal) IsBot() bool { return p.Bot.ID != "" }

// Actor returns the principal as the actor of the changes it makes.
func (p Principal) Actor() domain.Actor {
	if p.IsBot() {
		return domain.Actor{Kind: domain.ActorBot, ID: p.Bot.ID}
	}
	return domain.Actor{Kind: domain.ActorUser, ID: p.User.ID}
}

// SystemActor is the actor of changes made from the command line.
var SystemActor = domain.Actor{Kind: domain.ActorSystem, ID: "cli"}
