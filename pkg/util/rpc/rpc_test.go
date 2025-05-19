// Copyright (C) 2025 wangyusong
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestAPI(t *testing.T) {
	RegisterTestingT(t)

	type TestRequest struct {
		Name string `json:"name"`
	}

	type TestResponse struct {
		Greeting string `json:"greeting"`
	}

	type givenDetail struct {
		handler Handler[TestRequest, TestResponse]
	}
	type whenDetail struct {
		method      string
		requestBody string
	}
	type thenExpected struct {
		statusCode   int
		responseBody string
	}

	successHandler := func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
		return &TestResponse{Greeting: "Hello, " + req.Name}, nil
	}

	badRequestHandler := func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
		return nil, ErrBadRequest(errors.New("invalid request"))
	}

	notFoundHandler := func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
		return nil, ErrNotFound(errors.New("resource not found"))
	}

	internalErrorHandler := func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
		return nil, ErrInternal(errors.New("server error"))
	}

	genericErrorHandler := func(ctx context.Context, req *TestRequest) (*TestResponse, error) {
		return nil, errors.New("generic error")
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Successful request",
			Given:    "a handler that returns a successful response",
			When:     "making a valid request",
			Then:     "should return 200 OK with the expected response",
			GivenDetail: givenDetail{
				handler: successHandler,
			},
			WhenDetail: whenDetail{
				method:      http.MethodPost,
				requestBody: `{"name":"World"}`,
			},
			ThenExpected: thenExpected{
				statusCode:   http.StatusOK,
				responseBody: `{"greeting":"Hello, World"}`,
			},
		},
		{
			Scenario: "Empty request body",
			Given:    "a handler that returns a successful response",
			When:     "making a request with empty body",
			Then:     "should return 200 OK with default values",
			GivenDetail: givenDetail{
				handler: successHandler,
			},
			WhenDetail: whenDetail{
				method:      http.MethodPost,
				requestBody: "",
			},
			ThenExpected: thenExpected{
				statusCode:   http.StatusOK,
				responseBody: `{"greeting":"Hello, "}`,
			},
		},
		{
			Scenario: "Invalid JSON request",
			Given:    "a handler that processes JSON",
			When:     "making a request with invalid JSON",
			Then:     "should return 400 Bad Request",
			GivenDetail: givenDetail{
				handler: successHandler,
			},
			WhenDetail: whenDetail{
				method:      http.MethodPost,
				requestBody: `{"name":`,
			},
			ThenExpected: thenExpected{
				statusCode: http.StatusBadRequest,
			},
		},
		{
			Scenario: "Bad request error",
			Given:    "a handler that returns a bad request error",
			When:     "making a request that triggers a bad request error",
			Then:     "should return 400 Bad Request with error details",
			GivenDetail: givenDetail{
				handler: badRequestHandler,
			},
			WhenDetail: whenDetail{
				method:      http.MethodPost,
				requestBody: `{"name":"World"}`,
			},
			ThenExpected: thenExpected{
				statusCode:   http.StatusBadRequest,
				responseBody: `{"code":400,"message":"invalid request"}`,
			},
		},
		{
			Scenario: "Not found error",
			Given:    "a handler that returns a not found error",
			When:     "making a request that triggers a not found error",
			Then:     "should return 404 Not Found with error details",
			GivenDetail: givenDetail{
				handler: notFoundHandler,
			},
			WhenDetail: whenDetail{
				method:      http.MethodPost,
				requestBody: `{"name":"World"}`,
			},
			ThenExpected: thenExpected{
				statusCode:   http.StatusNotFound,
				responseBody: `{"code":404,"message":"resource not found"}`,
			},
		},
		{
			Scenario: "Internal server error",
			Given:    "a handler that returns an internal server error",
			When:     "making a request that triggers an internal server error",
			Then:     "should return 500 Internal Server Error with error details",
			GivenDetail: givenDetail{
				handler: internalErrorHandler,
			},
			WhenDetail: whenDetail{
				method:      http.MethodPost,
				requestBody: `{"name":"World"}`,
			},
			ThenExpected: thenExpected{
				statusCode:   http.StatusInternalServerError,
				responseBody: `{"code":500,"message":"server error"}`,
			},
		},
		{
			Scenario: "Generic error",
			Given:    "a handler that returns a generic error",
			When:     "making a request that triggers a generic error",
			Then:     "should return 500 Internal Server Error",
			GivenDetail: givenDetail{
				handler: genericErrorHandler,
			},
			WhenDetail: whenDetail{
				method:      http.MethodPost,
				requestBody: `{"name":"World"}`,
			},
			ThenExpected: thenExpected{
				statusCode: http.StatusInternalServerError,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			handler := API(tt.GivenDetail.handler)

			// When.
			var req *http.Request
			if tt.WhenDetail.requestBody == "" {
				req = httptest.NewRequest(tt.WhenDetail.method, "/test", nil)
			} else {
				req = httptest.NewRequest(tt.WhenDetail.method, "/test", bytes.NewBufferString(tt.WhenDetail.requestBody))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Then.
			Expect(rec.Code).To(Equal(tt.ThenExpected.statusCode))

			if tt.ThenExpected.responseBody != "" {
				var expected, actual interface{}
				err := json.Unmarshal([]byte(tt.ThenExpected.responseBody), &expected)
				Expect(err).NotTo(HaveOccurred())

				body, err := io.ReadAll(rec.Body)
				Expect(err).NotTo(HaveOccurred())

				err = json.Unmarshal(body, &actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(Equal(expected))
			}
		})
	}
}
