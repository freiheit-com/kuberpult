/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package auth

import (
	"context"
	"google.golang.org/grpc/metadata"
)

type ctxMarker struct{}

var (
	ctxMarkerKey = &ctxMarker{}
	defaultUser  = &User{
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
			u = defaultUser
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
		u = defaultUser
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

func GetActionAuthor() *User {
	// Local
	return defaultUser
}
