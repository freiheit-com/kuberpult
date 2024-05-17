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

Copyright freiheit.com*/

package release

import (
	"bytes"
	"encoding/base64"
	"fmt"
	kutil "github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
	"io"
	"mime/multipart"
	"net/http"
	urllib "net/url"
)

func prepareHttpRequest(url string, authParams kutil.AuthenticationParameters, parsedArgs ReleaseParameters) (*http.Request, error) {
	form := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(form)

	if err := writer.WriteField("application", parsedArgs.Application); err != nil {
		return nil, fmt.Errorf("error writing application field, error: %w", err)
	}

	for environment, manifest := range parsedArgs.Manifests {
		part, err := writer.CreateFormFile(fmt.Sprintf("manifests[%s]", environment), fmt.Sprintf("%s-manifest", environment))
		if err != nil {
			return nil, fmt.Errorf("error creating the form entry for environment %s with manifest file %s, error: %w", environment, manifest, err)
		}
		_, err = part.Write([]byte(manifest))
		if err != nil {
			return nil, fmt.Errorf("error writing the form entry for environment %s with manifest file %s, error: %w", environment, manifest, err)
		}
	}

	for environment, signature := range parsedArgs.Signatures {
		part, err := writer.CreateFormFile(fmt.Sprintf("signatures[%s]", environment), fmt.Sprintf("%s-signature", environment))
		if err != nil {
			return nil, fmt.Errorf("error creating the form entry for environment %s with signature file %s, error: %w", environment, signature, err)
		}
		_, err = part.Write([]byte(signature))
		if err != nil {
			return nil, fmt.Errorf("error writing the form entry for environment %s with signature file %s, error: %w", environment, signature, err)
		}
	}

	if parsedArgs.Team != nil {
		if err := writer.WriteField("team", *parsedArgs.Team); err != nil {
			return nil, fmt.Errorf("error writing team field, error: %w", err)
		}
	}

	if parsedArgs.SourceCommitId != nil {
		if err := writer.WriteField("source_commit_id", *parsedArgs.SourceCommitId); err != nil {
			return nil, fmt.Errorf("error writing source_commit_id field, error: %w", err)
		}
	}

	if parsedArgs.PreviousCommitId != nil {
		if err := writer.WriteField("previous_commit_id", *parsedArgs.PreviousCommitId); err != nil {
			return nil, fmt.Errorf("error writing previous_commit_id field, error: %w", err)
		}
	}

	if parsedArgs.SourceAuthor != nil {
		if err := writer.WriteField("source_author", *parsedArgs.SourceAuthor); err != nil {
			return nil, fmt.Errorf("error writing source_author field, error: %w", err)
		}
	}

	if parsedArgs.SourceMessage != nil {
		if err := writer.WriteField("source_message", *parsedArgs.SourceMessage); err != nil {
			return nil, fmt.Errorf("error writing source_message field, error: %w", err)
		}
	}

	if parsedArgs.Version != nil {
		if err := writer.WriteField("version", fmt.Sprintf("%v", *parsedArgs.Version)); err != nil {
			return nil, fmt.Errorf("error writing version field, error: %w", err)
		}
	}

	if parsedArgs.DisplayVersion != nil {
		if err := writer.WriteField("display_version", *parsedArgs.DisplayVersion); err != nil {
			return nil, fmt.Errorf("error writing display_version field, error: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("error closing the writer, error: %w", err)
	}

	urlStruct, err := urllib.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("the provided url %s is invalid, error: %w", url, err)
	}

	urlStruct = urlStruct.JoinPath("release")

	req, err := http.NewRequest(http.MethodPost, urlStruct.String(), form)
	if err != nil {
		return nil, fmt.Errorf("error creating the HTTP request, error: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if authParams.IapToken != nil {
		req.Header.Add("Authorization", "Bearer "+*authParams.IapToken)
	}

	if authParams.AuthorName != nil {
		req.Header.Add("author-name", base64.StdEncoding.EncodeToString([]byte(*authParams.AuthorName)))
	}

	if authParams.AuthorEmail != nil {
		req.Header.Add("author-email", base64.StdEncoding.EncodeToString([]byte(*authParams.AuthorEmail)))
	}

	return req, nil
}

func issueHttpRequest(req http.Request) error {
	client := &http.Client{}
	resp, err := client.Do(&req)
	if err != nil {
		return fmt.Errorf("error issuing the HTTP request, error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("response was not OK or Accepted\nresponse code: %v\nresponse body could not be be read, error: %w", resp.StatusCode, err)
		}
		strBody := string(body)
		return fmt.Errorf("response was not OK or Accepted\nresponse code: %v\nresponse body:\n   %v", resp.StatusCode, strBody)
	}

	return nil
}
