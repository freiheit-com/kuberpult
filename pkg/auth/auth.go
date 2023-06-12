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
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"google.golang.org/grpc/metadata"
	"unicode"
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

func MakeSpecialUser() *User {
	return &User{
		Email: "mynamééé.user@freiheit.com",
		Name:  "mynamééé",
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
		if md.Get("author-email") == nil {
			return MakeDefaultUser()
		} else {
			userMail, err := decode64(md.Get("author-email")[0])
			if err != nil {
				logger.FromContext(ctx).Warn(fmt.Sprintf("Extract: non-base64 in author-email %s", md.Get("author-email")[0]))
				return MakeDefaultUser()
			}
			userName, err := decode64(md.Get("author-username")[0])
			if err != nil {
				logger.FromContext(ctx).Warn(fmt.Sprintf("Extract: non-base64 in author-username %s", md.Get("author-username")[0]))
				return MakeDefaultUser()
			}
			logger.FromContext(ctx).Warn(fmt.Sprintf("Extract: oiginal %s. Decoded: %s", md.Get("author-email")[0], userMail))
			u = &User{
				Email: userMail,
				Name:  userName,
			}
		}
	}
	logger.FromContext(ctx).Warn(fmt.Sprintf("Extract: %s", u.Name))

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
	var newMethod = true
	if newMethod {
		logger.FromContext(ctx).Warn(fmt.Sprintf("ToContext 1: Found user.Name: %s", u.Name))
		logger.FromContext(ctx).Warn(fmt.Sprintf("ToContext 1: Found user.Email: %s", u.Email))
		//re := regexp.MustCompile("[[:^ascii:]]")

		t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
		result, _, _ := transform.String(t, "žůžo")
		fmt.Println(result)

		//u.Name = re.ReplaceAllLiteralString(u.Name, "")
		//u.Email = re.ReplaceAllLiteralString(u.Email, "")
		if false {
			u.Name, _, _ = transform.String(t, u.Name)
			u.Email, _, _ = transform.String(t, u.Email)
		} else {
			u.Name = encode64(u.Name)
			u.Email = encode64(u.Email)
		}
		logger.FromContext(ctx).Warn(fmt.Sprintf("ToContext 2: Replaced user.Name: %s", u.Name))
		logger.FromContext(ctx).Warn(fmt.Sprintf("ToContext 2: Replaced user.Email: %s", u.Email))
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "author-email", u.Email, "author-username", u.Name)
	return context.WithValue(ctx, ctxMarkerKey, u)
}

type User struct {
	Email string
	Name  string
}
