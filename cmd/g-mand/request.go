// Copyright (c) 2026 lemon4ksan. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/community"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

// ExecRequest executes a generic request to Steam (Community, Unified, or WebAPI)
func (s *Daemon) ExecRequest(ctx context.Context, req *pb.ExecRequestRequest) (*pb.ExecRequestResponse, error) {
	s.logger.Info("Exec request",
		log.String("type", req.GetType().String()),
		log.String("interface", req.GetInterface()),
		log.String("action", req.GetAction()),
		log.String("path", req.GetPath()),
	)

	switch req.GetType() {
	case pb.RequestType_REQUEST_TYPE_COMMUNITY:
		return s.execCommunity(ctx, req)
	case pb.RequestType_REQUEST_TYPE_UNIFIED:
		return s.execUnified(ctx, req)
	case pb.RequestType_REQUEST_TYPE_WEBAPI:
		return s.execWebAPI(ctx, req)
	case pb.RequestType_REQUEST_TYPE_UNSPECIFIED:
		return nil, errors.New("unspecified request type")
	default:
		return nil, fmt.Errorf("unsupported request type: %v", req.GetType())
	}
}

func (s *Daemon) execCommunity(ctx context.Context, req *pb.ExecRequestRequest) (*pb.ExecRequestResponse, error) {
	comm := s.client.Community()
	if comm == nil {
		return nil, errors.New("steam community client not authenticated or initialized")
	}

	method := strings.ToUpper(req.GetMethod())
	if method == "" {
		method = http.MethodGet
	}

	var mods []aoni.RequestModifier

	if method == http.MethodPost {
		if req.GetIsPostForm() {
			formData := url.Values{}
			for k, v := range req.GetParams() {
				formData.Set(k, v)
			}

			if formData.Get("sessionid") == "" {
				formData.Set("sessionid", comm.SessionID(community.BaseURL))
			}

			mods = append(mods,
				aoni.WithBody(strings.NewReader(formData.Encode())),
				aoni.WithContentType("application/x-www-form-urlencoded; charset=UTF-8"),
				aoni.WithHeader("Accept", "application/json, text/javascript; q=0.01"),
			)
		} else {
			var query url.Values
			if sid := comm.SessionID(community.BaseURL); sid != "" {
				query = url.Values{"sessionid": {sid}}
			}

			if len(query) > 0 {
				mods = append(mods, aoni.WithQuery(query))
			}

			if len(req.GetBody()) > 0 {
				mods = append(mods, aoni.WithBody(bytes.NewReader(req.GetBody())))
			}

			mods = append(mods,
				aoni.WithContentType("application/json; charset=UTF-8"),
				aoni.WithHeader("Accept", "application/json"),
			)
		}
	} else {
		if len(req.GetParams()) > 0 {
			q := url.Values{}
			for k, v := range req.GetParams() {
				q.Set(k, v)
			}

			mods = append(mods, aoni.WithQuery(q))
		}

		mods = append(mods,
			aoni.WithHeader("Accept", "application/json, text/javascript; q=0.01"),
			aoni.WithHeader("X-Requested-With", "XMLHttpRequest"),
		)
	}

	resp, err := comm.Request(ctx, method, req.GetPath(), mods...)
	if err != nil {
		return nil, fmt.Errorf("community request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read community response: %w", err)
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &pb.ExecRequestResponse{
		Success:    true,
		Message:    fmt.Sprintf("Community request completed with status %d", resp.StatusCode),
		Body:       respBody,
		StatusCode: int32(resp.StatusCode), // #nosec G115
		Headers:    headers,
	}, nil
}

func (s *Daemon) execUnified(ctx context.Context, req *pb.ExecRequestRequest) (*pb.ExecRequestResponse, error) {
	iface := req.GetInterface()
	action := req.GetAction()

	version := int(req.GetVersion())
	if version <= 0 {
		version = 1
	}

	method := req.GetMethod()
	if method == "" {
		method = http.MethodPost
	}

	var (
		reqType  protoreflect.MessageType
		respType protoreflect.MessageType
	)

	expectedReqSuffix := fmt.Sprintf("C%s_%s_Request", iface, action)
	expectedRespSuffix := fmt.Sprintf("C%s_%s_Response", iface, action)

	protoregistry.GlobalTypes.RangeMessages(func(mt protoreflect.MessageType) bool {
		name := string(mt.Descriptor().Name())
		switch name {
		case expectedReqSuffix:
			reqType = mt
		case expectedRespSuffix:
			respType = mt
		}

		return reqType == nil || respType == nil
	})

	var (
		bodyBytes []byte
		err       error
	)

	if reqType != nil && len(req.GetBody()) > 0 && req.GetBody()[0] == '{' {
		reqMsg := reqType.New().Interface()
		if err := protojson.Unmarshal(req.GetBody(), reqMsg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON to protobuf %s: %w", expectedReqSuffix, err)
		}

		bodyBytes, err = proto.Marshal(reqMsg)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal protobuf message %s: %w", expectedReqSuffix, err)
		}
	} else {
		bodyBytes = req.GetBody()
	}

	unifiedReq, err := service.NewUnifiedRequest(method, iface, action, version, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create unified request: %w", err)
	}

	resp, err := s.client.Do(ctx, unifiedReq)
	if err != nil {
		return nil, fmt.Errorf("unified request execution failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read unified response: %w", err)
	}

	var (
		eresult    uint32
		statusCode int32
	)

	headers := make(map[string]string)

	if httpMeta, ok := resp.HTTP(); ok {
		eresult = uint32(httpMeta.Result) // #nosec G115

		statusCode = int32(httpMeta.StatusCode) // #nosec G115
		for k, v := range httpMeta.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}
	} else if socketMeta, ok := resp.Socket(); ok {
		eresult = uint32(socketMeta.Result) // #nosec G115
	}

	var outputBytes []byte
	if respType != nil && eresult == uint32(enums.EResult_OK) {
		respMsg := respType.New().Interface()
		if err := proto.Unmarshal(respBytes, respMsg); err == nil {
			if jsonBytes, err := protojson.Marshal(respMsg); err == nil {
				outputBytes = jsonBytes
			}
		}
	}

	if len(outputBytes) == 0 {
		outputBytes = respBytes
	}

	return &pb.ExecRequestResponse{
		Success:    true,
		Message:    fmt.Sprintf("Unified request %s.%s#%d completed", iface, action, version),
		Body:       outputBytes,
		StatusCode: statusCode,
		Eresult:    eresult,
		Headers:    headers,
	}, nil
}

