package auth

import (
	"context"
	"google.golang.org/grpc/metadata"
	"net/http"
)

type ctxMarker struct{}

var (
	ctxMarkerKey = &ctxMarker{}
	defUser      = &User{
		Email:    "local@freiheit.com",
		Username: "local@freiheit.com",
	}
)

// Extract takes the User from middleware.
// It always returns a User
func Extract(ctx context.Context) *User {
	md, _ := metadata.FromIncomingContext(ctx)
	u := defUser

	// if no user was specified, use default user
	if md.Get("author-email") == nil {
		return u
	}

	u.Email = md.Get("author-email")[0]

	// if no username was specified, use email as username
	if md.Get("author-username") == nil {
		u.Username = md.Get("author-email")[0]
	} else {
		u.Username = md.Get("author-username")[0]
	}

	return u
}

// ToContext adds the User to the context for extraction later.
// Returning the new context that has been created.
func ToContext(ctx context.Context, u *User) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, "author-email", u.Email, "author-username", u.Username)
	return context.WithValue(ctx, ctxMarkerKey, u)
}

// splits of grpc-traffic
type Auth struct {
	HttpServer http.Handler
}

type User struct {
	Email    string
	Username string
}

func (p *Auth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := r.Context()
	u := getActionAuthor()
	p.HttpServer.ServeHTTP(w, r.WithContext(ToContext(c, u)))
}

func getActionAuthor() *User {
	// Local
	return defUser
}
