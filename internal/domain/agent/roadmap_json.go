package agent

import "encoding/json"

// MarshalJSON 在 v3 规划中保留合法空列表；旧 v2 不增加虚构规划字段。
func (arc RoadmapArc) MarshalJSON() ([]byte, error) {
	type plainArc RoadmapArc
	if arc.ReaderExpectation == "" && arc.StagePayoff == "" && arc.IrreversibleChange == "" && arc.EndingContribution == "" && arc.CommitmentsOmission == "" && arc.RetainedQuestionsOmission == "" {
		return json.Marshal(plainArc(arc))
	}
	commitments := arc.Commitments
	if commitments == nil {
		commitments = []RoadmapCommitment{}
	}
	questions := arc.RetainedQuestions
	if questions == nil {
		questions = []RoadmapRetainedQuestion{}
	}
	return json.Marshal(struct {
		plainArc
		Commitments []RoadmapCommitment       `json:"commitments"`
		Questions   []RoadmapRetainedQuestion `json:"retained_questions"`
	}{plainArc(arc), commitments, questions})
}
