package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

func checkResponseError(resp *http.Response) error {
	category := strconv.Itoa(resp.StatusCode)[0]
	switch category {
	case '5':
		return newServerError(resp)
	case '4':
		return newClientError(resp)
	case '2':
		return nil
	default:
		return newHTTPError(resp)
	}
}

// HTTPError represents a server or client error in response.
type HTTPError struct {
	Code    int
	Header  http.Header
	Message string
}

func newHTTPError(resp *http.Response) *HTTPError {
	body, err := ioutil.ReadAll(resp.Body)
	message := string(body)
	if err != nil {
		message = "incomplete: " + message
	}
	return &HTTPError{
		Code:    resp.StatusCode,
		Header:  resp.Header,
		Message: message,
	}
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP error; code: %d, message: %s", e.Code, e.Message)
}

// ServerError wraps the error returned to an Efes request.
type ServerError struct {
	*HTTPError
}

func newServerError(resp *http.Response) *ServerError {
	return &ServerError{newHTTPError(resp)}
}

// ClientError wraps the error returned to an Efes request.
type ClientError struct {
	*HTTPError
}

func newClientError(resp *http.Response) *ClientError {
	return &ClientError{newHTTPError(resp)}
}
