package pgutil

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func NetIPToAddr(ip net.IP) *netip.Addr {
	if ip == nil {
		return nil
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return nil
	}

	return &addr
}

func AddrToNetIP(addr *netip.Addr) net.IP {
	if addr == nil {
		return nil
	}

	return addr.AsSlice()
}

func PrefixToString(prefix *netip.Prefix) string {
	if prefix == nil {
		return ""
	}

	return prefix.String()
}

func TextToString(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}

	v := value.String
	return &v
}

func StringToText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}

	return pgtype.Text{String: *value, Valid: true}
}

func OptionalStringToText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}

	return pgtype.Text{String: value, Valid: true}
}

func Int4ToInt32Ptr(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}

	v := value.Int32
	return &v
}

func Int32PtrToInt4(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}

	return pgtype.Int4{Int32: *value, Valid: true}
}

func TimestamptzToTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	t := value.Time
	return &t
}

func TimeToTimestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}

	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func NumericToFloat64(value pgtype.Numeric) *float64 {
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

func Float64ToNumeric(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}

	var n pgtype.Numeric
	_ = n.Scan(*value)
	return n
}

func UnmarshalJSONMap(data []byte) map[string]string {
	if len(data) == 0 {
		return map[string]string{}
	}

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]string{}
	}

	return result
}

func MarshalJSONMap(value map[string]string) ([]byte, error) {
	if value == nil {
		value = map[string]string{}
	}

	return json.Marshal(value)
}

func UnmarshalJSONRaw(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	return result
}

func MarshalJSONRaw(value map[string]any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	return json.Marshal(value)
}

func GeneratePrefixedID(prefix string) (string, error) {
	randomPart, err := GenerateRandomHex(8)
	if err != nil {
		return "", err
	}

	return prefix + randomPart, nil
}

func GenerateRandomHex(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
