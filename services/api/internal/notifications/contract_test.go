package notifications

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const contracts = "../../../../contracts/"

func TestMessagesMatchTheSharedContract(t *testing.T) {
	schema := loadSchema(t)

	queue := &recordingQueue{}
	if err := NewService(queue).Request(context.Background(), "hello worker"); err != nil {
		t.Fatal(err)
	}

	example, err := os.ReadFile(contracts + "notification.example.json")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}

	for name, body := range map[string][]byte{"sent message": []byte(queue.bodies[0]), "shared example": example} {
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			t.Fatalf("%s is not JSON: %v", name, err)
		}
		if err := schema.VisitJSON(value, openapi3.EnableFormatValidation()); err != nil {
			t.Errorf("%s does not match the contract: %v", name, err)
		}
	}
}

func loadSchema(t *testing.T) *openapi3.Schema {
	t.Helper()
	raw, err := os.ReadFile(contracts + "notification.schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	delete(document, "$schema")
	stripped, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	schema := &openapi3.Schema{}
	if err := schema.UnmarshalJSON(stripped); err != nil {
		t.Fatalf("load schema: %v", err)
	}
	return schema
}
