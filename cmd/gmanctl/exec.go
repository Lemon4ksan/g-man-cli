// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/spf13/cobra"

	"github.com/lemon4ksan/g-man-cli/internal/cli/builder"
	"github.com/lemon4ksan/g-man-cli/internal/cli/output"
	"github.com/lemon4ksan/g-man-cli/pkg/game"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

var execCmd = builder.New("exec <appid> [action] [params...]").
	Short("Execute game action or query supported actions (e.g. exec 440 inventory)").
	Long(`Executes game coordinator actions directly against the active game driver.
If no action or 'list' is specified, displays all supported action commands for the target game.`).
	Args(cobra.MinimumNArgs(1)).
	Run(func(c *builder.Context) error {
		appID64, err := strconv.ParseUint(c.Args[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid AppID %q: %w", c.Args[0], err)
		}

		appID := uint32(appID64)

		if len(c.Args) < 2 || c.Args[1] == "help" || c.Args[1] == "list" {
			return listGameActions(c, appID)
		}

		action := c.Args[1]

		params := make(map[string]string)
		for _, arg := range c.Args[2:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				params[parts[0]] = parts[1]
			} else {
				params[arg] = "true"
			}
		}

		c.Renderer.Warningf("Executing action %q on game AppID %d...", action, appID)

		resp, err := c.Client.ExecAction(c.Ctx, &pb.ExecActionRequest{
			Appid:  appID,
			Action: action,
			Params: params,
		})
		if err != nil {
			return fmt.Errorf("action %q failed: %w", action, err)
		}

		return c.Render(resp, func() error {
			c.Renderer.Success("Action completed! Result message:")
			fmt.Println(resp.GetMessage())

			if resp.GetDetails() != "" {
				fmt.Println(resp.GetDetails())
			}

			if len(resp.GetItems()) > 0 {
				var sch *schema.Schema

				schemaResp, err := c.Client.ExecAction(c.Ctx, &pb.ExecActionRequest{Appid: appID, Action: "schema"})
				if err == nil {
					var raw schema.Raw
					if json.Unmarshal([]byte(schemaResp.GetDetails()), &raw) == nil {
						sch = schema.New(&raw)
					}
				}

				printItemsTable(resp.GetItems(), sch)
			}

			return nil
		})
	})

func listGameActions(c *builder.Context, appID uint32) error {
	resp, err := c.Client.ExecAction(c.Ctx, &pb.ExecActionRequest{
		Appid:  appID,
		Action: "list-actions",
	})
	if err != nil {
		return fmt.Errorf("failed to list actions: %w", err)
	}

	var actions []game.ActionInfo
	if err := json.Unmarshal([]byte(resp.GetDetails()), &actions); err != nil {
		return fmt.Errorf("failed to parse actions JSON: %w", err)
	}

	return c.Render(actions, func() error {
		fmt.Printf(
			"\n%s\n\n",
			output.TitleStyle.Render(fmt.Sprintf("=== SUPPORTED EXEC ACTIONS FOR APPID %d ===", appID)),
		)

		if len(actions) == 0 {
			fmt.Println("No actions supported by this driver.")
			return nil
		}

		for _, act := range actions {
			fmt.Printf("  %s\n", output.SuccessStyle.Render(act.Name))
			fmt.Printf("    Description: %s\n", act.Description)

			if len(act.Params) > 0 {
				fmt.Printf("    Parameters:\n")

				for _, param := range act.Params {
					reqStr := "optional"
					if param.Required {
						reqStr = "required"
					}

					fmt.Printf("      - %s (%s): %s\n",
						output.WarningStyle.Render(param.Name),
						reqStr,
						param.Description,
					)
				}
			}

			fmt.Println()
		}

		return nil
	})
}

func printItemsTable(items []*pb.Item, s *schema.Schema) {
	fmt.Printf("\n%s\n\n", output.TitleStyle.Render(fmt.Sprintf("=== BACKPACK INVENTORY (%d items) ===", len(items))))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.Debug)
	fmt.Fprintln(w, "Asset ID\tDef Index\tName\tQuality\tQuantity\tTradable\tCraftable\tSKU")
	fmt.Fprintln(w, "--------\t---------\t----\t-------\t--------\t--------\t---------\t---")

	for _, item := range items {
		tradStr := "Yes"
		if !item.GetIsTradable() {
			tradStr = "No"
		}

		craftStr := "Yes"
		if !item.GetIsCraftable() {
			craftStr = "No"
		}

		skuStr := item.GetAttributes()["sku"]
		if skuStr == "" {
			skuStr = "N/A"
		}

		itemName := resolveItemName(skuStr, int(item.GetDefIndex()), item.GetAttributes(), false, s)
		qualityStr := resolveQuality(item.GetQuality(), s, nil)

		fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%d\t%s\t%s\t%s\n",
			item.GetAssetId(),
			item.GetDefIndex(),
			itemName,
			qualityStr,
			item.GetQuantity(),
			tradStr,
			craftStr,
			skuStr,
		)
	}

	_ = w.Flush()
}

func resolveQuality(quality uint32, schema *schema.Schema, fallbackQualityName map[uint32]string) string {
	if schema != nil {
		if qName := schema.QualityByID(int(quality)); qName != "" {
			return qName
		}
	}

	if qName, ok := fallbackQualityName[quality]; ok {
		return qName
	}

	return strconv.FormatUint(uint64(quality), 10)
}

func resolveItemName(
	skuStr string,
	defIndex int,
	attributes map[string]string,
	simple bool,
	schema *schema.Schema,
) string {
	if schema == nil || skuStr == "" || skuStr == "N/A" {
		return "Unknown Item"
	}

	var customName string
	if attributes != nil {
		customName = attributes["custom_name"]
	}

	skuItem, err := sku.FromString(skuStr)
	if err != nil {
		if customName != "" {
			return fmt.Sprintf("''%s''", customName)
		}

		if item := schema.ItemByDef(defIndex); item != nil {
			return item.ItemName
		}

		return "Unknown Item"
	}

	if attributes != nil && !simple {
		if pvStr, ok := attributes["part_values"]; ok && pvStr != "" {
			skuItem.PartValues = make(map[int]int)
			for pair := range strings.SplitSeq(pvStr, ",") {
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					k, _ := strconv.Atoi(parts[0])
					v, _ := strconv.Atoi(parts[1])
					skuItem.PartValues[k] = v
				}
			}
		}
	}

	name := schema.ItemName(skuItem, true, true, false)
	if name == "" {
		if customName != "" {
			return fmt.Sprintf("''%s''", customName)
		}

		sch := schema.ItemByDef(defIndex)
		if sch != nil {
			return sch.ItemName
		}

		return "Unknown Item"
	}

	if customName != "" {
		return fmt.Sprintf("''%s'' (%s)", customName, name)
	}

	return name
}
