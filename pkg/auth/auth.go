
package auth

import (
	"context"
	"google.golang.org/grpc/metadata"
)

type ctxMarker struct{}

var (
	ctxMarkerKey = &ctxMarker{}
	DefaultUser  = &User{
		Email: "local.user@freiheit.com",
		Name:  "defaultUser",
	}
)

// Extract takes the User from middleware.
// It always returns a User
func Extract(ctx context.Context) *User {
	// check if User is in go Context
	u, ok := ctx.Value(ctxMarkerKey).(*User)
	if !ok {
		// check if User is in Metadata
		md, _ := metadata.FromIncomingContext(ctx)
		if md.Get("author-email") == nil {
			u = DefaultUser
		} else {
			u = &User{
				Email: md.Get("author-email")[0],
				Name:  md.Get("author-username")[0],
			}
		}
	}
	return u
}

// ToContext adds the User to the context for extraction later.
// Returning the new context that has been created.
func ToContext(ctx context.Context, u *User) context.Context {
	if u == nil || u.Email == "" {
		u = DefaultUser
	}
	// if no username was specified, use email as username
	if u.Name == "" {
		u.Name = u.Email
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "author-email", u.Email, "author-username", u.Name)
	return context.WithValue(ctx, ctxMarkerKey, u)
}

type User struct {
	Email string
	Name  string
}
