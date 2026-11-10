package database

import (
	"database/sql/driver"
	"strings"

	"modernc.org/sqlite"
)

func init() {
	// SQLite's built-in lower() only folds ASCII. Archive titles and user queries
	// are Unicode, so use the same Go case mapping on both sides of the search.
	sqlite.MustRegisterDeterministicScalarFunction("jkb_lower", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		value, ok := args[0].(string)
		if !ok {
			return nil, nil
		}
		return strings.ToLower(value), nil
	})
}
