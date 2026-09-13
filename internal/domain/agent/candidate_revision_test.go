package agent

import (
	"encoding/json"
	"testing"
)

func TestCandidateRevisionRejectsWrongSchemaAndIncompletePayload(t *testing.T) {
	for _, output := range []StoryCandidateRevisionOutput{
		{CandidateKind: "proposal", PayloadSchemaRef: "story_cast.schema@2", Payload: json.RawMessage(`{}`)},
		{CandidateKind: "proposal", PayloadSchemaRef: "story_blueprint.schema@2", Payload: json.RawMessage(`{"title":"只给标题"}`)},
		{CandidateKind: "unknown", PayloadSchemaRef: "story_blueprint.schema@2", Payload: json.RawMessage(`{}`)},
	} {
		if output.Validate() == nil {
			t.Fatal("接受了错误类型或缺失字段")
		}
	}
}
