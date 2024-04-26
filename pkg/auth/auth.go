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
	"errors"
	"fmt"
	"net/http"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"google.golang.org/grpc/metadata"
)

type ctxMarker struct{}

var (
	ctxMarkerKey = &ctxMarker{}
)

/*
The frontend-service now defines the default author for git commits.
The frontend-service also allows overwriting the default values, see function `getRequestAuthorFromGoogleIAP`.
The cd-service generally expects these headers, either in the grpc context or the http headers.
*/
const (
	HeaderUserName   = "author-name"
	HeaderUserEmail  = "author-email"
	HeaderUserRole   = "author-role"
	HeaderClaimsInfo = "author-claims"
)

func Encode64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func Decode64(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	return string(b), err
}

// ReadUserFromContext returns a user from the ctx or an error if none was found, or it is invalid
func ReadUserFromContext(ctx context.Context) (*User, error) {
	u, ok := ctx.Value(ctxMarkerKey).(*User)
	if !ok || u == nil {
		return nil, grpc.InternalError(ctx, errors.New("could not read user from context"))
	}
	return u, nil
}

// WriteUserToContext should be used in both frontend-service and cd-service.
// WriteUserToContext adds the User to the context for extraction later.
// The user must not be nil.
// Returning the new context that has been created.
func WriteUserToContext(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxMarkerKey, &u)
}

func WriteUserToGrpcContext(ctx context.Context, u User) context.Context {
	return metadata.AppendToOutgoingContext(ctx, HeaderUserEmail, Encode64(u.Email), HeaderUserName, Encode64(u.Name))
}

// WriteUserRoleToGrpcContext adds the user role to the GRPC context.
// Only used when RBAC is enabled.
func WriteUserRoleToGrpcContext(ctx context.Context, userRole string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, HeaderUserRole, Encode64(userRole))
}

// WriteUserRoleToGrpcContext adds the user claims to the GRPC context.
// Only used when RBAC is enabled.
func WriteUserClaimsToGrpcContext(ctx context.Context, claims string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, HeaderClaimsInfo, Encode64(claims))
}

type GrpcContextReader interface {
	ReadUserFromGrpcContext(ctx context.Context) (*User, error)
}

type DexGrpcContextReader struct {
	DexEnabled bool
}

type DummyGrpcContextReader struct {
	Role string
}

func (x *DummyGrpcContextReader) ReadUserFromGrpcContext(ctx context.Context) (*User, error) {
	user := &User{
		Email: "dummyMail@example.com",
		Name:  "userName",
		DexAuthContext: &DexAuthContext{
			Role: x.Role,
		},
	}
	return user, nil
}

// ReadUserFromGrpcContext should only be used in the cd-service.
// ReadUserFromGrpcContext takes the User from middleware (context).
// It returns a User or an error if the user is not found.
func (x *DexGrpcContextReader) ReadUserFromGrpcContext(ctx context.Context) (*User, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpc.AuthError(ctx, errors.New("could not retrieve metadata context with git author in grpc context"))
	}
	originalEmailArr := md.Get(HeaderUserEmail)
	if len(originalEmailArr) != 1 {
		return nil, grpc.AuthError(ctx, fmt.Errorf("did not find exactly 1 author-email in grpc context: %+v", originalEmailArr))
	}
	originalEmail := originalEmailArr[0]
	userMail, err := Decode64(originalEmail)
	if err != nil {
		return nil, grpc.AuthError(ctx, fmt.Errorf("extract: non-base64 in author-email in grpc context %s", originalEmail))
	}
	originalNameArr := md.Get(HeaderUserName)
	if len(originalNameArr) != 1 {
		return nil, grpc.AuthError(ctx, fmt.Errorf("did not find exactly 1 author-name in grpc context %+v", originalNameArr))
	}
	originalName := originalNameArr[0]
	userName, err := Decode64(originalName)
	if err != nil {
		return nil, grpc.AuthError(ctx, fmt.Errorf("extract: non-base64 in author-username in grpc context %s", userName))
	}
	logger.FromContext(ctx).Info(fmt.Sprintf("Extract: original mail %s. Decoded: %s", originalEmail, userMail))
	logger.FromContext(ctx).Info(fmt.Sprintf("Extract: original name %s. Decoded: %s", originalName, userName))
	u := &User{
		DexAuthContext: nil,
		Email:          userMail,
		Name:           userName,
	}
	if u.Email == "" || u.Name == "" {
		return nil, grpc.AuthError(ctx, errors.New("email and name in grpc context cannot both be empty"))
	}
	// RBAC Role of the user. only mandatory if DEX is enabled.
	if x.DexEnabled {
		rolesInHeader := md.Get(HeaderUserRole)
		if len(rolesInHeader) == 0 {
			return nil, grpc.AuthError(ctx, fmt.Errorf("extract: role undefined but dex is enabled"))
		}
		userRole, err := Decode64(rolesInHeader[0])
		if err != nil {
			return nil, grpc.AuthError(ctx, fmt.Errorf("extract: non-base64 in author-role in grpc context %s", userRole))
		}
		u.DexAuthContext = &DexAuthContext{
			Role: userRole,
		}
	}
	return u, nil
}

