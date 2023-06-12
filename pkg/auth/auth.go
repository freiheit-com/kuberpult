/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com*/

package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"google.golang.org/grpc/metadata"
)

type ctxMarker struct{}

const (
	defaultEmail = "local.user@freiheit.com"
	defaultName  = "defaultUser"
)

var (
	ctxMarkerKey = &ctxMarker{}
)

func MakeDefaultUser() *User {
	return &User{
		Email: defaultEmail,
		Name:  defaultName,
	}
}

func encode64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func decode64(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	return string(b), err
}

// Extract takes the User from middleware.
// It always returns a User
func Extract(ctx context.Context) *User {
	// check if User is in go Context
	u, ok := ctx.Value(ctxMarkerKey).(*User)
	if !ok {
		// check if User is in Metadata
		md, _ := metadata.FromIncomingContext(ctx)
		originalEmailArr := md.Get("author-email")
		if len(originalEmailArr) == 0 {
			return MakeDefaultUser()
		} else {
			originalEmail := originalEmailArr[0]
			userMail, err := decode64(originalEmail)
			if err != nil {
				logger.FromContext(ctx).Warn(fmt.Sprintf("Extract: non-base64 in author-email %s", originalEmail))
				return MakeDefaultUser()
			}
			originalNameArr := md.Get("author-username")
			if len(originalNameArr) == 0 {
				logger.FromContext(ctx).Warn(fmt.Sprintf("Extract: username undefined but mail defined %s", userMail))
				return MakeDefaultUser()
			}
			originalName := originalNameArr[0]
			userName, err := decode64(originalNameArr[0])
			if err != nil {
				logger.FromContext(ctx).Warn(fmt.Sprintf("Extract: bbbbb non-base64 in author-username %s", userName))
				return MakeDefaultUser()
			}
			logger.FromContext(ctx).Info(fmt.Sprintf("Extract: original mail %s. Decoded: %s", originalEmail, userMail))
			logger.FromContext(ctx).Info(fmt.Sprintf("Extract: original name %s. Decoded: %s", originalName, userName))
			u = &User{
				Email: userMail,
				Name:  userName,
			}
		}
	}
	return u
}

// ToContext adds the User to the context for extraction later.
// Returning the new context that has been created.
func ToContext(ctx context.Context, u *User) context.Context {
	if u == nil || u.Email == "" {
		u = MakeDefaultUser()
	}
	// if no username was specified, use email as username
	if u.Name == "" {
		u.Name = u.Email
	}

	u.Name = encode64(u.Name)
	u.Email = encode64(u.Email)
	ctx = metadata.AppendToOutgoingContext(ctx, "author-email", u.Email, "author-username", u.Name)
	return context.WithValue(ctx, ctxMarkerKey, u)
}

type User struct {
	Email string
	Name  string
}
