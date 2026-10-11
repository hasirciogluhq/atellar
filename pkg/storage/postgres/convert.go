package postgres

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func parseAddr(ip string) *netip.Addr {
	if ip == "" {
		return nil
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil
	}
	addr = addr.Unmap()
	return &addr
}

func addrString(addr *netip.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.Unmap().String()
}

func prefixString(prefix *netip.Prefix) string {
	if prefix == nil {
		return ""
	}
	return prefix.String()
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func toText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func toTextNonEmpty(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func int4Ptr(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	v := value.Int32
	return &v
}

func toInt4(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *value, Valid: true}
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

func toTimestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func numericPtr(value pgtype.Numeric) *float64 {
	if !value.Valid {
		return nil
	}
	f, err := value.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

func toNumeric(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	// Numeric.Scan only accepts the decimal text form.
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(*value, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

func unmarshalStringMap(data []byte) map[string]string {
	result := map[string]string{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &result)
	}
	return result
}

func unmarshalAnyMap(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func newID(prefix string) (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