// ReadUserFromHttpHeader should only be used in the cd-service.
// ReadUserFromHttpHeader takes the User from the http request.
// It returns a User or an error if the user is not found.
func ReadUserFromHttpHeader(ctx context.Context, r *http.Request) (*User, error) {
	headerEmail64 := r.Header.Get(HeaderUserEmail)
	headerEmail, err := Decode64(headerEmail64)
	if err != nil {
		return nil, grpc.AuthError(ctx, fmt.Errorf("ExtractUserHttp: invalid data in email: '%s'", headerEmail64))
	}
	headerName64 := r.Header.Get(HeaderUserName)
	headerName, err := Decode64(headerName64)
	if err != nil {
		return nil, grpc.AuthError(ctx, fmt.Errorf("ExtractUserHttp: invalid data in name: '%s'", headerName64))
	}
	headerRole64 := r.Header.Get(HeaderUserRole)
	headerRole, err := Decode64(headerRole64)
	if err != nil {
		return nil, grpc.AuthError(ctx, fmt.Errorf("ExtractUserHttp: invalid data in role: '%s'", headerRole64))
	}

	if headerName != "" && headerEmail != "" {
		return &User{
			Email: headerEmail,
			Name:  headerName,
			DexAuthContext: &DexAuthContext{
				Role: headerRole,
			},
		}, nil
	}
	return nil, nil // no user, but the user is not always necessary
}

// WriteUserToHttpHeader should only be used in the frontend-service
// WriteUserToHttpHeader writes the user into http headers
// it is used for requests like /release which are delegated from frontend-service to cd-service
func WriteUserToHttpHeader(r *http.Request, user User) {
	r.Header.Set(HeaderUserName, Encode64(user.Name))
	r.Header.Set(HeaderUserEmail, Encode64(user.Email))
}

// WriteUserRoleToHttpHeader should only be used in the frontend-service
// WriteUserRoleToHttpHeader writes the user role into http headers
// it is used for requests like /release and managing locks which are delegated from frontend-service to cd-service
func WriteUserRoleToHttpHeader(r *http.Request, role string) {
	r.Header.Add(HeaderUserRole, Encode64(role))
}

func GetUserOrDefault(u *User, defaultUser User) User {
	var userAdapted = User{
		DexAuthContext: nil,
		Email:          defaultUser.Email,
		Name:           defaultUser.Name,
	}
	if u != nil && u.Email != "" {
		userAdapted.Email = u.Email
		// if no username was specified, use email as username
		if u.Name == "" {
			userAdapted.Name = u.Email
		} else {
			userAdapted.Name = u.Name
		}
	}
	if u != nil && u.DexAuthContext != nil {
		userAdapted.DexAuthContext = u.DexAuthContext
	} else {
		userAdapted.DexAuthContext = defaultUser.DexAuthContext
	}
	return userAdapted
}

type User struct {
	Email string
	Name  string
	// Optional. User role, only used if RBAC is enabled.
	DexAuthContext *DexAuthContext
}
