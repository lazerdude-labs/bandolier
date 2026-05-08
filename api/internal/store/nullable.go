package store

import "database/sql"

func nullableInt64(v int64) any {
	if v == 0 {
		return sql.NullInt64{}
	}
	return v
}

func nullableString(v string) any {
	if v == "" {
		return sql.NullString{}
	}
	return v
}

func nullableStringPtr(v *string) any {
	if v == nil {
		return sql.NullString{}
	}
	return *v
}
