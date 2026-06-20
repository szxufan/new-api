/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package model

import (
	"testing"
)

// TestGetInt64FromMap_float64 verifies that float64 values (as produced by
// encoding/json when unmarshalling into map[string]interface{}) are correctly
// extracted as int64.
func TestGetInt64FromMap_float64(t *testing.T) {
	m := map[string]interface{}{
		"rate_limit_until": float64(1718700000),
	}
	val, ok := getInt64FromMap(m, "rate_limit_until")
	if !ok {
		t.Error("expected ok=true for float64 value")
	}
	if val != 1718700000 {
		t.Errorf("expected 1718700000, got %d", val)
	}
}

// TestGetInt64FromMap_int64 verifies that int64 values pass through correctly.
func TestGetInt64FromMap_int64(t *testing.T) {
	m := map[string]interface{}{
		"rate_limit_until": int64(1718700000),
	}
	val, ok := getInt64FromMap(m, "rate_limit_until")
	if !ok {
		t.Error("expected ok=true for int64 value")
	}
	if val != 1718700000 {
		t.Errorf("expected 1718700000, got %d", val)
	}
}

// TestGetInt64FromMap_missingKey verifies that a missing key returns ok=false.
func TestGetInt64FromMap_missingKey(t *testing.T) {
	m := map[string]interface{}{}
	_, ok := getInt64FromMap(m, "rate_limit_until")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

// TestGetInt64FromMap_wrongType verifies that non-numeric types return ok=false.
func TestGetInt64FromMap_wrongType(t *testing.T) {
	m := map[string]interface{}{
		"rate_limit_until": "not-a-number",
	}
	_, ok := getInt64FromMap(m, "rate_limit_until")
	if ok {
		t.Error("expected ok=false for string value")
	}
}

// TestGetInt64FromMap_float64WithFraction verifies that float64 values with
// fractional parts are truncated (not rounded) to int64.
func TestGetInt64FromMap_float64WithFraction(t *testing.T) {
	m := map[string]interface{}{
		"rate_limit_until": float64(1718700000.99),
	}
	val, ok := getInt64FromMap(m, "rate_limit_until")
	if !ok {
		t.Error("expected ok=true for float64 value with fraction")
	}
	if val != 1718700000 {
		t.Errorf("expected 1718700000 (truncated), got %d", val)
	}
}

// TestGetInt64FromMap_zeroFloat64 verifies that a zero float64 value is correctly
// extracted.
func TestGetInt64FromMap_zeroFloat64(t *testing.T) {
	m := map[string]interface{}{
		"rate_limit_until": float64(0),
	}
	val, ok := getInt64FromMap(m, "rate_limit_until")
	if !ok {
		t.Error("expected ok=true for zero float64 value")
	}
	if val != 0 {
		t.Errorf("expected 0, got %d", val)
	}
}

// TestGetInt64FromMap_nilValue verifies that a nil value returns ok=false.
func TestGetInt64FromMap_nilValue(t *testing.T) {
	m := map[string]interface{}{
		"rate_limit_until": nil,
	}
	_, ok := getInt64FromMap(m, "rate_limit_until")
	if ok {
		t.Error("expected ok=false for nil value")
	}
}
