package mcpserver

import (
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Enrich the schema actually sent by tools/list, rather than maintaining a
// second, documentation-only description that clients never see.
func addTool[In, Out any](server *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	schema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err)
	}
	for name, property := range schema.Properties {
		description, ok := parameterDescriptions[name]
		if !ok {
			panic("missing MCP parameter description: " + name)
		}
		property.Description = description
		if name == "expected_version" && tool.Name != "create_epic" {
			property.Minimum = number(1)
		}
		if name == "color" {
			property.Pattern = "^#[0-9a-fA-F]{6}$"
		}
		if name == "mode" {
			property.Enum = []any{"project_only", "with_cards"}
		}
		if name == "restore_mode" {
			property.Enum = []any{"", "project_only", "with_cards"}
		}
		if name == "type" {
			property.Enum = []any{"", "card", "project"}
		}
		if name == "state" {
			property.Enum = []any{"", "archived", "restored", "all"}
		}
	}
	if tool.Name == "create_epic" {
		for _, name := range []string{"expected_version", "restore_mode", "epic_id"} {
			schema.Properties[name].Description = "Deprecated compatibility field; ignored on creation. Omit it."
			schema.Properties[name].Deprecated = true
		}
		schema.Properties["is_archived"].Description = "Must be false on creation; archived projects cannot be created."
	}
	if p := schema.Properties["limit"]; p != nil {
		def, max := 50, 100
		if tool.Name == "list_card_movements" {
			def, max = 100, 500
		}
		rule := "Omitted, zero or negative uses the default."
		if tool.Name == "list_archive" {
			def = 30
			p.Minimum = number(0)
			rule = "Omitted or zero uses the default; negative is invalid."
		}
		p.Maximum = number(float64(max))
		p.Description = fmt.Sprintf("Page size: use 1..%d; default %d. %s Never exceed %d; use next_cursor to read further pages where supported.", max, def, rule, max)
		tool.Description += " " + p.Description
	}
	if tool.Name == "get_card_activity" {
		tool.Description += " Returns an array ordered newest first (time, ID). Only the latest 100 events are accessible; no cursor or full-history pagination."
	}
	if tool.Name == "list_cards" {
		tool.Description += " Returns items and next_cursor in column/position/ID order. epic_id and group_id accept 'none'; assignee_id accepts UUID, 'me', 'none'. All tag_ids must match. archived selects one state, not both."
	}
	if tool.Name == "list_card_movements" {
		tool.Description += " Returns period_from, period_to, items, next_cursor, newest first (time, ID). Creation is not a movement. Assignees/project/group reflect current state, not a historical snapshot. Freeze from/to for all pages, or reuse returned period_from/period_to. from is inclusive, to exclusive."
	}
	if schema.Properties["cursor"] != nil {
		tool.Description += " Copy next_cursor unchanged into cursor with the same filters until next_cursor is empty/absent. Pagination is not a database snapshot."
	}
	if tool.Name == "list_comments" || tool.Name == "add_comment" {
		tool.Description += " Supply exactly one non-empty card_id or project_id. Comments are ordered oldest first and are not paginated."
	}
	if strings.HasPrefix(tool.Name, "list_") && schema.Properties["cursor"] == nil && tool.Name != "list_comments" {
		tool.Description += " Returns an unpaginated array."
	}
	if tool.Name == "get_card" {
		tool.Description += " Supply card_id or card_number; card_id takes precedence when both are supplied. Accepts UUID, JKB-42 or decimal number."
	}
	if tool.Name == "update_card" || tool.Name == "assign_card" {
		tool.Description += " Omitted fields keep their value. Empty group_id/epic_id clears the link; null keeps it. assignee_id=null or empty clears only the primary assignee; assignee_ids=[] clears all. Never combine assignee_id and assignee_ids. Empty tag_ids clears tags; null keeps them. Archived cards are read-only."
	}
	if tool.Name == "create_card" {
		tool.Description += " title must be non-blank. Never combine assignee_id and assignee_ids. Places the card first in the target column. An archived project cannot receive new cards."
	}
	if tool.Name == "move_card" {
		tool.Description += " before_card_id and after_card_id are mutually exclusive and must refer to a card in the target column. Omit both to append to the column. Archived cards are read-only."
	}
	if tool.Name == "list_archive" {
		tool.Description += " history=false returns current archive; history=true includes past events. state filters current entity state. batch_id expands cards of a batch. Dates are optional RFC3339 bounds [from,to)."
	}
	if schema.Properties["expected_version"] != nil && tool.Name != "create_epic" {
		tool.Description += " Read the object first. On a version conflict, read again and reconsider the change; never blindly retry a mutation."
	}
	if tool.Name != "get_current_user" {
		scope := "cards:write"
		if tool.Annotations != nil && tool.Annotations.ReadOnlyHint {
			scope = "cards:read"
		}
		tool.Description += " Requires " + scope + "."
	}
	if tool.Annotations == nil {
		tool.Annotations = &mcp.ToolAnnotations{}
	}
	tool.Annotations.OpenWorldHint = boolPointer(false)
	tool.InputSchema = schema
	mcp.AddTool(server, tool, handler)
}
func number(n float64) *float64 { return &n }

