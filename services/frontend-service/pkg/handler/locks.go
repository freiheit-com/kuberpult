
package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"golang.org/x/crypto/openpgp"
	pgperrors "golang.org/x/crypto/openpgp/errors"
	"io"
	"net/http"
	"strings"
)

func (s Server) handleEnvironmentLocks(w http.ResponseWriter, req *http.Request, environment, tail string) {
	lockID, tail := xpath.Shift(tail)
	if lockID == "" {
		http.Error(w, "missing lock ID", http.StatusNotFound)
		return
	}
	if tail != "/" {
		http.Error(w, fmt.Sprintf("locks does not accept additional path arguments after the lock ID, got: '%s'", tail), http.StatusNotFound)
		return
	}

	switch req.Method {
	case http.MethodPut:
		s.handlePutEnvironmentLock(w, req, environment, lockID)
	case http.MethodDelete:
		s.handleDeleteEnvironmentLock(w, req, environment, lockID)
	default:
		http.Error(w, fmt.Sprintf("unsupported method '%s'", req.Method), http.StatusMethodNotAllowed)
	}
}

func (s Server) handlePutEnvironmentLock(w http.ResponseWriter, req *http.Request, environment, lockID string) {
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, fmt.Sprintf("body must be application/json, got: '%s'", contentType), http.StatusUnsupportedMediaType)
		return
	}

	var body putLockRequest
	invalidMessage := "Please provide lock message in body"
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		decodeError := err.Error()
		if errors.Is(err, io.EOF) {
			decodeError = invalidMessage
		}
		http.Error(w, decodeError, http.StatusBadRequest)
		return
	}

	if len(body.Message) == 0 {
		http.Error(w, invalidMessage, http.StatusBadRequest)
		return
	}

	if s.AzureAuth {
		signature := body.Signature
		if len(signature) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing signature in request body"))
			return
		}

		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environment+lockID), strings.NewReader(signature)); err != nil {
			if err != pgperrors.ErrUnknownIssuer {
				w.WriteHeader(500)
				fmt.Fprintf(w, "Internal: Invalid Signature: %s", err)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Invalid signature")
			return
		}
	}

	_, err := s.LockClient.CreateEnvironmentLock(req.Context(), &api.CreateEnvironmentLockRequest{
		Environment: environment,
		LockId:      lockID,
		Message:     body.Message,
	})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s Server) handleDeleteEnvironmentLock(w http.ResponseWriter, req *http.Request, environment, lockID string) {
	if s.AzureAuth {
		if req.Body == nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "missing request body")
			return
		}
		signature, err := io.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Can't read request body %s", err)
			return
		}

		if len(signature) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Missing signature in request body"))
			return
		}

		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environment+lockID), bytes.NewReader(signature)); err != nil {
			if err != pgperrors.ErrUnknownIssuer {
				w.WriteHeader(500)
				fmt.Fprintf(w, "Internal: Invalid Signature: %s", err)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Invalid signature")
			return
		}
	}

	_, err := s.LockClient.DeleteEnvironmentLock(req.Context(), &api.DeleteEnvironmentLockRequest{
		Environment: environment,
		LockId:      lockID,
	})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
