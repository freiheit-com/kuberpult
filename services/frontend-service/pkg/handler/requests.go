
package handler

type putLockRequest struct {
	Message   string `json:"message"`
	Signature string `json:"signature,omitempty"`
}