var parameterDescriptions = map[string]string{
	"epic_id":          "Project UUID. On card update, empty clears; omitted or null keeps the value. list_cards additionally accepts 'none'.",
	"project_id":       "Project UUID (not its display name).",
	"card_id":          "Card UUID. get_card additionally accepts JKB number.",
	"card_number":      "Card number, e.g. JKB-42 or 42; alternative to card_id.",
	"column_id":        "Column UUID from list_columns.",
	"target_column_id": "Destination column UUID from list_columns.",
	"group_id":         "Group UUID. On update, empty clears, omitted/null keeps. list_cards additionally accepts 'none'.",
	"assignee_id":      "User UUID; do not combine with assignee_ids. List filters may accept 'me' (list_cards also 'none').",
	"assignee_ids":     "Ordered unique active user UUIDs, primary first. Empty array clears all on update; omitted/null keeps. Mutually exclusive with assignee_id.",
	"actor_user_id":    "User UUID who performed the movement, or 'me'. Omit for all actors.",
	"tag_ids":          "Tag UUIDs. In list_cards all must match; on update [] clears tags, omitted/null keeps them.",
	"name":             "Non-blank display name; surrounding whitespace is trimmed.",
	"title":            "Non-blank card title; surrounding whitespace is trimmed.",
	"description":      "Markdown text. Empty string clears; omitted/null keeps on card update but clears on project update.",
	"body":             "Non-blank Markdown comment; surrounding whitespace is trimmed.",
	"color":            "Hex color #RRGGBB. Default #00d1b2 when omitted; project updates replace the full state.",
	"expected_version": "Current positive version from the latest read; required for optimistic locking.",
	"restore_mode":     "When restoring a project: project_only or with_cards; otherwise omit.",
	"mode":             "project_only restores only the project; with_cards also restores eligible cards from its archive batch.",
	"is_archived":      "Project archived state; omitted means false. Read the complete state before updating.",
	"include_archived": "Include archived projects; default false.",
	"include_inactive": "Include inactive users; default false.",
	"archived":         "Select archived cards when true, active cards when false (default). Not an include-all switch.",
	"due_at":           "Optional deadline in RFC3339 with timezone; null means no deadline on creation.",
	"from":             "Inclusive RFC3339 lower bound with timezone. Movements default to to minus 24 hours; archive has no default lower bound.",
	"to":               "Exclusive RFC3339 upper bound with timezone; must be after from. Movements default to now; archive has no default upper bound.",
	"cursor":           "Opaque next_cursor from the previous page. Keep filters and time bounds unchanged; omit for first page.",
	"limit":            "Maximum records per page; see tool-specific range and default.",
	"query":            "Card number or case-insensitive substring of title/description.",
	"q":                "Case-insensitive archive title/number search.",
	"before_card_id":   "Place before this card UUID in the destination; cannot combine with after_card_id.",
	"after_card_id":    "Place after this card UUID in the destination; cannot combine with before_card_id.",
	"type":             "Archive entity type: card or project; omit for both.",
	"state":            "Current entity state: archived, restored or all. With history=false only current archive is returned.",
	"history":          "Include past archive transitions when true; default false (current archive only).",
	"batch_id":         "Archive batch UUID; expands the batch's card entries.",
}