func (s *Daemon) execWebAPI(ctx context.Context, req *pb.ExecRequestRequest) (*pb.ExecRequestResponse, error) {
	iface := req.GetInterface()
	action := req.GetAction()

	version := int(req.GetVersion())
	if version <= 0 {
		version = 1
	}

	method := req.GetMethod()
	if method == "" {
		method = http.MethodGet
	}

	webapiReq := service.NewWebAPIRequest(method, iface, action, version)

	if len(req.GetParams()) > 0 {
		params := url.Values{}
		for k, v := range req.GetParams() {
			params.Set(k, v)
		}

		webapiReq.WithParams(params)
	}

	resp, err := s.client.Do(ctx, webapiReq)
	if err != nil {
		return nil, fmt.Errorf("webapi request execution failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read webapi response: %w", err)
	}

	var (
		eresult    uint32
		statusCode int32
	)

	headers := make(map[string]string)

	if httpMeta, ok := resp.HTTP(); ok {
		eresult = uint32(httpMeta.Result)       // #nosec G115
		statusCode = int32(httpMeta.StatusCode) // #nosec G115

		for k, v := range httpMeta.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}
	}

	return &pb.ExecRequestResponse{
		Success:    true,
		Message:    fmt.Sprintf("WebAPI request %s/%s/v%d completed", iface, action, version),
		Body:       respBytes,
		StatusCode: statusCode,
		Eresult:    eresult,
		Headers:    headers,
	}, nil
}
