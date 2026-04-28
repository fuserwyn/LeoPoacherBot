package sickleave

import "testing"

func TestEvaluateHeuristics_Ponos(t *testing.T) {
	ok, neg := EvaluateHeuristics("понос жёсткий без #sick_leave")
	if neg {
		t.Fatal("expected no negative match for понос")
	}
	if !ok {
		t.Fatal("expected heuristic approve for понос (GI keyword)")
	}
}

func TestEvaluateHeuristics_PererabotalWholeWordSafety(t *testing.T) {
	_, neg := EvaluateHeuristics("переработал, устал")
	if neg {
		t.Fatal("«переработал» не должен цепляться к отрицательному корню «работа» без целого слова")
	}
}

func TestEvaluateHeuristics_WorkLazy(t *testing.T) {
	ok, neg := EvaluateHeuristics("лень делать тренировку #sick_leave")
	if !neg {
		t.Fatal("expected rejection for лень as whole word")
	}
	if ok {
		t.Fatal("should not approve laziness")
	}
}

func TestEvaluateHeuristics_NoAILocal(t *testing.T) {
	ok, neg := EvaluateHeuristics("диарея второй день")
	if neg {
		t.Fatal("diarrhea is not refusal")
	}
	if !ok {
		t.Fatal("GI keywords should suffice without AI")
	}
}
