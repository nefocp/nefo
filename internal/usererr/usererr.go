// Package usererr describes problems with what the user asked for, such as
// an invalid domain. The API answers them with 400 and a stable code that the
// UI translates; Detail (a name, a path) can appear in the message.
package usererr

// Error is a problem with the user's request.
type Error struct {
	Code   string
	Detail string
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return e.Code + ": " + e.Detail
	}
	return e.Code
}

// New returns an *Error with an optional detail.
func New(code string, detail ...string) error {
	e := &Error{Code: code}
	if len(detail) > 0 {
		e.Detail = detail[0]
	}
	return e
}
