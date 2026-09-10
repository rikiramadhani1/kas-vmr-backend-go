package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONMap is a small helper type implementing sql.Scanner / driver.Valuer
// so we can store arbitrary metadata (equivalent to Prisma's `Json?`
// field) in a jsonb column without pulling in gorm.io/datatypes as an
// extra dependency.
type JSONMap map[string]interface{}

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		if s, ok := value.(string); ok {
			bytes = []byte(s)
		} else {
			return errors.New("JSONMap: unsupported Scan type")
		}
	}
	if len(bytes) == 0 {
		*m = nil
		return nil
	}
	return json.Unmarshal(bytes, m)
}
