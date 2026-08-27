// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lemon4ksan/g-man-cli/internal/cli/builder"
	"github.com/lemon4ksan/g-man-cli/internal/cli/output"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var (
	ErrCommunityRequestArgs = errors.New(
		"community request requires method and path (e.g. request community GET profiles/...)",
	)
	ErrUnifiedRequestArgs = errors.New(
		"unified request requires interface and action (e.g. request unified Player GetNickname)",
	)
	ErrWebAPIRequestArgs = errors.New(
		"webapi request requires interface and action (e.g. request webapi ISteamUser GetPlayerSummaries)",
	)
)

var requestCmd = builder.New("request <community|unified|webapi> [args...]").
	Short("Execute raw Steam request (community, unified, webapi)").
	Long(`Executes arbitrary raw Steam requests via the active daemon session.
Examples:
  gmanctl request community GET profiles/76561198000000000/inventory/json/730/2
  gmanctl request unified Player GetNickname version=1 body='{"steamids": [76561198000000000]}'
  gmanctl request webapi ISteamUser GetPlayerSummaries version=2 steamids=76561198000000000`).
	Aliases("req").
	Args(cobra.MinimumNArgs(1)).
	Run(func(c *builder.Context) error {
		reqTypeStr := strings.ToLower(c.Args[0])

		var reqType pb.RequestType
		switch reqTypeStr {
		case "community", "comm":
			reqType = pb.RequestType_REQUEST_TYPE_COMMUNITY
		case "unified", "uni":
			reqType = pb.RequestType_REQUEST_TYPE_UNIFIED
		case "webapi", "api":
			reqType = pb.RequestType_REQUEST_TYPE_WEBAPI
		default:
			return fmt.Errorf("unknown request type %q: expected community, unified, or webapi", c.Args[0])
		}

		req := &pb.ExecRequestRequest{
			Type:   reqType,
			Params: make(map[string]string),
		}

		switch reqType {
		case pb.RequestType_REQUEST_TYPE_COMMUNITY:
			if len(c.Args) < 3 {
				return ErrCommunityRequestArgs
			}

			req.Method = c.Args[1]
			req.Path = c.Args[2]
			parseKeyValArgs(c.Args[3:], req)

		case pb.RequestType_REQUEST_TYPE_UNIFIED:
			if len(c.Args) < 3 {
				return ErrUnifiedRequestArgs
			}

			req.Interface = c.Args[1]
			req.Action = c.Args[2]
			parseKeyValArgs(c.Args[3:], req)

		case pb.RequestType_REQUEST_TYPE_WEBAPI:
			if len(c.Args) < 3 {
				return ErrWebAPIRequestArgs
			}

			req.Interface = c.Args[1]
			req.Action = c.Args[2]
			parseKeyValArgs(c.Args[3:], req)
		}

		c.Renderer.Warningf("Executing %s request to Steam...", reqTypeStr)

		resp, err := c.Client.ExecRequest(c.Ctx, req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}

		if !resp.GetSuccess() {
			return fmt.Errorf("request unsuccessful: %s", resp.GetMessage())
		}

		return c.Render(resp, func() error {
			c.Renderer.Success("Request completed successfully!")

			if resp.GetStatusCode() > 0 {
				fmt.Printf("HTTP Status:   %d\n", resp.GetStatusCode())
			}

			if resp.GetEresult() > 0 {
				fmt.Printf("Steam EResult: %d\n", resp.GetEresult())
			}

			if len(resp.GetHeaders()) > 0 {
				fmt.Printf("\n%s\n", output.TitleStyle.Render("Response Headers:"))

				keys := make([]string, 0, len(resp.GetHeaders()))
				for k := range resp.GetHeaders() {
					keys = append(keys, k)
				}

				sort.Strings(keys)

				for _, k := range keys {
					fmt.Printf("  %s: %s\n", output.KeyStyle.Render(k), resp.GetHeaders()[k])
				}
			}

			if len(resp.GetBody()) > 0 {
				fmt.Printf("\n%s\n", output.TitleStyle.Render("Response Body:"))

				bodyStr := string(resp.GetBody())
				trimmed := strings.TrimSpace(bodyStr)

				if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
					(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
					var pretty bytes.Buffer
					if json.Indent(&pretty, resp.GetBody(), "", "  ") == nil {
						fmt.Println(pretty.String())
						return nil
					}
				}

				fmt.Println(bodyStr)
			}

			return nil
		})
	})

func parseKeyValArgs(args []string, req *pb.ExecRequestRequest) {
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		val := parts[1]

		switch key {
		case "body":
			req.Body = []byte(val)
		case "version":
			if v, err := strconv.Atoi(val); err == nil {
				req.Version = int32(v) //nolint:gosec
			}
		case "is_post_form":
			req.IsPostForm = (val == "true" || val == "1")
		case "method":
			req.Method = val
		default:
			req.Params[key] = val
		}
	}
}
