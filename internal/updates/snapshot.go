package updates

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
)

// Snapshot contains only domain fields. Version and timestamps are excluded
// from comparison so a repeated update does not wake a consumer unnecessarily.
type Snapshot struct {
	Fields          map[string]any
	ProjectID       string
	Number, Version int64
}

func Capture(ctx context.Context, tx *sql.Tx, kind, id string) (Snapshot, error) {
	var s Snapshot
	var raw []byte
	query := `SELECT json_object('name',name,'description',description,'color',color,'is_archived',json(CASE WHEN is_archived THEN 'true' ELSE 'false' END)),id,0,version FROM epics WHERE id=?`
	if kind == "card" {
		query = `SELECT json_object('title',title,'description',description,'column_id',column_id,'position',CAST(position AS TEXT),'epic_id',epic_id,'group_id',group_id,'due_at',due_at,'is_archived',json(CASE WHEN is_archived THEN 'true' ELSE 'false' END),
 'assignee_ids',json((SELECT json_group_array(user_id) FROM (SELECT c.assignee_id AS user_id,0 AS position WHERE c.assignee_id IS NOT NULL UNION ALL SELECT user_id,position FROM card_additional_assignees WHERE card_id=c.id AND user_id IS NOT c.assignee_id ORDER BY position))),
 'tag_ids',json((SELECT json_group_array(tag_id) FROM (SELECT tag_id FROM card_tags WHERE card_id=c.id ORDER BY tag_id)))),COALESCE(epic_id,''),number,version FROM cards c WHERE id=?`
	}
	if err := tx.QueryRowContext(ctx, query, id).Scan(&raw, &s.ProjectID, &s.Number, &s.Version); err != nil {
		return s, err
	}
	err := json.Unmarshal(raw, &s.Fields)
	return s, err
}
func RecordChange(ctx context.Context, tx *sql.Tx, actor Actor, kind, id, action string, before Snapshot) error {
	after, err := Capture(ctx, tx, kind, id)
	if err != nil {
		return err
	}
	changes := map[string]any{}
	for k, v := range after.Fields {
		old, exists := before.Fields[k]
		if !exists || !reflect.DeepEqual(old, v) {
			changes[k] = map[string]any{"before": old, "after": v}
		}
	}
	if len(changes) == 0 {
		return nil
	}
	e := Event{Type: kind + "." + action, EntityType: kind, EntityID: id, ProjectIDs: []string{before.ProjectID, after.ProjectID}, Version: after.Version, Changes: changes}
	if kind == "card" {
		e.CardID = id
		e.CardNumber = after.Number
	}
	return Append(ctx, tx, actor, e)
}
