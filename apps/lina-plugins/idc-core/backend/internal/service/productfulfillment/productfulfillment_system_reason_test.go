package productfulfillment

import "testing"

func TestSystemTransitionReasonCatalogRequiresCompleteBilingualResources(t *testing.T) {
	complete := map[string]SystemTransitionReason{
		SystemReasonOrderCancelled: {
			LabelZhCN: "订单已取消", LabelEnUS: "Order cancelled",
		},
		SystemReasonCancellationRequiresAttention: {
			LabelZhCN: "需要人工处理", LabelEnUS: "Manual attention required",
		},
		SystemReasonNewBusinessStopped: {
			LabelZhCN: "已停止新业务", LabelEnUS: "New business stopped",
		},
	}
	catalog, err := NewSystemTransitionReasonCatalog(complete)
	if err != nil {
		t.Fatalf("create complete catalog: %v", err)
	}
	complete[SystemReasonOrderCancelled] = SystemTransitionReason{}
	resolved, err := catalog.ResolveSystemTransitionReason(SystemReasonOrderCancelled)
	if err != nil || resolved.LabelZhCN == "" || resolved.LabelEnUS != "Order cancelled" {
		t.Fatalf("catalog did not retain its immutable copy: reason=%+v err=%v", resolved, err)
	}

	delete(complete, SystemReasonOrderCancelled)
	if _, err = NewSystemTransitionReasonCatalog(complete); err == nil {
		t.Fatal("incomplete system reason resources were accepted")
	}
}

func TestSystemTransitionReasonCatalogRejectsUnknownAndInvalidReasons(t *testing.T) {
	resources := map[string]SystemTransitionReason{
		SystemReasonOrderCancelled: {
			LabelZhCN: "订单已取消", LabelEnUS: "Order cancelled",
		},
		SystemReasonCancellationRequiresAttention: {
			LabelZhCN: "需要人工处理", LabelEnUS: "Manual attention required",
		},
		SystemReasonNewBusinessStopped: {
			LabelZhCN: "已停止新业务", LabelEnUS: "New business stopped",
		},
	}
	catalog, err := NewSystemTransitionReasonCatalog(resources)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.ResolveSystemTransitionReason("provider_internal_reason"); err == nil {
		t.Fatal("unknown system reason was accepted")
	}

	resources[SystemReasonNewBusinessStopped] = SystemTransitionReason{LabelZhCN: "有效", LabelEnUS: "invalid\nlabel"}
	if _, err = NewSystemTransitionReasonCatalog(resources); err == nil {
		t.Fatal("control character in system reason was accepted")
	}
}
