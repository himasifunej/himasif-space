package mail

import (
	"context"
	"errors"
)

type disabledMailer struct{}

func Disabled() Mailer { return disabledMailer{} }
func (disabledMailer) Send(context.Context, Message) error {
	return errors.New("outgoing email is disabled in this runtime mode")
}
