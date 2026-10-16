package utils

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type sampleStruct struct {
	ReportDate string  `json:"report_date"`
	TotalSale  float64 `json:"total_sale"`
}

func TestToCleanMap(t *testing.T) {
	// 1. Direct map
	m := map[string]any{"report_date": "2026-10-05", "total_sale": 50000.0}
	res := ToCleanMap(m)
	if res["report_date"] != "2026-10-05" || res["total_sale"] != 50000.0 {
		t.Fatalf("Direct map failed: %v", res)
	}

	// 2. bson.D
	d := bson.D{{Key: "report_date", Value: "2026-10-05"}, {Key: "total_sale", Value: 60000.0}}
	res = ToCleanMap(d)
	if res["report_date"] != "2026-10-05" || res["total_sale"] != 60000.0 {
		t.Fatalf("bson.D failed: %v", res)
	}

	// 3. *bson.D
	res = ToCleanMap(&d)
	if res["report_date"] != "2026-10-05" || res["total_sale"] != 60000.0 {
		t.Fatalf("*bson.D failed: %v", res)
	}

	// 4. bson.M
	bm := bson.M{"report_date": "2026-10-05", "total_sale": 70000.0}
	res = ToCleanMap(bm)
	if res["report_date"] != "2026-10-05" || res["total_sale"] != 70000.0 {
		t.Fatalf("bson.M failed: %v", res)
	}

	// 5. JSON string
	jsonStr := `{"report_date": "2026-10-05", "total_sale": 80000}`
	res = ToCleanMap(jsonStr)
	if res["report_date"] != "2026-10-05" || res["total_sale"] != 80000.0 {
		t.Fatalf("JSON string failed: %v", res)
	}

	// 6. Serialized Key-Value slice (e.g. from json.Unmarshal of bson.D)
	kvSlice := []any{
		map[string]any{"Key": "report_date", "Value": "2026-10-05"},
		map[string]any{"Key": "total_sale", "Value": 90000.0},
	}
	res = ToCleanMap(kvSlice)
	if res["report_date"] != "2026-10-05" || res["total_sale"] != 90000.0 {
		t.Fatalf("kvSlice failed: %v", res)
	}

	// 7. Struct
	s := sampleStruct{ReportDate: "2026-10-05", TotalSale: 100000.0}
	res = ToCleanMap(s)
	if res["report_date"] != "2026-10-05" || res["total_sale"] != 100000.0 {
		t.Fatalf("Struct failed: %v", res)
	}

	// 8. Nil
	if ToCleanMap(nil) != nil {
		t.Fatalf("Expected nil for nil input")
	}
}
