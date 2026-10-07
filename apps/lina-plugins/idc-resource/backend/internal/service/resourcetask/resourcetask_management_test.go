package resourcetask

import "testing"

func TestValidateManualTakeoverInput(t *testing.T) {
	t.Parallel()
	valid := ManualTakeoverInput{
		TaskNo: "task-management-test", ExpectedVersion: 3,
		ReasonCode: TakeoverReasonProviderResultUnresolved,
		EvidenceNos: []string{
			"evidence_22222222222222222222222222222222",
			"evidence_11111111111111111111111111111111",
		},
	}
	evidence, err := validateTakeoverInput(valid)
	if err != nil || len(evidence) != 2 ||
		evidence[0] != "evidence_11111111111111111111111111111111" {
		t.Fatalf("valid takeover evidence=%v err=%v", evidence, err)
	}

	tests := []struct {
		name  string
		input ManualTakeoverInput
	}{
		{name: "free text reason", input: ManualTakeoverInput{
			TaskNo: valid.TaskNo, ExpectedVersion: 3, ReasonCode: "please force success",
			EvidenceNos: valid.EvidenceNos[:1],
		}},
		{name: "transport violation", input: ManualTakeoverInput{
			TaskNo: valid.TaskNo, ExpectedVersion: 3, ReasonCode: valid.ReasonCode,
			EvidenceNos: valid.EvidenceNos[:1], TransportViolation: true,
		}},
		{name: "duplicate evidence", input: ManualTakeoverInput{
			TaskNo: valid.TaskNo, ExpectedVersion: 3, ReasonCode: valid.ReasonCode,
			EvidenceNos: []string{valid.EvidenceNos[0], valid.EvidenceNos[0]},
		}},
		{name: "no evidence", input: ManualTakeoverInput{
			TaskNo: valid.TaskNo, ExpectedVersion: 3, ReasonCode: valid.ReasonCode,
		}},
		{name: "stale version", input: ManualTakeoverInput{
			TaskNo: valid.TaskNo, ExpectedVersion: 0, ReasonCode: valid.ReasonCode,
			EvidenceNos: valid.EvidenceNos[:1],
		}},
		{name: "too many evidence records", input: ManualTakeoverInput{
			TaskNo: valid.TaskNo, ExpectedVersion: 3, ReasonCode: valid.ReasonCode,
			EvidenceNos: []string{
				"evidence_00000000000000000000000000000001",
				"evidence_00000000000000000000000000000002",
				"evidence_00000000000000000000000000000003",
				"evidence_00000000000000000000000000000004",
				"evidence_00000000000000000000000000000005",
				"evidence_00000000000000000000000000000006",
				"evidence_00000000000000000000000000000007",
				"evidence_00000000000000000000000000000008",
				"evidence_00000000000000000000000000000009",
				"evidence_0000000000000000000000000000000a",
				"evidence_0000000000000000000000000000000b",
				"evidence_0000000000000000000000000000000c",
				"evidence_0000000000000000000000000000000d",
				"evidence_0000000000000000000000000000000e",
				"evidence_0000000000000000000000000000000f",
				"evidence_00000000000000000000000000000010",
				"evidence_00000000000000000000000000000011",
			},
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := validateTakeoverInput(test.input); err == nil {
				t.Fatal("unsafe Provider task takeover input was accepted")
			}
		})
	}
}

func TestManualTakeoverDigestIsOrderIndependent(t *testing.T) {
	t.Parallel()
	left, leftErr := canonicalDigest([]string{"evidence_a", "evidence_b"})
	right, rightErr := canonicalDigest([]string{"evidence_a", "evidence_b"})
	if leftErr != nil || rightErr != nil || left != right || len(left) != 64 {
		t.Fatalf("canonical takeover digest left=%q right=%q errors=%v/%v", left, right, leftErr, rightErr)
	}
}
